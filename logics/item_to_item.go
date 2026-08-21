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
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/gorse-io/gorse/common/bfloats"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type ItemToItemOptions struct {
	Context      context.Context
	VectorClient vectors.Database
	VectorConfig vectors.VectorConfig
	BatchSize    int
	TagsIDF      []float32
	UsersIDF     []float32
}

type ItemToItem interface {
	Add(item *data.Item, feedback []int32)
	Clean() error
}

func QueryItemToItem(ctx context.Context, vectorClient vectors.Database, itemToItemConfig config.ItemToItemConfig, itemId string, categories []string, n int) ([]cache.Score, error) {
	collection := vectors.ItemToItemCollection(itemToItemConfig.Name)
	queries, err := vectorClient.GetVectors(ctx, collection, []string{itemId})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(queries) == 0 {
		return nil, nil
	}
	neighbors, err := vectorClient.QueryVectors(ctx, collection, queries[0], categories, n+1)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	distance := vectors.Dot
	if itemToItemConfig.Type == "embedding" {
		distance = vectors.Euclidean
	}
	scoreScale := 1.0
	if itemToItemConfig.Type == "auto" {
		scoreScale = .5
	}
	scores := make([]cache.Score, 0, min(n, len(neighbors)))
	for _, neighbor := range neighbors {
		if neighbor.Id == itemId || distance == vectors.Dot && neighbor.Score <= 0 {
			continue
		}
		score := float64(neighbor.Score) * scoreScale
		if distance == vectors.Euclidean {
			score = 1 / (1 - score)
		}
		scores = append(scores, cache.Score{Id: neighbor.Id, Score: score, Categories: neighbor.Categories})
		if len(scores) == n {
			break
		}
	}
	return scores, nil
}

func NewItemToItem(cfg config.ItemToItemConfig, timestamp time.Time, opts *ItemToItemOptions) (ItemToItem, error) {
	if opts == nil || opts.VectorClient == nil {
		return nil, errors.New("vector database is required for item-to-item")
	}
	switch cfg.Type {
	case "embedding":
		return newEmbeddingItemToItem(cfg, timestamp, opts)
	case "tags":
		if opts.TagsIDF == nil {
			return nil, errors.New("tags IDF is required for tags item-to-item")
		}
		return newTagsItemToItem(cfg, timestamp, opts, opts.TagsIDF)
	case "users":
		if opts.UsersIDF == nil {
			return nil, errors.New("users IDF is required for users item-to-item")
		}
		return newUsersItemToItem(cfg, timestamp, opts, opts.UsersIDF)
	case "auto":
		if opts.TagsIDF == nil || opts.UsersIDF == nil {
			return nil, errors.New("tags and users IDF are required for auto item-to-item")
		}
		return newAutoItemToItem(cfg, timestamp, opts, opts.TagsIDF, opts.UsersIDF)
	default:
		return nil, errors.New("invalid item-to-item type")
	}
}

type baseItemToItem struct {
	writer *similarityVectorWriter
}

func newBaseItemToItem(cfg config.ItemToItemConfig, timestamp time.Time, opts *ItemToItemOptions, sparse bool) *baseItemToItem {
	distance := vectors.Euclidean
	if sparse {
		distance = vectors.Dot
	}
	return &baseItemToItem{
		writer: newSimilarityVectorWriter(opts.Context, opts.VectorClient, vectors.ItemToItemCollection(cfg.Name), distance,
			opts.VectorConfig, timestamp, opts.BatchSize, sparse),
	}
}

func (b *baseItemToItem) push(item *data.Item, query vectors.Vector) {
	query.Id = item.ItemId
	query.IsHidden = item.IsHidden
	query.Categories = item.Categories
	b.writer.Push(query)
}

func (b *baseItemToItem) Clean() error {
	return b.writer.Clean()
}

type embeddingItemToItem struct {
	*baseItemToItem
	columnFunc *vm.Program
}

func newEmbeddingItemToItem(cfg config.ItemToItemConfig, timestamp time.Time, opts *ItemToItemOptions) (*embeddingItemToItem, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"item": data.Item{}}))
	if err != nil {
		return nil, err
	}
	return &embeddingItemToItem{
		baseItemToItem: newBaseItemToItem(cfg, timestamp, opts, false),
		columnFunc:     columnFunc,
	}, nil
}

func (e *embeddingItemToItem) Add(item *data.Item, _ []int32) {
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

type tagsItemToItem struct {
	*baseItemToItem
	columnFunc *vm.Program
	idf        []float32
}

func newTagsItemToItem(cfg config.ItemToItemConfig, timestamp time.Time, opts *ItemToItemOptions, idf []float32) (ItemToItem, error) {
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{"item": data.Item{}}))
	if err != nil {
		return nil, err
	}
	return &tagsItemToItem{baseItemToItem: newBaseItemToItem(cfg, timestamp, opts, true), columnFunc: columnFunc, idf: idf}, nil
}

func (t *tagsItemToItem) Add(item *data.Item, _ []int32) {
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

func newUsersItemToItem(cfg config.ItemToItemConfig, timestamp time.Time, opts *ItemToItemOptions, idf []float32) (ItemToItem, error) {
	if cfg.Column != "" {
		return nil, errors.New("column is not supported in users item-to-item")
	}
	return &usersItemToItem{baseItemToItem: newBaseItemToItem(cfg, timestamp, opts, true), idf: idf}, nil
}

func (u *usersItemToItem) Add(item *data.Item, feedback []int32) {
	slices.Sort(feedback)
	u.push(item, newSparseVector(feedback, u.idf, 0))
}

type autoItemToItem struct {
	*baseItemToItem
	tagsIDF  []float32
	usersIDF []float32
}

func newAutoItemToItem(cfg config.ItemToItemConfig, timestamp time.Time, opts *ItemToItemOptions, tagsIDF, usersIDF []float32) (ItemToItem, error) {
	return &autoItemToItem{baseItemToItem: newBaseItemToItem(cfg, timestamp, opts, true), tagsIDF: tagsIDF, usersIDF: usersIDF}, nil
}

func (a *autoItemToItem) Add(item *data.Item, feedback []int32) {
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
