// Copyright 2024 gorse Project Authors
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
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/gorse-io/gorse/base/heap"
	"github.com/gorse-io/gorse/base/log"
	"github.com/gorse-io/gorse/common/ann"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/tiktoken-go/tokenizer"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"go.uber.org/zap"
)

var cl100kBaseTokenizer tokenizer.Codec

func init() {
	var err error
	cl100kBaseTokenizer, err = tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		panic(err)
	}
}

type ItemToItemOptions struct {
	TagsIDF      []float32
	UsersIDF     []float32
	OpenAIConfig config.OpenAIConfig
}

type ItemToItem interface {
	Timestamp() time.Time
	Items() []*data.Item
	Push(item *data.Item, feedback []int32)
	PopAll(i int) []cache.Score
	Pool() parallel.Pool
}

func NewItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, opts *ItemToItemOptions) (ItemToItem, error) {
	switch cfg.Type {
	case "embedding":
		return newEmbeddingItemToItem(cfg, n, timestamp)
	case "tags":
		if opts == nil || opts.TagsIDF == nil {
			return nil, errors.New("tags IDF is required for tags item-to-item")
		}
		return newTagsItemToItem(cfg, n, timestamp, opts.TagsIDF)
	case "users":
		if opts == nil || opts.UsersIDF == nil {
			return nil, errors.New("users IDF is required for users item-to-item")
		}
		return newUsersItemToItem(cfg, n, timestamp, opts.UsersIDF)
	case "auto":
		if opts == nil || opts.TagsIDF == nil || opts.UsersIDF == nil {
			return nil, errors.New("tags and users IDF are required for auto item-to-item")
		}
		return newAutoItemToItem(cfg, n, timestamp, opts.TagsIDF, opts.UsersIDF)
	case "chat":
		if opts == nil || opts.OpenAIConfig.BaseURL == "" || opts.OpenAIConfig.AuthToken == "" {
			return nil, errors.New("OpenAI config is required for chat item-to-item")
		}
		return newChatItemToItem(cfg, n, timestamp, opts.OpenAIConfig)
	default:
		return nil, errors.New("invalid item-to-item type")
	}
}

type baseItemToItem[T any] struct {
	name       string
	n          int
	timestamp  time.Time
	columnFunc *vm.Program
	index      *ann.HNSW[T]
	items      []*data.Item
}

func (b *baseItemToItem[T]) Timestamp() time.Time {
	return b.timestamp
}

func (b *baseItemToItem[T]) Items() []*data.Item {
	return b.items
}

func (b *baseItemToItem[T]) PopAll(i int) []cache.Score {
	scores, err := b.index.SearchIndex(i, b.n+1, true)
	if err != nil {
		log.Logger().Error("failed to search index", zap.Error(err))
		return nil
	}
	return lo.Map(scores, func(v lo.Tuple2[int, float32], _ int) cache.Score {
		return cache.Score{
			Id:         b.items[v.A].ItemId,
			Categories: b.items[v.A].Categories,
			Score:      -float64(v.B),
			Timestamp:  b.timestamp,
		}
	})
}

func (b *baseItemToItem[T]) Pool() parallel.Pool {
	return parallel.NewSequentialPool()
}

type embeddingItemToItem struct {
	baseItemToItem[[]float32]
	dimension int
}

func newEmbeddingItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time) (*embeddingItemToItem, error) {
	// Compile column expression
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{
		"item": data.Item{},
	}))
	if err != nil {
		return nil, err
	}
	return &embeddingItemToItem{baseItemToItem: baseItemToItem[[]float32]{
		name:       cfg.Name,
		n:          n,
		timestamp:  timestamp,
		columnFunc: columnFunc,
		index:      ann.NewHNSW[[]float32](floats.Euclidean),
	}}, nil
}

