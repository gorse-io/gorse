// Copyright 2024 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logics

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/gorse-io/gorse/common/bfloats"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/juju/errors"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/sashabaranov/go-openai"
	"github.com/tiktoken-go/tokenizer"
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
	Context      context.Context
	VectorClient vectors.Database
	VectorConfig vectors.VectorConfig
	BatchSize    int
	TagsIDF      []float32
	UsersIDF     []float32
	OpenAIConfig config.OpenAIConfig
}

type ItemToItem interface {
	Timestamp() time.Time
	Count() int
	Get(i int) *data.Item
	Push(item *data.Item, feedback []int32)
	Finish() error
	PopAll(i int) []cache.Score
}

func NewItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, opts *ItemToItemOptions) (ItemToItem, error) {
	if opts == nil || opts.VectorClient == nil {
		return nil, errors.New("vector database is required for item-to-item")
	}
	switch cfg.Type {
	case "embedding":
		return newEmbeddingItemToItem(cfg, n, timestamp, opts)
	case "tags":
		if opts.TagsIDF == nil {
			return nil, errors.New("tags IDF is required for tags item-to-item")
		}
		return newTagsItemToItem(cfg, n, timestamp, opts, opts.TagsIDF)
	case "users":
		if opts.UsersIDF == nil {
			return nil, errors.New("users IDF is required for users item-to-item")
		}
		return newUsersItemToItem(cfg, n, timestamp, opts, opts.UsersIDF)
	case "auto":
		if opts.TagsIDF == nil || opts.UsersIDF == nil {
			return nil, errors.New("tags and users IDF are required for auto item-to-item")
		}
		return newAutoItemToItem(cfg, n, timestamp, opts, opts.TagsIDF, opts.UsersIDF)
	case "chat":
		if opts.OpenAIConfig.BaseURL == "" || opts.OpenAIConfig.AuthToken == "" {
			return nil, errors.New("OpenAI config is required for chat item-to-item")
		}
		return newChatItemToItem(cfg, n, timestamp, opts, opts.OpenAIConfig)
	default:
		return nil, errors.New("invalid item-to-item type")
	}
}

type baseItemToItem struct {
	ctx          context.Context
	n            int
	timestamp    time.Time
	collection   string
	distance     vectors.Distance
	scoreScale   float64
	vectorClient vectors.Database
	writer       *similarityVectorWriter

	mu      sync.Mutex
	items   []*data.Item
	queries []vectors.Vector
}

func newBaseItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, opts *ItemToItemOptions, sparse bool) *baseItemToItem {
	distance := vectors.Euclidean
	if sparse {
		distance = vectors.Dot
	}
	collection := vectors.ItemToItemCollection(cfg.Name)
	return &baseItemToItem{
		ctx:          opts.Context,
		n:            n,
		timestamp:    timestamp,
		collection:   collection,
		distance:     distance,
		scoreScale:   1,
		vectorClient: opts.VectorClient,
		writer: newSimilarityVectorWriter(opts.Context, opts.VectorClient, collection, distance,
			opts.VectorConfig, timestamp, opts.BatchSize, sparse),
	}
}

func (b *baseItemToItem) Timestamp() time.Time {
	return b.timestamp
}

func (b *baseItemToItem) Count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.items)
}

func (b *baseItemToItem) Get(i int) *data.Item {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.items[i]
}

func (b *baseItemToItem) push(item *data.Item, query vectors.Vector) {
	stored := query
	stored.Id = item.ItemId
	stored.IsHidden = item.IsHidden
	stored.Categories = item.Categories
	written := b.writer.Push(stored)
	if !written && (!b.writer.sparse || len(query.Indices) > 0) {
		return
	}
	b.mu.Lock()
	b.items = append(b.items, item)
	b.queries = append(b.queries, query)
	b.mu.Unlock()
}

func (b *baseItemToItem) Finish() error {
	return b.writer.Finish()
}

