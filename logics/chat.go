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

	"github.com/expr-lang/expr"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
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

func (g *Chat) PopAll(indices []int) []cache.Score {
	// render template
	var buf strings.Builder
	ctx := exec.NewContext(map[string]any{
		"items": lo.Map(indices, func(i int, _ int) any {
			return g.items[i]
		}),
	})
	if err := g.template.Execute(&buf, ctx); err != nil {
		log.Logger().Error("failed to execute template", zap.Error(err))
		return nil
	}
	// chat completion
	resp, err := g.client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: g.chatModel,
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
	resp2, err := g.client.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
		Input: message,
		Model: openai.EmbeddingModel(g.embeddingModel),
	})
	if err != nil {
		log.Logger().Error("failed to create embeddings", zap.Error(err))
		return nil
	}
	embedding := resp2.Data[0].Embedding
	// search index
	scores := g.index.SearchVector(embedding, g.n, true)
	return lo.Map(scores, func(v lo.Tuple2[int, float32], _ int) cache.Score {
		return cache.Score{
			Id:         g.items[v.A].ItemId,
			Categories: g.items[v.A].Categories,
			Score:      -float64(v.B),
			Timestamp:  g.timestamp,
		}
	})
}

// stripThink strips the <think> tag from the message.
func stripThink(s string) string {
	if len(s) < 7 || s[:7] != "<think>" {
		return s
	}
	end := strings.Index(s, "</think>")
	if end == -1 {
		return s
	}
	return s[end+8:]
}

// parseMessage parse message from chat completion response.
// If there is any JSON in the message, it returns the JSON.
// Otherwise, it returns the message.
func parseMessage(message string) []string {
	source := []byte(stripThink(message))
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
						bytes, err := json.Marshal(v)
						if err != nil {
							return []string{string(bytes)}
						}
						result = append(result, string(bytes))
					}
					return result
				}
				return []string{string(bytes)}
			}
		}
	}
	return []string{string(source)}
}
