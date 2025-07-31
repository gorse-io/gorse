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
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/gorse-io/gorse/base/heap"
	"github.com/gorse-io/gorse/base/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type NonPersonalized struct {
	sync.Mutex
	name       string
	timestamp  time.Time
	scoreFunc  *vm.Program
	filterFunc *vm.Program
	heapSize   int
	heaps      map[string]*heap.TopKFilter[string, float64]
}

func NewNonPersonalized(cfg config.NonPersonalizedConfig, n int, timestamp time.Time) (*NonPersonalized, error) {
	// Compile score expression
	scoreFunc, err := expr.Compile(cfg.Score, expr.Env(map[string]any{
		"item":     data.Item{},
		"feedback": []data.Feedback{},
	}))
	if err != nil {
		return nil, err
	}
	switch scoreFunc.Node().Type().Kind() {
	case reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
	default:
		return nil, errors.New("score function must return float64")
	}
	// Compile filter expression
	var filterFunc *vm.Program
	if cfg.Filter != "" {
		filterFunc, err = expr.Compile(cfg.Filter, expr.Env(map[string]any{
			"item":     data.Item{},
			"feedback": []data.Feedback{},
		}))
		if err != nil {
			return nil, err
		}
		if filterFunc.Node().Type().Kind() != reflect.Bool {
			return nil, errors.New("filter function must return bool")
		}
	}
	// Initialize heap
	heaps := make(map[string]*heap.TopKFilter[string, float64])
	heaps[""] = heap.NewTopKFilter[string, float64](n)
	return &NonPersonalized{
		name:       cfg.Name,
		timestamp:  timestamp,
		scoreFunc:  scoreFunc,
		filterFunc: filterFunc,
		heapSize:   n,
		heaps:      heaps,
	}, nil
}

func NewLatest(n int, timestamp time.Time) *NonPersonalized {
	return lo.Must(NewNonPersonalized(config.NonPersonalizedConfig{
		Name:  "latest",
		Score: "item.Timestamp.Unix()",
	}, n, timestamp))
}

func NewPopular(window time.Duration, n int, timestamp time.Time) *NonPersonalized {
	var filter string
	if window > 0 {
		filter = fmt.Sprintf("(now() - item.Timestamp).Nanoseconds() < %d", window.Nanoseconds())
	}
	return lo.Must(NewNonPersonalized(config.NonPersonalizedConfig{
		Name:   "popular",
		Score:  "len(feedback)",
		Filter: filter,
	}, n, timestamp))
}

func (l *NonPersonalized) Push(item data.Item, feedback []data.Feedback) {
	// Skip hidden items
	if item.IsHidden {
		return
	}
	// Evaluate filter function
	if l.filterFunc != nil {
		result, err := expr.Run(l.filterFunc, map[string]any{
			"item":     item,
			"feedback": feedback,
		})
		if err != nil {
			log.Logger().Error("evaluate filter function", zap.Error(err))
			return
		}
		if !result.(bool) {
			return
		}
	}
	// Evaluate score function
	result, err := expr.Run(l.scoreFunc, map[string]any{
		"item":     item,
		"feedback": feedback,
	})
	if err != nil {
		log.Logger().Error("evaluate score function", zap.Error(err))
		return
	}
	var score float64
	switch typed := result.(type) {
	case float64:
		score = typed
	case int:
		score = float64(typed)
	case int8:
		score = float64(typed)
	case int16:
		score = float64(typed)
	case int32:
		score = float64(typed)
	case int64:
		score = float64(typed)
	default:
		log.Logger().Error("score function must return float64", zap.Any("result", result))
		return
	}
	// Add to heap
	l.Lock()
	defer l.Unlock()
	l.heaps[""].Push(item.ItemId, score)
	for _, group := range item.Categories {
		if _, exist := l.heaps[group]; !exist {
			l.heaps[group] = heap.NewTopKFilter[string, float64](l.heapSize)
		}
		l.heaps[group].Push(item.ItemId, score)
	}
}

func (l *NonPersonalized) PopAll() []cache.Score {
	scores := make(map[string]*cache.Score)
	l.Lock()
	defer l.Unlock()
	for category, h := range l.heaps {
		names, values := h.PopAll()
		for i, name := range names {
			if _, exist := scores[name]; !exist {
				scores[name] = &cache.Score{
					Id:         name,
					Score:      values[i],
					Categories: []string{category},
					Timestamp:  l.timestamp,
				}
			} else {
				scores[name].Categories = append(scores[name].Categories, category)
			}
		}
	}
	result := lo.MapToSlice(scores, func(_ string, v *cache.Score) cache.Score {
		return *v
	})
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})
	return result
}

func (l *NonPersonalized) Name() string {
	return l.name
}

func (l *NonPersonalized) Timestamp() time.Time {
	return l.timestamp
}