func (b *baseItemToItem) PopAll(i int) []cache.Score {
	b.mu.Lock()
	item := b.items[i]
	query := b.queries[i]
	b.mu.Unlock()
	if len(query.Values) == 0 {
		return []cache.Score{}
	}
	neighbors, err := b.vectorClient.QueryVectors(b.ctx, b.collection, query, nil, b.n+1)
	if err != nil {
		log.Logger().Error("failed to query item-to-item vectors",
			zap.String("collection", b.collection), zap.String("item_id", item.ItemId), zap.Error(err))
		return nil
	}
	scores := make([]cache.Score, 0, min(b.n, len(neighbors)))
	for _, neighbor := range neighbors {
		if neighbor.Id == item.ItemId || b.distance == vectors.Dot && neighbor.Score <= 0 {
			continue
		}
		score := float64(neighbor.Score) * b.scoreScale
		if b.distance == vectors.Euclidean {
			score = 1 / (1 - float64(neighbor.Score))
		}
		scores = append(scores, cache.Score{
			Id:         neighbor.Id,
			Score:      score,
			Categories: neighbor.Categories,
			Timestamp:  b.timestamp,
		})
		if len(scores) == b.n {
			break
		}
	}
	return scores
}

type embeddingItemToItem struct {
	*baseItemToItem
	columnFunc *vm.Program
}

func newEmbeddingItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, opts *ItemToItemOptions) (*embeddingItemToItem, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"item": data.Item{}}))
	if err != nil {
		return nil, err
	}
	return &embeddingItemToItem{
		baseItemToItem: newBaseItemToItem(cfg, n, timestamp, opts, false),
		columnFunc:     columnFunc,
	}, nil
}

func (e *embeddingItemToItem) Push(item *data.Item, _ []int32) {
	embedding, ok := ExtractItemEmbedding(item, e.columnFunc)
	if !ok || len(embedding) == 0 {
		return
	}
	e.push(item, vectors.Vector{Values: embedding})
}

func ExtractItemEmbedding(item *data.Item, columnFunc *vm.Program) ([]float32, bool) {
	result, err := expr.Run(columnFunc, map[string]any{"item": item})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression", zap.Any("item", item), zap.Error(err))
		return nil, false
	}
	v, ok := bfloats.FromAny(result)
	if !ok {
		log.Logger().Error("failed to convert column to BF16 slice", zap.Any("column", result))
		return nil, false
	}
	return bfloats.ToFloat32(v), true
}

// EmbeddingItemToItemVectorWriter is kept as the focused embedding writer used
// by callers that only need to refresh an embedding collection.
type EmbeddingItemToItemVectorWriter struct {
	columnFunc *vm.Program
	writer     *similarityVectorWriter
}

func NewEmbeddingItemToItemVectorWriter(ctx context.Context, cfg config.ItemToItemConfig, timestamp time.Time, vectorClient vectors.Database, vectorConfig vectors.VectorConfig, batchSize int) (*EmbeddingItemToItemVectorWriter, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"item": data.Item{}}))
	if err != nil {
		return nil, err
	}
	return &EmbeddingItemToItemVectorWriter{
		columnFunc: columnFunc,
		writer: newSimilarityVectorWriter(ctx, vectorClient, vectors.ItemToItemCollection(cfg.Name), vectors.Euclidean,
			vectorConfig, timestamp, batchSize, false),
	}, nil
}

func (w *EmbeddingItemToItemVectorWriter) Push(item *data.Item, _ []int32) {
	embedding, ok := ExtractItemEmbedding(item, w.columnFunc)
	if !ok || len(embedding) == 0 {
		return
	}
	w.writer.Push(vectors.Vector{
		Id: item.ItemId, Values: embedding, IsHidden: item.IsHidden, Categories: item.Categories,
	})
}

func (w *EmbeddingItemToItemVectorWriter) Finish() error { return w.writer.Finish() }

func (w *EmbeddingItemToItemVectorWriter) Dimension() int { return w.writer.Dimension() }

type tagsItemToItem struct {
	*baseItemToItem
	columnFunc *vm.Program
	idf        []float32
}

func newTagsItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, opts *ItemToItemOptions, idf []float32) (ItemToItem, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"item": data.Item{}}))
	if err != nil {
		return nil, err
	}
	return &tagsItemToItem{baseItemToItem: newBaseItemToItem(cfg, n, timestamp, opts, true), columnFunc: columnFunc, idf: idf}, nil
}

