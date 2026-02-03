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
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/sashabaranov/go-openai"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"go.uber.org/zap"
)

type FeedbackItem struct {
	FeedbackType string
	data.Item
}

type ChatRanker struct {
	template *exec.Template
	client   *openai.Client
	model    string
}

func NewChatRanker(cfg config.OpenAIConfig, prompt string) (*ChatRanker, error) {
	// create OpenAI client
	clientConfig := openai.DefaultConfig(cfg.AuthToken)
	clientConfig.BaseURL = cfg.BaseURL
	client := openai.NewClientWithConfig(clientConfig)
	// create template
	template, err := gonja.FromString(prompt)
	if err != nil {
		return nil, err
	}
	return &ChatRanker{
		template: template,
		client:   client,
		model:    cfg.ChatCompletionModel,
	}, nil
}

func (r *ChatRanker) Rank(ctx context.Context, user *data.User, feedback []*FeedbackItem, items []*data.Item) ([]string, error) {
	// render template
	var buf strings.Builder
	tplCtx := exec.NewContext(map[string]any{
		"user":     user,
		"feedback": feedback,
		"items":    items,
	})
	if err := r.template.Execute(&buf, tplCtx); err != nil {
		return nil, err
	}
	// chat completion
	start := time.Now()
	resp, err := backoff.Retry(ctx, func() (openai.ChatCompletionResponse, error) {
		resp, err := r.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: r.model,
			Messages: []openai.ChatCompletionMessage{{
				Role:    openai.ChatMessageRoleUser,
				Content: buf.String(),
			}},
			ChatTemplateKwargs: map[string]any{
				"enable_thinking": false, // Ollama, Alibaba Cloud
				"thinking":        false, // NVIDIA NIM
			},
		})
		if err == nil {
			return resp, nil
		}
		if isThrottled(err) {
			return openai.ChatCompletionResponse{}, err
		}
		return openai.ChatCompletionResponse{}, backoff.Permanent(err)
	}, backoff.WithBackOff(backoff.NewConstantBackOff(time.Minute)))
	if err != nil {
		return nil, err
	}
	duration := time.Since(start)
	// parse response
	parsed := parseArrayFromCompletion(resp.Choices[0].Message.Content)
	log.OpenAILogger().Info("chat completion",
		zap.String("prompt", buf.String()),
		zap.String("completion", resp.Choices[0].Message.Content),
		zap.Strings("parsed", parsed),
		zap.Int("prompt_tokens", resp.Usage.PromptTokens),
		zap.Int("completion_tokens", resp.Usage.CompletionTokens),
		zap.Int("total_tokens", resp.Usage.TotalTokens),
		zap.Duration("duration", duration))
	// filter items
	s := mapset.NewSet[string]()
	for _, item := range items {
		s.Add(item.ItemId)
	}
	var result []string
	m := mapset.NewSet[string]()
	for _, itemId := range parsed {
		if s.Contains(itemId) && !m.Contains(itemId) {
			result = append(result, itemId)
			m.Add(itemId)
		}
	}
	return result, nil
}

// parseArrayFromCompletion parse JSON array from completion.
// If the completion contains a JSON array, it will return each element in the array.
// If the completion contains a JSON object, it will return the object as a string.
// Otherwise, it will return the completion as a string.
func parseArrayFromCompletion(completion string) []string {
	source := []byte(stripThinkInCompletion(completion))
	root := goldmark.DefaultParser().Parse(text.NewReader(source))
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		if n.Kind() != ast.KindFencedCodeBlock {
			continue
		}
		if codeBlock, ok := n.(*ast.FencedCodeBlock); ok {
			if string(codeBlock.Language(source)) == "json" {
				bytes := codeBlock.Text(source)
				if bytes[0] == '[' {
					var temp []any
					err := json.Unmarshal(bytes, &temp)
					if err != nil {
						return []string{string(bytes)}
					}
					var result []string
					for _, v := range temp {
						var bytes []byte
						switch typed := v.(type) {
						case string:
							bytes = []byte(typed)
						default:
							bytes, err = json.Marshal(v)
							if err != nil {
								return []string{string(bytes)}
							}
						}
						result = append(result, string(bytes))
					}
					return result
				}
				return []string{string(bytes)}
			} else if string(codeBlock.Language(source)) == "csv" {
				// If the code block is CSV, retrieve 1st column as IDs.
				bytes := codeBlock.Text(source)
				lines := strings.Split(string(bytes), "\n")
				var result []string
				for _, line := range lines {
					fields := strings.Split(line, ",")
					if len(fields) > 0 && strings.TrimSpace(fields[0]) != "" {
						result = append(result, strings.TrimSpace(fields[0]))
					}
				}
				return result
			}
		}
	}
	var result []string
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func isThrottled(err error) bool {
	switch e := err.(type) {
	case *openai.APIError:
		if e.HTTPStatusCode == 429 {
			return true
		}
	case *openai.RequestError:
		return e.HTTPStatusCode == 504 || e.HTTPStatusCode == 520
	}
	return false
}
