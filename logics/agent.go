// Copyright 2025 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logics

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/juju/errors"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

const (
	agentSearchToolName       = "search_items"
	defaultAgentMaxIterations = 4
)

type Agent struct {
	config       config.AgentConfig
	openAIConfig config.OpenAIConfig
	dataClient   data.Database

	userId       string
	userFeedback []data.Feedback
	categories   []string
	excludeSet   mapset.Set[string]
	cacheSize    int
	client       *openai.Client
	promptTpl    *exec.Template
}

type agentSearchResult struct {
	ItemId     string   `json:"item_id"`
	Categories []string `json:"categories,omitempty"`
	Labels     any      `json:"labels,omitempty"`
	Comment    string   `json:"comment,omitempty"`
}

type agentSearchArguments struct {
	Query string `json:"query"`
	N     int    `json:"n"`
}

func NewAgent(cfg config.AgentConfig, openAIConfig config.OpenAIConfig, dataClient data.Database, userId string,
	userFeedback []data.Feedback, categories []string, excludeSet mapset.Set[string], cacheSize int) (*Agent, error) {
	promptTpl, err := gonja.FromString(cfg.PromptTemplate)
	if err != nil {
		return nil, errors.Trace(err)
	}
	clientConfig := openai.DefaultConfig(openAIConfig.AuthToken)
	clientConfig.BaseURL = openAIConfig.BaseURL
	return &Agent{
		config:       cfg,
		openAIConfig: openAIConfig,
		dataClient:   dataClient,
		userId:       userId,
		userFeedback: userFeedback,
		categories:   categories,
		excludeSet:   excludeSet,
		cacheSize:    cacheSize,
		client:       openai.NewClientWithConfig(clientConfig),
		promptTpl:    promptTpl,
	}, nil
}

func (a *Agent) Recommend(ctx context.Context) ([]cache.Score, error) {
	if a.openAIConfig.ChatCompletionModel == "" {
		return nil, errors.New("chat completion model is required for agent recommender")
	}

	feedback := append([]data.Feedback(nil), a.userFeedback...)
	data.SortFeedbacks(feedback)
	if a.cacheSize > 0 && len(feedback) > a.cacheSize {
		feedback = feedback[:a.cacheSize]
	}
	prompt, err := a.renderPrompt(feedback)
	if err != nil {
		return nil, errors.Trace(err)
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: "You are an item recommendation agent. Use the search_items tool to find candidate items from the catalog based on the user's feedback history. " +
				"When you have enough candidates, return recommended item IDs only, ordered from most to least relevant. Prefer a JSON array of item IDs.",
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}

	candidateItems := make(map[string]data.Item)
	var finalContent string
	for i := 0; i < a.maxIterations(); i++ {
		resp, err := a.createChatCompletion(ctx, messages)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if len(resp.Choices) == 0 {
			return nil, errors.New("empty chat completion response")
		}
		message := resp.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			finalContent = message.Content
			break
		}
		messages = append(messages, message)
		for _, toolCall := range message.ToolCalls {
			if toolCall.Function.Name != agentSearchToolName {
				continue
			}
			items, err := a.callSearchItems(ctx, toolCall.Function.Arguments)
			if err != nil {
				return nil, errors.Trace(err)
			}
			for _, item := range items {
				candidateItems[item.ItemId] = item
			}
			content, err := marshalAgentSearchResults(items)
			if err != nil {
				return nil, errors.Trace(err)
			}
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: toolCall.ID,
				Name:       agentSearchToolName,
				Content:    content,
			})
		}
	}
	if finalContent == "" {
		resp, err := a.createChatCompletion(ctx, messages)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if len(resp.Choices) == 0 {
			return nil, errors.New("empty chat completion response")
		}
		finalContent = resp.Choices[0].Message.Content
	}

	return a.parseRecommendations(ctx, finalContent, candidateItems)
}

func (a *Agent) maxIterations() int {
	if a.config.MaxIterations <= 0 {
		return defaultAgentMaxIterations
	}
	return a.config.MaxIterations
}

func (a *Agent) renderPrompt(feedback []data.Feedback) (string, error) {
	var buf strings.Builder
	promptCtx := exec.NewContext(map[string]any{
		"user_id":  a.userId,
		"feedback": feedback,
	})
	if err := a.promptTpl.Execute(&buf, promptCtx); err != nil {
		return "", errors.Trace(err)
	}
	return strings.TrimSpace(buf.String()), nil
}

