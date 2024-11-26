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
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base/heap"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"reflect"
	"time"
)

type LeaderBoard struct {
	timestamp  time.Time
	scoreFunc  *vm.Program
	filterFunc *vm.Program
	heap       *heap.TopKFilter[cache.Score, float64]
}

func NewLeaderBoard(cfg config.NonPersonalizedConfig, n int, timestamp time.Time) (*LeaderBoard, error) {
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
	return &LeaderBoard{
		timestamp:  timestamp,
		scoreFunc:  scoreFunc,
		filterFunc: filterFunc,
		heap:       heap.NewTopKFilter[cache.Score, float64](n),
	}, nil
}

func NewLatest(n int, timestamp time.Time) *LeaderBoard {
	return lo.Must(NewLeaderBoard(config.NonPersonalizedConfig{
		Name:  "latest",
		Score: "item.Timestamp.Unix()",
	}, n, timestamp))
}

func NewPopular(n int, timestamp time.Time) *LeaderBoard {
	return lo.Must(NewLeaderBoard(config.NonPersonalizedConfig{
		Name:  "popular",
		Score: "len(feedback)",
	}, n, timestamp))
}

func (l *LeaderBoard) Push(item data.Item, feedback []data.Feedback) {
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
	l.heap.Push(cache.Score{
		Id:         item.ItemId,
		Score:      score,
		IsHidden:   item.IsHidden,
		Categories: item.Categories,
		Timestamp:  l.timestamp,
	}, score)
}

func (l *LeaderBoard) PopAll() []cache.Score {
	scores, _ := l.heap.PopAll()
	return scores
}
