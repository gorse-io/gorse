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

package recommend

import (
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/zhenghaoz/gorse/base/heap"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
)

type LeaderBoard struct {
	scoreFunc  *vm.Program
	filterFunc *vm.Program
	heap       heap.TopKFilter[cache.Document, float64]
}

func NewLeaderBoard(cfg config.LeaderBoardConfig) (*LeaderBoard, error) {
	// Compile score expression
	scoreFunc, err := expr.Compile(cfg.Score, expr.Env(map[string]any{
		"item":     data.Item{},
		"feedback": []data.Feedback{},
	}))
	if err != nil {
		return nil, err
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
	}
	return &LeaderBoard{
		scoreFunc:  scoreFunc,
		filterFunc: filterFunc,
	}, nil
}

func (l *LeaderBoard) Push(item data.Item, feedback []data.Feedback) {
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
	score := result.(float64)
	l.heap.Push(item, score)
}

func (l *LeaderBoard) PopAll() []data.Item {
	return nil
}
