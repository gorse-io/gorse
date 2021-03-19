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

func init() {
	gob.Register(&UnifiedMapIndex{})
}

type UnifiedIndex interface {
	Len() int
	EncodeUser(userId string) int
	EncodeItem(itemId string) int
	EncodeUserLabel(userLabel string) int
	EncodeItemLabel(itemLabel string) int
	EncodeContextLabel(ctxLabel string) int
	GetUsers() []string
	GetItems() []string
	GetUserLabels() []string
	GetItemLabels() []string
	GetContextLabels() []string
	CountUsers() int
	CountItems() int
	CountUserLabels() int
	CountItemLabels() int
	CountContextLabels() int
}

// UnifiedMapIndexBuilder is the id -> index mapper for factorization machines.
// The division of id is: | user | item | user label | item label | context label |
type UnifiedMapIndexBuilder struct {
	UserIndex      base.Index
	ItemIndex      base.Index
	UserLabelIndex base.Index
	ItemLabelIndex base.Index
	CtxLabelIndex  base.Index
}

func NewUnifiedMapIndexBuilder() *UnifiedMapIndexBuilder {
	return &UnifiedMapIndexBuilder{
		UserIndex:      base.NewMapIndex(),
		ItemIndex:      base.NewMapIndex(),
		UserLabelIndex: base.NewMapIndex(),
		ItemLabelIndex: base.NewMapIndex(),
		CtxLabelIndex:  base.NewMapIndex(),
	}
}

func (builder *UnifiedMapIndexBuilder) AddUser(userId string) {
	builder.UserIndex.Add(userId)
}

func (builder *UnifiedMapIndexBuilder) AddItem(itemId string) {
	builder.ItemIndex.Add(itemId)
}

func (builder *UnifiedMapIndexBuilder) AddUserLabel(userLabel string) {
	builder.UserLabelIndex.Add(userLabel)
}

func (builder *UnifiedMapIndexBuilder) AddItemLabel(itemLabel string) {
	builder.ItemLabelIndex.Add(itemLabel)
}

func (builder *UnifiedMapIndexBuilder) AddCtxLabel(ctxLabel string) {
	builder.CtxLabelIndex.Add(ctxLabel)
}

func (builder *UnifiedMapIndexBuilder) Build() UnifiedIndex {
	return &UnifiedMapIndex{
		UserIndex:      builder.UserIndex,
		ItemIndex:      builder.ItemIndex,
		UserLabelIndex: builder.UserLabelIndex,
		ItemLabelIndex: builder.ItemLabelIndex,
		CtxLabelIndex:  builder.CtxLabelIndex,
	}
}

type UnifiedMapIndex struct {
	UserIndex      base.Index
	ItemIndex      base.Index
	UserLabelIndex base.Index
	ItemLabelIndex base.Index
	CtxLabelIndex  base.Index
}

func (unified *UnifiedMapIndex) GetUserLabels() []string {
	return unified.UserLabelIndex.GetNames()
}

func (unified *UnifiedMapIndex) GetItemLabels() []string {
	return unified.ItemLabelIndex.GetNames()
}

func (unified *UnifiedMapIndex) GetContextLabels() []string {
	return unified.CtxLabelIndex.GetNames()
}

func (unified *UnifiedMapIndex) CountUserLabels() int {
	return unified.UserLabelIndex.Len()
}

func (unified *UnifiedMapIndex) CountItemLabels() int {
	return unified.ItemLabelIndex.Len()
}

func (unified *UnifiedMapIndex) CountContextLabels() int {
	return unified.CtxLabelIndex.Len()
}

func (unified *UnifiedMapIndex) Len() int {
	return unified.UserIndex.Len() + unified.ItemIndex.Len() +
		unified.UserLabelIndex.Len() + unified.ItemLabelIndex.Len() +
		unified.CtxLabelIndex.Len()
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

func (unified *UnifiedMapIndex) EncodeUserLabel(userLabel string) int {
	userLabelIndex := unified.UserLabelIndex.ToNumber(userLabel)
	if userLabelIndex != base.NotId {
		userLabelIndex += unified.UserIndex.Len() + unified.ItemIndex.Len()
	}
	return userLabelIndex
}

func (unified *UnifiedMapIndex) EncodeItemLabel(itemLabel string) int {
	itemLabelIndex := unified.ItemLabelIndex.ToNumber(itemLabel)
	if itemLabelIndex != base.NotId {
		itemLabelIndex += unified.UserIndex.Len() + unified.ItemIndex.Len() + unified.UserLabelIndex.Len()
	}
	return itemLabelIndex
}

func (unified *UnifiedMapIndex) EncodeContextLabel(label string) int {
	ctxLabelIndex := unified.CtxLabelIndex.ToNumber(label)
	if ctxLabelIndex != base.NotId {
		ctxLabelIndex += unified.UserIndex.Len() + unified.ItemIndex.Len() +
			unified.UserLabelIndex.Len() + unified.ItemLabelIndex.Len()
	}
	return ctxLabelIndex
}

func (unified *UnifiedMapIndex) GetUsers() []string {
	return unified.UserIndex.GetNames()
}

func (unified *UnifiedMapIndex) GetItems() []string {
	return unified.ItemIndex.GetNames()
}

func (unified *UnifiedMapIndex) CountUsers() int {
	return unified.UserIndex.Len()
}

func (unified *UnifiedMapIndex) CountItems() int {
	return unified.ItemIndex.Len()
}

type UnifiedDirectIndex struct {
	N int
}

func (unified *UnifiedDirectIndex) EncodeUserLabel(userLabel string) int {
	panic("implement me")
}

func (unified *UnifiedDirectIndex) EncodeItemLabel(itemLabel string) int {
	panic("implement me")
}

func (unified *UnifiedDirectIndex) GetUserLabels() []string {
	panic("implement me")
}

func (unified *UnifiedDirectIndex) GetItemLabels() []string {
	panic("implement me")
}

func (unified *UnifiedDirectIndex) GetContextLabels() []string {
	panic("implement me")
}

func (unified *UnifiedDirectIndex) CountUserLabels() int {
	panic("implement me")
}

func (unified *UnifiedDirectIndex) CountItemLabels() int {
	panic("implement me")
}

func (unified *UnifiedDirectIndex) CountContextLabels() int {
	panic("implement me")
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

func (unified *UnifiedDirectIndex) EncodeContextLabel(label string) int {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) GetUsers() []string {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) GetItems() []string {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) CountUsers() int {
	panic("not implemented")
}

func (unified *UnifiedDirectIndex) CountItems() int {
	panic("not implemented")
}