func (t *tagsItemToItem) Push(item *data.Item, _ []int32) {
	result, err := expr.Run(t.columnFunc, map[string]any{"item": item})
	if err != nil {
		log.Logger().Error("failed to evaluate column expression", zap.Any("item", item), zap.Error(err))
		return
	}
	tags := mapset.NewSet[dataset.ID]()
	flatten(result, tags)
	ids := tags.ToSlice()
	slices.Sort(ids)
	t.push(item, newSparseVector(ids, t.idf, 0))
}

type usersItemToItem struct {
	*baseItemToItem
	idf []float32
}

func newUsersItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, opts *ItemToItemOptions, idf []float32) (ItemToItem, error) {
	if cfg.Column != "" {
		return nil, errors.New("column is not supported in users item-to-item")
	}
	return &usersItemToItem{baseItemToItem: newBaseItemToItem(cfg, n, timestamp, opts, true), idf: idf}, nil
}

func (u *usersItemToItem) Push(item *data.Item, feedback []int32) {
	slices.Sort(feedback)
	u.push(item, newSparseVector(feedback, u.idf, 0))
}

type autoItemToItem struct {
	*baseItemToItem
	tagsIDF  []float32
	usersIDF []float32
}

func newAutoItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, opts *ItemToItemOptions, tagsIDF, usersIDF []float32) (ItemToItem, error) {
	base := newBaseItemToItem(cfg, n, timestamp, opts, true)
	base.scoreScale = .5
	return &autoItemToItem{baseItemToItem: base, tagsIDF: tagsIDF, usersIDF: usersIDF}, nil
}

func (a *autoItemToItem) Push(item *data.Item, feedback []int32) {
	tags := mapset.NewSet[dataset.ID]()
	flatten(item.Labels, tags)
	tagIDs := tags.ToSlice()
	slices.Sort(tagIDs)
	slices.Sort(feedback)
	vector := newSparseVector(tagIDs, a.tagsIDF, 0)
	vector = appendSparseVector(vector, newSparseVector(feedback, a.usersIDF, uint32(len(a.tagsIDF))))
	a.push(item, vector)
}

type IDF[T dataset.ID | int32] []float32

func (idf IDF[T]) similarity(a, b []T) float32 {
	i, j, sum := 0, 0, float32(0)
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			sum += idf[a[i]]
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}
	return sum
}

