// Copyright 2021 gorse Project Authors
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
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUnifiedMapIndex(t *testing.T) {
	// create unified map index
	builder := NewUnifiedMapIndexBuilder()
	var numUsers, numItems, numUserLabels, numItemLabels, numCtxLabels int32 = 3, 4, 5, 6, 7
	for i := int32(0); i < numUsers; i++ {
		builder.AddUser(fmt.Sprintf("user%v", i))
	}
	for i := int32(0); i < numItems; i++ {
		builder.AddItem(fmt.Sprintf("item%v", i))
	}
	for i := int32(0); i < numUserLabels; i++ {
		builder.AddUserLabel(fmt.Sprintf("user_label%v", i))
	}
	for i := int32(0); i < numItemLabels; i++ {
		builder.AddItemLabel(fmt.Sprintf("item_label%v", i))
	}
	for i := int32(0); i < numCtxLabels; i++ {
		builder.AddCtxLabel(fmt.Sprintf("ctx_label%v", i))
	}
	index := builder.Build()
	// check count
	assert.Equal(t, numUsers+numItems+numUserLabels+numItemLabels+numCtxLabels, index.Len())
	assert.Equal(t, numUsers, index.CountUsers())
	assert.Equal(t, numItems, index.CountItems())
	assert.Equal(t, numUserLabels, index.CountUserLabels())
	assert.Equal(t, numItemLabels, index.CountItemLabels())
	assert.Equal(t, numCtxLabels, index.CountContextLabels())
	// check encode
	users := index.GetUsers()
	for i := int32(0); i < numUsers; i++ {
		userIndex := index.EncodeUser(fmt.Sprintf("user%v", i))
		assert.Equal(t, i, userIndex)
		assert.Equal(t, fmt.Sprintf("user%v", i), users[i])
	}
	items := index.GetItems()
	for i := int32(0); i < numItems; i++ {
		itemIndex := index.EncodeItem(fmt.Sprintf("item%v", i))
		assert.Equal(t, numUsers+i, itemIndex)
		assert.Equal(t, fmt.Sprintf("item%v", i), items[i])
	}
	userLabels := index.GetUserLabels()
	for i := int32(0); i < numUserLabels; i++ {
		userLabelIndex := index.EncodeUserLabel(fmt.Sprintf("user_label%v", i))
		assert.Equal(t, numUsers+numItems+i, userLabelIndex)
		assert.Equal(t, fmt.Sprintf("user_label%v", i), userLabels[i])
	}
	itemLabels := index.GetItemLabels()
	for i := int32(0); i < numItemLabels; i++ {
		itemLabelIndex := index.EncodeItemLabel(fmt.Sprintf("item_label%v", i))
		assert.Equal(t, numUsers+numItems+numUserLabels+i, itemLabelIndex)
		assert.Equal(t, fmt.Sprintf("item_label%v", i), itemLabels[i])
	}
	ctxLabels := index.GetContextLabels()
	for i := int32(0); i < numCtxLabels; i++ {
		ctxLabelIndex := index.EncodeContextLabel(fmt.Sprintf("ctx_label%v", i))
		assert.Equal(t, numUsers+numItems+numUserLabels+numItemLabels+i, ctxLabelIndex)
		assert.Equal(t, fmt.Sprintf("ctx_label%v", i), ctxLabels[i])
	}
	// Encode and decode
	buf := bytes.NewBuffer(nil)
	err := MarshalIndex(buf, index)
	assert.NoError(t, err)
	indexCopy, err := UnmarshalIndex(buf)
	assert.NoError(t, err)
	assert.Equal(t, index, indexCopy)
}

func TestUnifiedDirectIndex(t *testing.T) {
	index := NewUnifiedDirectIndex(10)
	assert.Equal(t, int32(10), index.Len())
	assert.Equal(t, []string{"0", "1"}, index.GetItems())
	assert.Equal(t, []string{"2", "3"}, index.GetUsers())
	assert.Equal(t, []string{"4", "5"}, index.GetItemLabels())
	assert.Equal(t, []string{"6", "7"}, index.GetUserLabels())
	assert.Equal(t, []string{"8", "9"}, index.GetContextLabels())
	assert.Equal(t, int32(2), index.CountItems())
	assert.Equal(t, int32(2), index.CountUsers())
	assert.Equal(t, int32(2), index.CountItemLabels())
	assert.Equal(t, int32(2), index.CountUserLabels())
	assert.Equal(t, int32(2), index.CountContextLabels())
	assert.Panics(t, func() { index.EncodeItem("abc") })
	assert.Panics(t, func() { index.EncodeUser("abc") })
	assert.Panics(t, func() { index.EncodeItemLabel("abc") })
	assert.Panics(t, func() { index.EncodeUserLabel("abc") })
	assert.Panics(t, func() { index.EncodeContextLabel("abc") })
	assert.Equal(t, int32(1), index.EncodeItem("1"))
	assert.Equal(t, int32(2), index.EncodeUser("2"))
	assert.Equal(t, int32(3), index.EncodeItemLabel("3"))
	assert.Equal(t, int32(4), index.EncodeUserLabel("4"))
	assert.Equal(t, int32(5), index.EncodeContextLabel("5"))
	// Encode and decode
	buf := bytes.NewBuffer(nil)
	err := MarshalIndex(buf, index)
	assert.NoError(t, err)
	indexCopy, err := UnmarshalIndex(buf)
	assert.NoError(t, err)
	assert.Equal(t, index, indexCopy)
}