func (e *embeddingItemToItem) Push(item *data.Item, _ []int32) {
	// Check if hidden
	if item.IsHidden {
		return
	}
	// Evaluate filter function
	result, err := expr.Run(e.columnFunc, map[string]any{
		"item": item,
	})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression",
			zap.Any("item", item), zap.Error(err))
		return
	}
	// Check column type
	v, ok := result.([]float32)
	if !ok {
		log.Logger().Error("invalid column type", zap.Any("column", result))
		return
	}
	// Check dimension
	if e.dimension == 0 && len(v) > 0 {
		e.dimension = len(v)
	} else if e.dimension != len(v) {
		log.Logger().Error("invalid column dimension", zap.Int("dimension", len(v)))
		return
	}
	// Push item
	e.items = append(e.items, item)
	_ = e.index.Add(v)
}

type tagsItemToItem struct {
	baseItemToItem[[]dataset.ID]
	IDF[dataset.ID]
}

func newTagsItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, idf []float32) (ItemToItem, error) {
	// Compile column expression
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{
		"item": data.Item{},
	}))
	if err != nil {
		return nil, err
	}
	t := &tagsItemToItem{IDF: idf}
	t.baseItemToItem = baseItemToItem[[]dataset.ID]{
		name:       cfg.Name,
		n:          n,
		timestamp:  timestamp,
		columnFunc: columnFunc,
		index:      ann.NewHNSW[[]dataset.ID](t.distance),
	}
	return t, nil
}

func (t *tagsItemToItem) Push(item *data.Item, _ []int32) {
	// Check if hidden
	if item.IsHidden {
		return
	}
	// Evaluate filter function
	result, err := expr.Run(t.columnFunc, map[string]any{
		"item": item,
	})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression",
			zap.Any("item", item), zap.Error(err))
		return
	}
	// Extract tags
	tSet := mapset.NewSet[dataset.ID]()
	flatten(result, tSet)
	v := tSet.ToSlice()
	sort.Slice(v, func(i, j int) bool {
		return v[i] < v[j]
	})
	// Push item
	t.items = append(t.items, item)
	_ = t.index.Add(v)
}

type usersItemToItem struct {
	baseItemToItem[[]int32]
	IDF[int32]
}

func newUsersItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, idf []float32) (ItemToItem, error) {
	if cfg.Column != "" {
		return nil, errors.New("column is not supported in users item-to-item")
	}
	u := &usersItemToItem{IDF: idf}
	u.baseItemToItem = baseItemToItem[[]int32]{
		name:      cfg.Name,
		n:         n,
		timestamp: timestamp,
		index:     ann.NewHNSW[[]int32](u.distance),
	}
	return u, nil
}

func (u *usersItemToItem) Push(item *data.Item, feedback []int32) {
	// Check if hidden
	if item.IsHidden {
		return
	}
	// Sort feedback
	sort.Slice(feedback, func(i, j int) bool {
		return feedback[i] < feedback[j]
	})
	// Push item
	u.items = append(u.items, item)
	_ = u.index.Add(feedback)
}

type autoItemToItem struct {
	baseItemToItem[lo.Tuple2[[]dataset.ID, []int32]]
	tIDF IDF[dataset.ID]
	uIDF IDF[int32]
}

func newAutoItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, tIDF, uIDF []float32) (ItemToItem, error) {
	a := &autoItemToItem{
		tIDF: tIDF,
		uIDF: uIDF,
	}
	a.baseItemToItem = baseItemToItem[lo.Tuple2[[]dataset.ID, []int32]]{
		name:      cfg.Name,
		n:         n,
		timestamp: timestamp,
		index:     ann.NewHNSW[lo.Tuple2[[]dataset.ID, []int32]](a.distance),
	}
	return a, nil
}

func (a *autoItemToItem) Push(item *data.Item, feedback []int32) {
	// Check if hidden
	if item.IsHidden {
		return
	}
	// Extract tags
	tSet := mapset.NewSet[dataset.ID]()
	flatten(item.Labels, tSet)
	v := tSet.ToSlice()
	sort.Slice(v, func(i, j int) bool {
		return v[i] < v[j]
	})
	// Sort feedback
	sort.Slice(feedback, func(i, j int) bool {
		return feedback[i] < feedback[j]
	})
	// Push item
	a.items = append(a.items, item)
	_ = a.index.Add(lo.Tuple2[[]dataset.ID, []int32]{A: v, B: feedback})
}

