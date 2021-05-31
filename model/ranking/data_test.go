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
package ranking

import (
	"fmt"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/storage/data"
	"strconv"
	"testing"
)

func TestNewMapIndexDataset(t *testing.T) {
	dataSet := NewMapIndexDataset()
	for i := 0; i < 4; i++ {
		for j := i; j < 5; j++ {
			dataSet.AddFeedback(strconv.Itoa(i), strconv.Itoa(j), true)
		}
	}
	assert.Equal(t, 14, dataSet.Count())
	assert.Equal(t, 4, dataSet.UserCount())
	assert.Equal(t, 5, dataSet.ItemCount())
	dataSet.AddUser("10")
	dataSet.AddItem("10")
	assert.Equal(t, 5, dataSet.UserCount())
	assert.Equal(t, 6, dataSet.ItemCount())
}

func TestLoadDataFromCSV(t *testing.T) {
	dataset := LoadDataFromCSV("../../misc/csv_test/feedback.csv", ",", true)
	assert.Equal(t, 5, dataset.Count())
	for i := 0; i < dataset.Count(); i++ {
		userIndex, itemIndex := dataset.GetIndex(i)
		assert.Equal(t, i, userIndex)
		assert.Equal(t, i, itemIndex)
	}
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
	numUsers, numItems := 3, 5
	for i := 0; i < numUsers; i++ {
		err := database.InsertUser(data.User{
			UserId: fmt.Sprintf("user%v", i),
		})
		assert.Nil(t, err)
	}
	for i := 0; i < numItems; i++ {
		err := database.InsertItem(data.Item{
			ItemId: fmt.Sprintf("item%v", i),
		})
		assert.Nil(t, err)
	}
	for i := 0; i < numUsers; i++ {
		for j := i + 1; j < numItems; j++ {
			err := database.InsertFeedback(data.Feedback{
				FeedbackKey: data.FeedbackKey{
					UserId:       fmt.Sprintf("user%v", i),
					ItemId:       fmt.Sprintf("item%v", j),
					FeedbackType: "FeedbackType",
				},
			}, false, false)
			assert.Nil(t, err)
		}
	}
	// load data
	dataset, _, _, err := LoadDataFromDatabase(database.Database, []string{"FeedbackType"}, 0, 0)
	assert.Nil(t, err)
	assert.Equal(t, 9, dataset.Count())
	// split
	train, test := dataset.Split(0, 0)
	assert.Equal(t, numUsers, train.UserCount())
	assert.Equal(t, numItems, train.ItemCount())
	assert.Equal(t, 9-numUsers, train.Count())
	assert.Equal(t, numUsers, test.UserCount())
	assert.Equal(t, numItems, test.ItemCount())
	assert.Equal(t, numUsers, test.Count())
	// part split
	train2, test2 := dataset.Split(2, 0)
	assert.Equal(t, numUsers, train2.UserCount())
	assert.Equal(t, numItems, train2.ItemCount())
	assert.Equal(t, 7, train2.Count())
	assert.Equal(t, numUsers, test2.UserCount())
	assert.Equal(t, numItems, test2.ItemCount())
	assert.Equal(t, 2, test2.Count())
}
