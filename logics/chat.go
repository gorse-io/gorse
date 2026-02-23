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

	"github.com/gorse-io/gorse/common/reranker"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/sashabaranov/go-openai"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type FeedbackItem struct {
	FeedbackType string
	data.Item
}

type ChatReranker struct {
	queryTemplate *exec.Template
	docTemplate   *exec.Template
	client        *reranker.Client
	model         string
}

func NewChatReranker(cfg config.RerankerAPIConfig, queryTemplate, docTemplate string) (*ChatReranker, error) {
	// create reranker client
	client := reranker.NewClient(cfg.AuthToken, cfg.URL)
	// create templates
	qTpl, err := gonja.FromString(queryTemplate)
	if err != nil {
		return nil, err
	}
	dTpl, err := gonja.FromString(docTemplate)
	if err != nil {
		return nil, err
	}
	return &ChatReranker{
		queryTemplate: qTpl,
		docTemplate:   dTpl,
		client:        client,
		model:         cfg.Model,
	}, nil
}

func (r *ChatReranker) Rank(ctx context.Context, user *data.User, feedback []*FeedbackItem, items []*data.Item) ([]cache.Score, error) {
	// render query
	var queryBuf strings.Builder
	queryCtx := exec.NewContext(map[string]any{
		"user":     user,
		"feedback": feedback,
	})
	if err := r.queryTemplate.Execute(&queryBuf, queryCtx); err != nil {
		return nil, err
	}
	// render documents
	documents := make([]string, len(items))
	for i, item := range items {
		var docBuf strings.Builder
		docCtx := exec.NewContext(map[string]any{
			"item": item,
		})
		if err := r.docTemplate.Execute(&docBuf, docCtx); err != nil {
			return nil, err
		}
		documents[i] = docBuf.String()
	}
	// rerank
	resp, err := r.client.Rerank(ctx, reranker.RerankRequest{
		Model:     r.model,
		Query:     queryBuf.String(),
		Documents: documents,
	})
	if err != nil {
		return nil, err
	}
	// sort items
	result := make([]cache.Score, len(resp.Results))
	for i, rerankResult := range resp.Results {
		result[i].Id = items[rerankResult.Index].ItemId
		result[i].Score = rerankResult.RelevanceScore
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
