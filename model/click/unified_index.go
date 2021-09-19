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

package click

import (
	"encoding/gob"
	"github.com/zhenghaoz/gorse/base"
	"strconv"
)

func init() {
	gob.Register(&UnifiedMapIndex{})
	gob.Register(&UnifiedDirectIndex{})
}

// UnifiedIndex maps users, items and labels into a unified encoding space.
type UnifiedIndex interface {
	Len() int32
	EncodeUser(userId string) int32
	EncodeItem(itemId string) int32
	EncodeUserLabel(userLabel string) int32
	EncodeItemLabel(itemLabel string) int32
	EncodeContextLabel(ctxLabel string) int32
	GetUsers() []string
	GetItems() []string
	GetUserLabels() []string
	GetItemLabels() []string
	GetContextLabels() []string
	CountUsers() int32
	CountItems() int32
	CountUserLabels() int32
	CountItemLabels() int32
	CountContextLabels() int32
}

// UnifiedMapIndexBuilder is the builder for UnifiedMapIndex.
type UnifiedMapIndexBuilder struct {
	UserIndex      base.Index
	ItemIndex      base.Index
	UserLabelIndex base.Index
	ItemLabelIndex base.Index
	CtxLabelIndex  base.Index
}

// NewUnifiedMapIndexBuilder creates a UnifiedMapIndexBuilder.
func NewUnifiedMapIndexBuilder() *UnifiedMapIndexBuilder {
	return &UnifiedMapIndexBuilder{
		UserIndex:      base.NewMapIndex(),
		ItemIndex:      base.NewMapIndex(),
		UserLabelIndex: base.NewMapIndex(),
		ItemLabelIndex: base.NewMapIndex(),
		CtxLabelIndex:  base.NewMapIndex(),
	}
}

// AddUser adds a user into the unified index.
func (builder *UnifiedMapIndexBuilder) AddUser(userId string) {
	builder.UserIndex.Add(userId)
}

// AddItem adds a item into the unified index.
func (builder *UnifiedMapIndexBuilder) AddItem(itemId string) {
	builder.ItemIndex.Add(itemId)
}

// AddUserLabel adds a user label into the unified index.
func (builder *UnifiedMapIndexBuilder) AddUserLabel(userLabel string) {
	builder.UserLabelIndex.Add(userLabel)
}

// AddItemLabel adds a item label into the unified index.
func (builder *UnifiedMapIndexBuilder) AddItemLabel(itemLabel string) {
	builder.ItemLabelIndex.Add(itemLabel)
}

// AddCtxLabel adds a context label the unified index.
func (builder *UnifiedMapIndexBuilder) AddCtxLabel(ctxLabel string) {
	builder.CtxLabelIndex.Add(ctxLabel)
}

// Build UnifiedMapIndex from UnifiedMapIndexBuilder.
func (builder *UnifiedMapIndexBuilder) Build() UnifiedIndex {
	return &UnifiedMapIndex{
		UserIndex:      builder.UserIndex,
		ItemIndex:      builder.ItemIndex,
		UserLabelIndex: builder.UserLabelIndex,
		ItemLabelIndex: builder.ItemLabelIndex,
		CtxLabelIndex:  builder.CtxLabelIndex,
	}
}

// UnifiedMapIndex is the id -> index mapper for factorization machines.
// The division of id is: | user | item | user label | item label | context label |
type UnifiedMapIndex struct {
	UserIndex      base.Index
	ItemIndex      base.Index
	UserLabelIndex base.Index
	ItemLabelIndex base.Index
	CtxLabelIndex  base.Index
}

// GetUserLabels returns all user labels.
func (unified *UnifiedMapIndex) GetUserLabels() []string {
	return unified.UserLabelIndex.GetNames()
}

// GetItemLabels returns all item labels.
func (unified *UnifiedMapIndex) GetItemLabels() []string {
	return unified.ItemLabelIndex.GetNames()
}

// GetContextLabels returns all context labels.
func (unified *UnifiedMapIndex) GetContextLabels() []string {
	return unified.CtxLabelIndex.GetNames()
}

// CountUserLabels returns the number of user labels.
func (unified *UnifiedMapIndex) CountUserLabels() int32 {
	return unified.UserLabelIndex.Len()
}

// CountItemLabels returns the number of item labels.
func (unified *UnifiedMapIndex) CountItemLabels() int32 {
	return unified.ItemLabelIndex.Len()
}

// CountContextLabels returns the number of context labels.
func (unified *UnifiedMapIndex) CountContextLabels() int32 {
	return unified.CtxLabelIndex.Len()
}

// Len returns the size of unified index.
func (unified *UnifiedMapIndex) Len() int32 {
	return unified.UserIndex.Len() + unified.ItemIndex.Len() +
		unified.UserLabelIndex.Len() + unified.ItemLabelIndex.Len() +
		unified.CtxLabelIndex.Len()
}

// EncodeUser converts a user id to a integer in the encoding space.
func (unified *UnifiedMapIndex) EncodeUser(userId string) int32 {
	return unified.UserIndex.ToNumber(userId)
}

// EncodeItem converts a item id to a integer in the encoding space.
func (unified *UnifiedMapIndex) EncodeItem(itemId string) int32 {
	itemIndex := unified.ItemIndex.ToNumber(itemId)
	if itemIndex != base.NotId {
		itemIndex += unified.UserIndex.Len()
	}
	return itemIndex
}

// EncodeUserLabel converts a user label to a integer in the encoding space.
func (unified *UnifiedMapIndex) EncodeUserLabel(userLabel string) int32 {
	userLabelIndex := unified.UserLabelIndex.ToNumber(userLabel)
	if userLabelIndex != base.NotId {
		userLabelIndex += unified.UserIndex.Len() + unified.ItemIndex.Len()
	}
	return userLabelIndex
}

