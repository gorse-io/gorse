// Copyright 2020 gorse Project Authors
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
package rank

import (
	"encoding/gob"
	"github.com/zhenghaoz/gorse/base"
)

type EntityType int

const (
	EntityUser EntityType = iota
	EntityItem
	EntityLabel
)

func init() {
	gob.Register(&UnifiedMapIndex{})
}

type UnifiedIndex interface {
	Len() int
	EncodeUser(userId string) int
	EncodeItem(itemId string) int
	EncodeLabel(label string) int
	GetUsers() []string
	GetItems() []string
	GetLabels() []string
	CountUsers() int
	CountItems() int
	CountLabels() int
}

type UnifiedMapIndexBuilder struct {
	UserIndex  base.Index
	ItemIndex  base.Index
	LabelIndex base.Index
}

func NewUnifiedMapIndexBuilder() *UnifiedMapIndexBuilder {
	return &UnifiedMapIndexBuilder{
		UserIndex:  base.NewMapIndex(),
		ItemIndex:  base.NewMapIndex(),
		LabelIndex: base.NewMapIndex(),
	}
}

func (builder *UnifiedMapIndexBuilder) AddUser(userId string) {
	builder.UserIndex.Add(userId)
}

func (builder *UnifiedMapIndexBuilder) AddItem(itemId string) {
	builder.ItemIndex.Add(itemId)
}

func (builder *UnifiedMapIndexBuilder) AddLabel(label string) {
	builder.LabelIndex.Add(label)
}

func (builder *UnifiedMapIndexBuilder) Build() UnifiedIndex {
	return &UnifiedMapIndex{
		UserIndex:  builder.UserIndex,
		ItemIndex:  builder.ItemIndex,
		LabelIndex: builder.LabelIndex,
	}
}

type UnifiedMapIndex struct {
	UserIndex  base.Index
	ItemIndex  base.Index
	LabelIndex base.Index
}

func (unified *UnifiedMapIndex) Len() int {
	return unified.UserIndex.Len() + unified.ItemIndex.Len() + unified.LabelIndex.Len()
}

func (unified *UnifiedMapIndex) EncodeUser(userId string) int {
	return unified.UserIndex.ToNumber(userId)
}

func (unified *UnifiedMapIndex) EncodeItem(itemId string) int {
	itemIndex := unified.ItemIndex.ToNumber(itemId)
	if itemIndex != base.NotId {
		itemIndex += unified.UserIndex.Len()
	}
	return itemIndex
}

func (unified *UnifiedMapIndex) EncodeLabel(label string) int {
	labelIndex := unified.LabelIndex.ToNumber(label)
	if labelIndex != base.NotId {
		labelIndex += unified.UserIndex.Len() + unified.ItemIndex.Len()
	}
	return labelIndex
}

func (unified *UnifiedMapIndex) GetUsers() []string {
	return unified.UserIndex.GetNames()
}

func (unified *UnifiedMapIndex) GetItems() []string {
	return unified.ItemIndex.GetNames()
}

func (unified *UnifiedMapIndex) GetLabels() []string {
	return unified.LabelIndex.GetNames()
}

func (unified *UnifiedMapIndex) CountUsers() int {
	return unified.UserIndex.Len()
}

func (unified *UnifiedMapIndex) CountItems() int {
	return unified.ItemIndex.Len()
}

func (unified *UnifiedMapIndex) CountLabels() int {
	return unified.LabelIndex.Len()
}

type UnifiedDirectIndex struct {
	N int
}

func NewUnifiedDirectIndex(n int) UnifiedIndex {
	return &UnifiedDirectIndex{N: n}
}

func (unified *UnifiedDirectIndex) Len() int {
	return unified.N
}

func (unified *UnifiedDirectIndex) EncodeUser(userId string) int {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) EncodeItem(itemId string) int {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) EncodeLabel(label string) int {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) GetUsers() []string {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) GetItems() []string {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) GetLabels() []string {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) CountUsers() int {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) CountItems() int {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) CountLabels() int {
	panic("not implemented")
}