func (a *Agent) createChatCompletion(ctx context.Context, messages []openai.ChatCompletionMessage) (openai.ChatCompletionResponse, error) {
	prompt := lo.SumBy(messages, func(message openai.ChatCompletionMessage) int {
		ids, _, _ := cl100kBaseTokenizer.Encode(message.Content)
		return len(ids)
	})
	return backoff.Retry(ctx, func() (openai.ChatCompletionResponse, error) {
		time.Sleep(parallel.ChatCompletionRequestsLimiter.Take(1))
		time.Sleep(parallel.ChatCompletionTokensLimiter.Take(int64(prompt)))
		resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    a.openAIConfig.ChatCompletionModel,
			Messages: messages,
			Tools: []openai.Tool{{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        agentSearchToolName,
					Description: "Search items from the catalog using a natural language query.",
					Parameters: jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"query": {
								Type:        jsonschema.String,
								Description: "Search query for item full-text search.",
							},
							"n": {
								Type:        jsonschema.Integer,
								Description: "Maximum number of items to return.",
							},
						},
						Required: []string{"query"},
					},
				},
			}},
		})
		if err == nil {
			return resp, nil
		}
		if isThrottled(err) {
			return openai.ChatCompletionResponse{}, err
		}
		return openai.ChatCompletionResponse{}, backoff.Permanent(err)
	}, backoff.WithBackOff(backoff.NewExponentialBackOff()))
}

func (a *Agent) callSearchItems(ctx context.Context, arguments string) ([]data.Item, error) {
	var args agentSearchArguments
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return nil, errors.Trace(err)
	}
	if args.N <= 0 || args.N > a.cacheSize {
		args.N = a.cacheSize
	}
	items, err := a.dataClient.SearchItems(ctx, args.Query, args.N+a.excludeSet.Cardinality())
	if err != nil {
		return nil, errors.Trace(err)
	}
	items = lo.Filter(items, func(item data.Item, _ int) bool {
		return !a.excludeSet.Contains(item.ItemId) && a.matchCategories(item.Categories)
	})
	if args.N > 0 && len(items) > args.N {
		items = items[:args.N]
	}
	return items, nil
}

func marshalAgentSearchResults(items []data.Item) (string, error) {
	results := lo.Map(items, func(item data.Item, _ int) agentSearchResult {
		return agentSearchResult{
			ItemId:     item.ItemId,
			Categories: item.Categories,
			Labels:     item.Labels,
			Comment:    item.Comment,
		}
	})
	bytes, err := json.Marshal(results)
	if err != nil {
		return "", errors.Trace(err)
	}
	return string(bytes), nil
}

func (a *Agent) parseRecommendations(ctx context.Context, completion string, candidateItems map[string]data.Item) ([]cache.Score, error) {
	ids := parseArrayFromCompletion(completion)
	if len(ids) == 0 {
		return nil, nil
	}
	missingIds := make([]string, 0)
	seen := mapset.NewSet[string]()
	for _, id := range ids {
		if _, ok := candidateItems[id]; !ok {
			missingIds = append(missingIds, id)
		}
	}
	if len(missingIds) > 0 {
		items, err := a.dataClient.BatchGetItems(ctx, missingIds, data.GetOptions{SkipHidden: true})
		if err != nil {
			return nil, errors.Trace(err)
		}
		for _, item := range items {
			candidateItems[item.ItemId] = item
		}
	}
	scores := make([]cache.Score, 0, len(ids))
	for _, id := range ids {
		if seen.Contains(id) || a.excludeSet.Contains(id) {
			continue
		}
		item, ok := candidateItems[id]
		if !ok || !a.matchCategories(item.Categories) {
			continue
		}
		seen.Add(id)
		scores = append(scores, cache.Score{
			Id:         id,
			Score:      float64(len(ids) - len(scores)),
			Categories: item.Categories,
		})
		if a.cacheSize > 0 && len(scores) >= a.cacheSize {
			break
		}
	}
	return scores, nil
}

func (a *Agent) matchCategories(itemCategories []string) bool {
	if len(a.categories) == 0 {
		return true
	}
	itemCategorySet := mapset.NewSet(itemCategories...)
	return itemCategorySet.Contains(a.categories...)
}
