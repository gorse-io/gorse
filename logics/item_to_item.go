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
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"time"
)

type ItemToItem struct {
	name       string
	timestamp  time.Time
	columnFunc *vm.Program
}

func NewItemToItem(cfg config.ItemToItemConfig) (*ItemToItem, error) {
	// Compile column expression
	columnFunc, err := expr.Compile(cfg.Column, expr.Env(map[string]any{
		"item": data.Item{},
	}))
	if err != nil {
		return nil, err
	}
	return &ItemToItem{
		columnFunc: columnFunc,
	}, nil
}

func (i *ItemToItem) Push(item data.Item) {
	// Evaluate filter function
	result, err := expr.Run(i.columnFunc, map[string]any{
		"item": item,
	})
	if err != nil {
		log.Logger().Error("evaluate filter function", zap.Error(err))
		return
	}
	fmt.Println(result)
}

func (i *ItemToItem) Fit() {

}
