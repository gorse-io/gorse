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
	"errors"
	"sort"
	"time"

	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base/floats"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/common/ann"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/dataset"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
)

type ItemToItemOptions struct {
	TagsIDF  []float32
	UsersIDF []float32
}

type ItemToItem interface {
	Items() []*data.Item
	Push(item *data.Item, feedback []dataset.ID)
	PopAll(callback func(itemId string, score []cache.Score))
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

func (b *baseItemToItem[T]) Items() []*data.Item {
	return b.items
}

func (b *baseItemToItem[T]) PopAll(callback func(itemId string, score []cache.Score)) {
	for index, item := range b.items {
		scores, err := b.index.SearchIndex(index, b.n+1, true)
		if err != nil {
			log.Logger().Error("failed to search index", zap.Error(err))
			return
		}
		callback(item.ItemId, lo.Map(scores, func(v lo.Tuple2[int, float32], _ int) cache.Score {
			return cache.Score{
				Id:         b.items[v.A].ItemId,
				Categories: b.items[v.A].Categories,
				Score:      -float64(v.B),
				Timestamp:  b.timestamp,
			}
		}))
	}
}

type embeddingItemToItem struct {
	baseItemToItem[[]float32]
	dimension int
}

func newEmbeddingItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time) (ItemToItem, error) {
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

func (e *embeddingItemToItem) Push(item *data.Item, _ []dataset.ID) {
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
	IDF
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

func (t *tagsItemToItem) Push(item *data.Item, _ []dataset.ID) {
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
	baseItemToItem[[]dataset.ID]
	IDF
}

func newUsersItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, idf []float32) (ItemToItem, error) {
	if cfg.Column != "" {
		return nil, errors.New("column is not supported in users item-to-item")
	}
	u := &usersItemToItem{IDF: idf}
	u.baseItemToItem = baseItemToItem[[]dataset.ID]{
		name:      cfg.Name,
		n:         n,
		timestamp: timestamp,
		index:     ann.NewHNSW[[]dataset.ID](u.distance),
	}
	return u, nil
}

func (u *usersItemToItem) Push(item *data.Item, feedback []dataset.ID) {
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
	baseItemToItem[lo.Tuple2[[]dataset.ID, []dataset.ID]]
	tIDF IDF
	uIDF IDF
}

func newAutoItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time, tIDF, uIDF []float32) (ItemToItem, error) {
	a := &autoItemToItem{
		tIDF: tIDF,
		uIDF: uIDF,
	}
	a.baseItemToItem = baseItemToItem[lo.Tuple2[[]dataset.ID, []dataset.ID]]{
		name:      cfg.Name,
		n:         n,
		timestamp: timestamp,
		index:     ann.NewHNSW[lo.Tuple2[[]dataset.ID, []dataset.ID]](a.distance),
	}
	return a, nil
}

func (a *autoItemToItem) Push(item *data.Item, feedback []dataset.ID) {
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
	_ = a.index.Add(lo.Tuple2[[]dataset.ID, []dataset.ID]{A: v, B: feedback})
}

func (a *autoItemToItem) distance(u, v lo.Tuple2[[]dataset.ID, []dataset.ID]) float32 {
	return (a.tIDF.distance(u.A, v.A) + a.uIDF.distance(u.B, v.B)) / 2
}

type IDF []float32

func (idf IDF) distance(a, b []dataset.ID) float32 {
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

func (idf IDF) weightedSumCommonElements(a, b []dataset.ID) (float32, float32) {
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

func (idf IDF) weightedSum(a []dataset.ID) float32 {
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