func (a *autoItemToItem) distance(u, v lo.Tuple2[[]dataset.ID, []int32]) float32 {
	return (a.tIDF.distance(u.A, v.A) + a.uIDF.distance(u.B, v.B)) / 2
}

type IDF[T dataset.ID | int32] []float32

func (idf IDF[T]) distance(a, b []T) float32 {
	commonSum, commonCount := idf.weightedSumCommonElements(a, b)
	if len(a) == len(b) && commonCount == float32(len(a)) {
		// If two items have the same tags, its distance is zero.
		return 0
	} else if commonCount > 0 && len(a) > 0 && len(b) > 0 {
		// Add shrinkage to avoid division by zero
		return 1 - commonSum*commonCount/
			math32.Sqrt(idf.weightedSum(a))/
			math32.Sqrt(idf.weightedSum(b))/
			(commonCount+100)
	} else {
		// If two items have no common tags, its distance is one.
		return 1
	}
}

func (idf IDF[T]) weightedSumCommonElements(a, b []T) (float32, float32) {
	i, j, sum, count := 0, 0, float32(0), float32(0)
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			sum += idf[a[i]]
			count++
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		}
	}
	return sum, count
}

func (idf IDF[T]) weightedSum(a []T) float32 {
	var sum float32
	for _, i := range a {
		sum += idf[i]
	}
	return sum
}

func flatten(o any, tSet mapset.Set[dataset.ID]) {
	switch typed := o.(type) {
	case dataset.ID:
		tSet.Add(typed)
		return
	case []dataset.ID:
		tSet.Append(typed...)
		return
	case map[string]any:
		for _, v := range typed {
			flatten(v, tSet)
		}
	}
}

type chatItemToItem struct {
	*embeddingItemToItem
	template            *exec.Template
	client              *openai.Client
	chatCompletionModel string
	embeddingModel      string
	embeddingDimensions int
	poolSize            int
}

func newChatItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, openaiConfig config.OpenAIConfig) (*chatItemToItem, error) {
	// create embedding item-to-item recommender
	embedding, err := newEmbeddingItemToItem(cfg, n, timestamp)
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
	return &chatItemToItem{
		embeddingItemToItem: embedding,
		template:            template,
		client:              openai.NewClientWithConfig(clientConfig),
		chatCompletionModel: openaiConfig.ChatCompletionModel,
		embeddingModel:      openaiConfig.EmbeddingModel,
		embeddingDimensions: openaiConfig.EmbeddingDimensions,
		poolSize:            min(openaiConfig.ChatCompletionRPM, openaiConfig.EmbeddingRPM),
	}, nil
}

