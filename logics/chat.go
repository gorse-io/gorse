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
	"fmt"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/zhenghaoz/gorse/base/floats"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/common/ann"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
)

type Chat chatItemToItem

func NewChat(cfg config.ChatConfig, n int, timestamp time.Time, openaiConfig config.OpenAIConfig) (*Chat, error) {
	// Compile column expression
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{
		"item": data.Item{},
	}))
	if err != nil {
		return nil, err
	}
	// parse template
	template, err := gonja.FromString(cfg.Prompt)
	if err != nil {
		return nil, err
	}
	// create openai client
	clientConfig := openai.DefaultConfig(openaiConfig.AuthToken)
	clientConfig.BaseURL = openaiConfig.BaseURL
	return &Chat{
		embeddingItemToItem: &embeddingItemToItem{baseItemToItem: baseItemToItem[[]float32]{
			name:       cfg.Name,
			n:          n,
			timestamp:  timestamp,
			columnFunc: columnFunc,
			index:      ann.NewHNSW(floats.Euclidean),
		}},
		template:       template,
		client:         openai.NewClientWithConfig(clientConfig),
		chatModel:      openaiConfig.ChatCompletionModel,
		embeddingModel: openaiConfig.EmbeddingsModel,
	}, nil
}

func (c *Chat) PopAll(i int) []cache.Score {
	// render template
	var buf strings.Builder
	ctx := exec.NewContext(map[string]any{
		"item": c.items[i],
	})
	if err := c.template.Execute(&buf, ctx); err != nil {
		log.Logger().Error("failed to execute template", zap.Error(err))
		return nil
	}
	fmt.Println(buf.String())
	// chat completion
	resp, err := c.client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: c.chatModel,
		Messages: []openai.ChatCompletionMessage{{
			Role:    openai.ChatMessageRoleUser,
			Content: buf.String(),
		}},
	})
	if err != nil {
		log.Logger().Error("failed to chat completion", zap.Error(err))
		return nil
	}
	message := stripThink(resp.Choices[0].Message.Content)
	// message embedding
	resp2, err := c.client.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
		Input: message,
		Model: openai.EmbeddingModel(c.embeddingModel),
	})
	if err != nil {
		log.Logger().Error("failed to create embeddings", zap.Error(err))
		return nil
	}
	embedding := resp2.Data[0].Embedding
	// search index
	scores := c.index.SearchVector(embedding, c.n+1, true)
	return lo.Map(scores, func(v lo.Tuple2[int, float32], _ int) cache.Score {
		return cache.Score{
			Id:         c.items[v.A].ItemId,
			Categories: c.items[v.A].Categories,
			Score:      -float64(v.B),
			Timestamp:  c.timestamp,
		}
	})
}