func flatten(o any, tags mapset.Set[dataset.ID]) {
	switch value := o.(type) {
	case dataset.ID:
		tags.Add(value)
	case []dataset.ID:
		tags.Append(value...)
	case map[string]any:
		for _, child := range value {
			flatten(child, tags)
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
}

func newChatItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, opts *ItemToItemOptions, openaiConfig config.OpenAIConfig) (*chatItemToItem, error) {
	embedding, err := newEmbeddingItemToItem(cfg, n, timestamp, opts)
	if err != nil {
		return nil, err
	}
	template, err := gonja.FromString(cfg.Prompt)
	if err != nil {
		return nil, err
	}
	clientConfig := openai.DefaultConfig(openaiConfig.AuthToken)
	clientConfig.BaseURL = openaiConfig.BaseURL
	return &chatItemToItem{
		embeddingItemToItem: embedding,
		template:            template,
		client:              openai.NewClientWithConfig(clientConfig),
		chatCompletionModel: openaiConfig.ChatCompletionModel,
		embeddingModel:      openaiConfig.EmbeddingModel,
		embeddingDimensions: openaiConfig.EmbeddingDimensions,
	}, nil
}

func (g *chatItemToItem) PopAll(i int) []cache.Score {
	g.mu.Lock()
	item := g.items[i]
	source := g.queries[i]
	g.mu.Unlock()

	var prompt strings.Builder
	if err := g.template.Execute(&prompt, exec.NewContext(map[string]any{"item": item})); err != nil {
		log.Logger().Error("failed to execute template", zap.Error(err))
		return nil
	}
	start := time.Now()
	ids, _, _ := cl100kBaseTokenizer.Encode(prompt.String())
	response, err := backoff.Retry(g.ctx, func() (openai.ChatCompletionResponse, error) {
		time.Sleep(parallel.ChatCompletionRequestsLimiter.Take(1))
		time.Sleep(parallel.ChatCompletionTokensLimiter.Take(int64(len(ids))))
		response, err := g.client.CreateChatCompletion(g.ctx, openai.ChatCompletionRequest{
			Model:    g.chatCompletionModel,
			Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: prompt.String()}},
		})
		if err == nil || isThrottled(err) {
			return response, err
		}
		return openai.ChatCompletionResponse{}, backoff.Permanent(err)
	}, backoff.WithBackOff(backoff.NewExponentialBackOff()))
	if err != nil {
		log.Logger().Error("failed to chat completion", zap.String("item_id", item.ItemId), zap.Error(err))
		return nil
	}
	messages := parseArrayFromCompletion(response.Choices[0].Message.Content)
	log.OpenAILogger().Info("chat completion",
		zap.String("prompt", prompt.String()),
		zap.String("completion", response.Choices[0].Message.Content),
		zap.Strings("parsed", messages),
		zap.Int("prompt_tokens", response.Usage.PromptTokens),
		zap.Int("completion_tokens", response.Usage.CompletionTokens),
		zap.Int("total_tokens", response.Usage.TotalTokens),
		zap.Duration("duration", time.Since(start)))

	best := make(map[string]cache.Score)
	for _, message := range messages {
		ids, _, _ := cl100kBaseTokenizer.Encode(message)
		embeddingResponse, err := backoff.Retry(g.ctx, func() (openai.EmbeddingResponse, error) {
			time.Sleep(parallel.EmbeddingRequestsLimiter.Take(1))
			time.Sleep(parallel.EmbeddingTokensLimiter.Take(int64(len(ids))))
			response, err := g.client.CreateEmbeddings(g.ctx, openai.EmbeddingRequest{
				Input: message, Model: openai.EmbeddingModel(g.embeddingModel), Dimensions: g.embeddingDimensions,
			})
			if err == nil || isThrottled(err) {
				return response, err
			}
			return openai.EmbeddingResponse{}, backoff.Permanent(err)
		}, backoff.WithBackOff(backoff.NewExponentialBackOff()))
		if err != nil {
			log.Logger().Error("failed to create embeddings", zap.String("item_id", item.ItemId), zap.Error(err))
			return nil
		}
		if len(embeddingResponse.Data) == 0 {
			continue
		}
		embedding := embeddingResponse.Data[0].Embedding
		originDistance := floats.Euclidean(embedding, source.Values)
		neighbors, err := g.vectorClient.QueryVectors(g.ctx, g.collection, vectors.Vector{Values: embedding}, nil, g.n+1)
		if err != nil {
			log.Logger().Error("failed to query chat item-to-item candidates",
				zap.String("collection", g.collection), zap.String("item_id", item.ItemId), zap.Error(err))
			return nil
		}
		for _, neighbor := range neighbors {
			if neighbor.Id == item.ItemId {
				continue
			}
			score := cache.Score{
				Id:         neighbor.Id,
				Categories: neighbor.Categories,
				Score:      1 / (1 + float64(-neighbor.Score*originDistance)),
				Timestamp:  g.timestamp,
			}
			if previous, exists := best[neighbor.Id]; !exists || score.Score > previous.Score {
				best[neighbor.Id] = score
			}
		}
	}
	scores := make([]cache.Score, 0, len(best))
	for _, score := range best {
		scores = append(scores, score)
	}
	slices.SortFunc(scores, func(a, b cache.Score) int {
		if a.Score > b.Score {
			return -1
		}
		if a.Score < b.Score {
			return 1
		}
		return strings.Compare(a.Id, b.Id)
	})
	return scores[:min(g.n, len(scores))]
}

func stripThinkInCompletion(s string) string {
	if len(s) < 7 || s[:7] != "<think>" {
		return s
	}
	_, after, ok := strings.Cut(s, "</think>")
	if !ok {
		return s
	}
	return after
}