func (g *chatItemToItem) PopAll(i int) []cache.Score {
	// evaluate column expression and get embedding vector
	result, err := expr.Run(g.columnFunc, map[string]any{
		"item": g.items[i],
	})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression",
			zap.Any("item", g.items[i]), zap.Error(err))
		return nil
	}
	embedding0, ok := result.([]float32)
	if !ok {
		log.Logger().Error("invalid column type", zap.Any("column", result))
		return nil
	}
	// render template
	var buf strings.Builder
	ctx := exec.NewContext(map[string]any{
		"item": g.items[i],
	})
	if err := g.template.Execute(&buf, ctx); err != nil {
		log.Logger().Error("failed to execute template", zap.Error(err))
		return nil
	}
	// chat completion
	start := time.Now()
	ids, _, _ := cl100kBaseTokenizer.Encode(buf.String())
	resp, err := backoff.Retry(context.Background(), func() (openai.ChatCompletionResponse, error) {
		time.Sleep(parallel.ChatCompletionRequestsLimiter.Take(1))
		time.Sleep(parallel.ChatCompletionTokensLimiter.Take(int64(len(ids))))
		resp, err := g.client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
			Model: g.chatCompletionModel,
			Messages: []openai.ChatCompletionMessage{{
				Role:    openai.ChatMessageRoleUser,
				Content: buf.String(),
			}},
		})
		if err == nil {
			return resp, nil
		}
		if throttled(err) {
			return openai.ChatCompletionResponse{}, err
		}
		return openai.ChatCompletionResponse{}, backoff.Permanent(err)
	}, backoff.WithBackOff(backoff.NewExponentialBackOff()))
	if err != nil {
		log.Logger().Error("failed to chat completion", zap.String("item_id", g.items[i].ItemId), zap.Error(err))
		return nil
	}
	duration := time.Since(start)
	parsed := parseJSONArrayFromCompletion(resp.Choices[0].Message.Content)
	log.OpenAILogger().Info("chat completion",
		zap.String("prompt", buf.String()),
		zap.String("completion", resp.Choices[0].Message.Content),
		zap.Strings("parsed", parsed),
		zap.Int("prompt_tokens", resp.Usage.PromptTokens),
		zap.Int("completion_tokens", resp.Usage.CompletionTokens),
		zap.Int("total_tokens", resp.Usage.TotalTokens),
		zap.Duration("duration", duration))
	// message embedding
	embeddings := make([][]float32, len(parsed))
	for i, message := range parsed {
		ids, _, _ := cl100kBaseTokenizer.Encode(message)
		resp, err := backoff.Retry(context.Background(), func() (openai.EmbeddingResponse, error) {
			time.Sleep(parallel.EmbeddingRequestsLimiter.Take(1))
			time.Sleep(parallel.EmbeddingTokensLimiter.Take(int64(len(ids))))
			resp, err := g.client.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
				Input:      message,
				Model:      openai.EmbeddingModel(g.embeddingModel),
				Dimensions: g.embeddingDimensions,
			})
			if err == nil {
				return resp, nil
			}
			if throttled(err) {
				return openai.EmbeddingResponse{}, err
			}
			return openai.EmbeddingResponse{}, backoff.Permanent(err)
		}, backoff.WithBackOff(backoff.NewExponentialBackOff()))
		if err != nil {
			log.Logger().Error("failed to create embeddings", zap.String("item_id", g.items[i].ItemId), zap.Error(err))
			return nil
		}
		embeddings[i] = resp.Data[0].Embedding
	}
	// search index
	pq := heap.NewPriorityQueue(true)
	for _, embedding := range embeddings {
		score0 := floats.Euclidean(embedding, embedding0)
		scores := g.index.SearchVector(embedding, g.n+1, true)
		for _, score := range scores {
			if score.A != i {
				pq.Push(int32(score.A), score.B*score0)
				if pq.Len() > g.n {
					pq.Pop()
				}
			}
		}
	}
	scores := make([]cache.Score, pq.Len())
	for i := 9; i >= 0; i-- {
		id, score := pq.Pop()
		scores[i] = cache.Score{
			Id:         g.items[id].ItemId,
			Categories: g.items[id].Categories,
			Score:      -float64(score),
			Timestamp:  g.timestamp,
		}
	}
	return scores
}

func (g *chatItemToItem) Pool() parallel.Pool {
	return parallel.NewConcurrentPool(g.poolSize)
}

func stripThinkInCompletion(s string) string {
	if len(s) < 7 || s[:7] != "<think>" {
		return s
	}
	end := strings.Index(s, "</think>")
	if end == -1 {
		return s
	}
	return s[end+8:]
}

// parseJSONArrayFromCompletion parse JSON array from completion.
// If the completion contains a JSON array, it will return each element in the array.
// If the completion contains a JSON object, it will return the object as a string.
// Otherwise, it will return the completion as a string.
func parseJSONArrayFromCompletion(completion string) []string {
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
			}
		}
	}
	return []string{string(source)}
}

func throttled(err error) bool {
	if requestErr, ok := err.(*openai.APIError); ok {
		if requestErr.HTTPStatusCode == 429 {
			return true
		}
	}
	return false
}
