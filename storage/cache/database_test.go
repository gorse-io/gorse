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
package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testMeta(t *testing.T, db Database) {
	// Set meta string
	if err := db.SetString("meta", "1", "2"); err != nil {
		t.Fatal(err)
	}
	// Get meta string
	value, err := db.GetString("meta", "1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "2", value)
	// Get meta not existed
	value, err = db.GetString("meta", "NULL")
	if err == nil {
		t.Fatal(err)
	}
	assert.Equal(t, "", value)
	// Set meta int
	if err = db.SetInt("meta", "1", 2); err != nil {
		t.Fatal(err)
	}
	// Get meta int
	if value, err := db.GetInt("meta", "1"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 2, value)
	}
}

func testList(t *testing.T, db Database) {
	// Put items
	items := []string{"0", "1", "2", "3", "4"}
	err := db.SetList("list", "0", items)
	assert.Nil(t, err)
	// Get items
	totalItems, err := db.GetList("list", "0", 0, -1)
	assert.Nil(t, err)
	assert.Equal(t, items, totalItems)
	// Get n items
	headItems, err := db.GetList("list", "0", 0, 2)
	assert.Nil(t, err)
	assert.Equal(t, items[:3], headItems)
	// Get n items with offset
	offsetItems, err := db.GetList("list", "0", 1, 3)
	assert.Nil(t, err)
	assert.Equal(t, items[1:4], offsetItems)
	// Get empty
	noItems, err := db.GetList("list", "1", 0, 0)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(noItems))
	// test overwrite
	overwriteItems := []string{"10", "11", "12", "13", "14"}
	err = db.SetList("list", "0", overwriteItems)
	assert.Nil(t, err)
	totalItems, err = db.GetList("list", "0", 0, -1)
	assert.Nil(t, err)
	assert.Equal(t, overwriteItems, totalItems)
}
