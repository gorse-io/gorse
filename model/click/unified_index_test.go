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
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUnifiedMapIndex(t *testing.T) {
	// create unified map index
	builder := NewUnifiedMapIndexBuilder()
	numUsers, numItems, numUserLabels, numItemLabels, numCtxLabels := 3, 4, 5, 6, 7
	for i := 0; i < numUsers; i++ {
		builder.AddUser(fmt.Sprintf("user%v", i))
	}
	for i := 0; i < numItems; i++ {
		builder.AddItem(fmt.Sprintf("item%v", i))
	}
	for i := 0; i < numUserLabels; i++ {
		builder.AddUserLabel(fmt.Sprintf("user_label%v", i))
	}
	for i := 0; i < numItemLabels; i++ {
		builder.AddItemLabel(fmt.Sprintf("item_label%v", i))
	}
	for i := 0; i < numCtxLabels; i++ {
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
	for i := 0; i < numUsers; i++ {
		userIndex := index.EncodeUser(fmt.Sprintf("user%v", i))
		assert.Equal(t, i, userIndex)
		assert.Equal(t, fmt.Sprintf("user%v", i), users[i])
	}
	items := index.GetItems()
	for i := 0; i < numItems; i++ {
		itemIndex := index.EncodeItem(fmt.Sprintf("item%v", i))
		assert.Equal(t, numUsers+i, itemIndex)
		assert.Equal(t, fmt.Sprintf("item%v", i), items[i])
	}
	userLabels := index.GetUserLabels()
	for i := 0; i < numUserLabels; i++ {
		userLabelIndex := index.EncodeUserLabel(fmt.Sprintf("user_label%v", i))
		assert.Equal(t, numUsers+numItems+i, userLabelIndex)
		assert.Equal(t, fmt.Sprintf("user_label%v", i), userLabels[i])
	}
	itemLabels := index.GetItemLabels()
	for i := 0; i < numItemLabels; i++ {
		itemLabelIndex := index.EncodeItemLabel(fmt.Sprintf("item_label%v", i))
		assert.Equal(t, numUsers+numItems+numUserLabels+i, itemLabelIndex)
		assert.Equal(t, fmt.Sprintf("item_label%v", i), itemLabels[i])
	}
	ctxLabels := index.GetContextLabels()
	for i := 0; i < numCtxLabels; i++ {
		ctxLabelIndex := index.EncodeContextLabel(fmt.Sprintf("ctx_label%v", i))
		assert.Equal(t, numUsers+numItems+numUserLabels+numItemLabels+i, ctxLabelIndex)
		assert.Equal(t, fmt.Sprintf("ctx_label%v", i), ctxLabels[i])
	}
}