// EncodeItemLabel converts a item label to a integer in the encoding space.
func (unified *UnifiedMapIndex) EncodeItemLabel(itemLabel string) int32 {
	itemLabelIndex := unified.ItemLabelIndex.ToNumber(itemLabel)
	if itemLabelIndex != base.NotId {
		itemLabelIndex += unified.UserIndex.Len() + unified.ItemIndex.Len() + unified.UserLabelIndex.Len()
	}
	return itemLabelIndex
}

// EncodeContextLabel converts a context label to a integer in the encoding space.
func (unified *UnifiedMapIndex) EncodeContextLabel(label string) int32 {
	ctxLabelIndex := unified.CtxLabelIndex.ToNumber(label)
	if ctxLabelIndex != base.NotId {
		ctxLabelIndex += unified.UserIndex.Len() + unified.ItemIndex.Len() +
			unified.UserLabelIndex.Len() + unified.ItemLabelIndex.Len()
	}
	return ctxLabelIndex
}

// GetUsers returns all users.
func (unified *UnifiedMapIndex) GetUsers() []string {
	return unified.UserIndex.GetNames()
}

// GetItems returns all items.
func (unified *UnifiedMapIndex) GetItems() []string {
	return unified.ItemIndex.GetNames()
}

// CountUsers returns the number of users.
func (unified *UnifiedMapIndex) CountUsers() int32 {
	return unified.UserIndex.Len()
}

// CountItems returns the number of items.
func (unified *UnifiedMapIndex) CountItems() int32 {
	return unified.ItemIndex.Len()
}

// UnifiedDirectIndex maps string to integer in literal.
type UnifiedDirectIndex struct {
	N int32
}

// EncodeUserLabel should be used by unit testing only.
func (unified *UnifiedDirectIndex) EncodeUserLabel(userLabel string) int32 {
	if val, err := strconv.Atoi(userLabel); err != nil {
		panic(err)
	} else {
		return int32(val)
	}
}

// EncodeItemLabel should be used by unit testing only.
func (unified *UnifiedDirectIndex) EncodeItemLabel(itemLabel string) int32 {
	if val, err := strconv.Atoi(itemLabel); err != nil {
		panic(err)
	} else {
		return int32(val)
	}
}

// GetUserLabels should be used by unit testing only.
func (unified *UnifiedDirectIndex) GetUserLabels() []string {
	var names []string
	begin, end := unified.N/5*3, unified.N/5*4
	for i := begin; i < end; i++ {
		names = append(names, strconv.Itoa(int(i)))
	}
	return names
}

// GetItemLabels should be used by unit testing only.
func (unified *UnifiedDirectIndex) GetItemLabels() []string {
	var names []string
	begin, end := unified.N/5*2, unified.N/5*3
	for i := begin; i < end; i++ {
		names = append(names, strconv.Itoa(int(i)))
	}
	return names
}

// GetContextLabels should be used by unit testing only.
func (unified *UnifiedDirectIndex) GetContextLabels() []string {
	var names []string
	begin, end := unified.N/5*4, unified.N
	for i := begin; i < end; i++ {
		names = append(names, strconv.Itoa(int(i)))
	}
	return names
}

// CountUserLabels should be used by unit testing only.
func (unified *UnifiedDirectIndex) CountUserLabels() int32 {
	return unified.N / 5
}

// CountItemLabels should be used by unit testing only.
func (unified *UnifiedDirectIndex) CountItemLabels() int32 {
	return unified.N / 5
}

// CountContextLabels should be used by unit testing only.
func (unified *UnifiedDirectIndex) CountContextLabels() int32 {
	return unified.N - unified.N/5*4
}

// NewUnifiedDirectIndex creates a UnifiedDirectIndex.
func NewUnifiedDirectIndex(n int32) UnifiedIndex {
	return &UnifiedDirectIndex{N: n}
}

// Len should be used by unit testing only.
func (unified *UnifiedDirectIndex) Len() int32 {
	return unified.N
}

// EncodeUser should be used by unit testing only.
func (unified *UnifiedDirectIndex) EncodeUser(userId string) int32 {
	if val, err := strconv.Atoi(userId); err != nil {
		panic(err)
	} else {
		return int32(val)
	}
}

// EncodeItem should be used by unit testing only.
func (unified *UnifiedDirectIndex) EncodeItem(itemId string) int32 {
	if val, err := strconv.Atoi(itemId); err != nil {
		panic(err)
	} else {
		return int32(val)
	}
}

// EncodeContextLabel should be used by unit testing only.
func (unified *UnifiedDirectIndex) EncodeContextLabel(label string) int32 {
	if val, err := strconv.Atoi(label); err != nil {
		panic(err)
	} else {
		return int32(val)
	}
}

// GetUsers should be used by unit testing only.
func (unified *UnifiedDirectIndex) GetUsers() []string {
	var names []string
	begin, end := unified.N/5, unified.N/5*2
	for i := begin; i < end; i++ {
		names = append(names, strconv.Itoa(int(i)))
	}
	return names
}

// GetItems should be used by unit testing only.
func (unified *UnifiedDirectIndex) GetItems() []string {
	base.Logger().Warn("")
	var names []string
	begin, end := int32(0), unified.N/5
	for i := begin; i < end; i++ {
		names = append(names, strconv.Itoa(int(i)))
	}
	return names
}

// CountUsers should be used by unit testing only.
func (unified *UnifiedDirectIndex) CountUsers() int32 {
	return unified.N / 5
}

// CountItems should be used by unit testing only.
func (unified *UnifiedDirectIndex) CountItems() int32 {
	return unified.N / 5
}
