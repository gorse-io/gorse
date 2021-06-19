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
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/storage/data"
	"testing"
)

func TestLoadDataFromBuiltIn(t *testing.T) {
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.Nil(t, err)
	assert.Equal(t, 202027, train.Count())
	assert.Equal(t, 28860, test.Count())
}

type mockDatastore struct {
	data.Database
	server *miniredis.Miniredis
}

func newMockDatastore(t *testing.T) *mockDatastore {
	var err error
	db := new(mockDatastore)
	db.server, err = miniredis.Run()
	assert.Nil(t, err)
	db.Database, err = data.Open("redis://" + db.server.Addr())
	assert.Nil(t, err)
	return db
}

func (db *mockDatastore) Close(t *testing.T) {
	err := db.Database.Close()
	assert.Nil(t, err)
	db.server.Close()
}

func TestLoadDataFromDatabase(t *testing.T) {
	// create database
	database := newMockDatastore(t)
	defer database.Close(t)
	numUsers, numItems := 5, 6
	for i := 0; i < numUsers; i++ {
		err := database.InsertUser(data.User{
			UserId: fmt.Sprintf("user%v", i),
			Labels: []string{
				fmt.Sprintf("user_label%v", i),
				fmt.Sprintf("user_label%v", i*2),
			},
		})
		assert.Nil(t, err)
	}
	for i := 0; i < numItems; i++ {
		err := database.InsertItem(data.Item{
			ItemId: fmt.Sprintf("item%v", i),
			Labels: []string{
				fmt.Sprintf("item_label%v", i),
				fmt.Sprintf("item_label%v", i*2),
			},
		})
		assert.Nil(t, err)
	}
	for i := 0; i < numUsers; i++ {
		for j := 0; j < numItems; j++ {
			var err error
			err = database.InsertFeedback(data.Feedback{
				FeedbackKey: data.FeedbackKey{
					UserId:       fmt.Sprintf("user%v", i),
					ItemId:       fmt.Sprintf("item%v", j),
					FeedbackType: "read",
				},
			}, false, false)
			assert.Nil(t, err)
			if i+j > 4 {
				err = database.InsertFeedback(data.Feedback{
					FeedbackKey: data.FeedbackKey{
						UserId:       fmt.Sprintf("user%v", i),
						ItemId:       fmt.Sprintf("item%v", j),
						FeedbackType: "click",
					},
				}, false, false)
				assert.Nil(t, err)
			}
		}
	}
	// load data
	dataset, err := LoadDataFromDatabase(database.Database, []string{"click"}, "read")
	assert.Nil(t, err)
	assert.Equal(t, numUsers*numItems, dataset.Count())
	assert.Equal(t, numUsers, dataset.UserCount())
	assert.Equal(t, numItems, dataset.ItemCount())
	// split
	train, test := dataset.Split(0.2, 0)
	assert.Equal(t, numUsers, train.UserCount())
	assert.Equal(t, numItems, train.ItemCount())
	assert.Equal(t, 24, train.Count())
	assert.Equal(t, numUsers, test.UserCount())
	assert.Equal(t, numItems, test.ItemCount())
	assert.Equal(t, 6, test.Count())
}
