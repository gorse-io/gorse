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
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/sashabaranov/go-openai"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/data"
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

func (r *ChatRanker) Rank(user *data.User, feedback []*FeedbackItem, items []*data.Item) ([]string, error) {
	// render template
	var buf strings.Builder
	ctx := exec.NewContext(map[string]any{
		"user":     user,
		"feedback": feedback,
		"items":    items,
	})
	if err := r.template.Execute(&buf, ctx); err != nil {
		return nil, err
	}
	// chat completion
	resp, err := r.client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: r.model,
		Messages: []openai.ChatCompletionMessage{{
			Role:    openai.ChatMessageRoleUser,
			Content: buf.String(),
		}},
	})
	if err != nil {
		return nil, err
	}
	log.OpenAILogger().Info("chat completion",
		zap.String("prompt", buf.String()),
		zap.String("completion", resp.Choices[0].Message.Content),
		zap.Int("prompt_tokens", resp.Usage.PromptTokens),
		zap.Int("completion_tokens", resp.Usage.CompletionTokens),
		zap.Int("total_tokens", resp.Usage.TotalTokens))
	// parse response
	s := mapset.NewSet[string]()
	for _, item := range items {
		s.Add(item.ItemId)
	}
	messages := parseMessage(resp.Choices[0].Message.Content)
	var result []string
	for _, message := range messages {
		if s.Contains(message) {
			result = append(result, message)
		}
	}
	return result, nil
}
