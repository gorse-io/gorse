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
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base/floats"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/common/ann"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"time"
)

type ItemToItem struct {
	name       string
	n          int
	timestamp  time.Time
	columnFunc *vm.Program
	index      *ann.HNSW[float32]
	items      []string
	dimension  int
}

func NewItemToItem(cfg config.ItemToItemConfig, n int, timestamp time.Time) (*ItemToItem, error) {
	// Compile column expression
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{
		"item": data.Item{},
	}))
	if err != nil {
		return nil, err
	}
	return &ItemToItem{
		name:       cfg.Name,
		n:          n,
		timestamp:  timestamp,
		columnFunc: columnFunc,
		index:      ann.NewHNSW[float32](floats.Euclidean),
	}, nil
}

func (i *ItemToItem) Push(item data.Item) {
	// Check if hidden
	if item.IsHidden {
		return
	}
	// Evaluate filter function
	result, err := expr.Run(i.columnFunc, map[string]any{
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
	if i.dimension == 0 && len(v) > 0 {
		i.dimension = len(v)
	} else if i.dimension != len(v) {
		log.Logger().Error("invalid column dimension", zap.Int("dimension", len(v)))
		return
	}
	// Push item
	i.items = append(i.items, item.ItemId)
	_, err = i.index.Add(v)
	if err != nil {
		log.Logger().Error("failed to add item to index", zap.Error(err))
		return
	}
}

func (i *ItemToItem) PopAll(callback func(itemId string, score []cache.Score)) {
	for index, item := range i.items {
		scores, err := i.index.SearchIndex(index, i.n, true)
		if err != nil {
			log.Logger().Error("failed to search index", zap.Error(err))
			return
		}
		callback(item, lo.Map(scores, func(v lo.Tuple2[int, float32], _ int) cache.Score {
			return cache.Score{
				Id:        i.items[v.A],
				Score:     float64(v.B),
				Timestamp: i.timestamp,
			}
		}))
	}
}
