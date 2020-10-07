// Copyright 2020 Zhenghao Zhang
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
package database

import (
	"github.com/stretchr/testify/assert"
	"github.com/thanhpk/randstr"
	"github.com/zhenghaoz/gorse/model"
	"log"
	"os"
	"path"
	"strconv"
	"testing"
	"time"
)

func createTempDir() string {
	dir := path.Join(os.TempDir(), randstr.String(16))
	err := os.Mkdir(dir, 0777)
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func TestDB_InsertGetFeedback(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert ratings
	users := []string{"0", "1", "2", "3", "4"}
	items := []string{"0", "2", "4", "6", "8"}
	ratings := []float64{0, 3, 6, 9, 12}
	for i := range users {
		if err := db.InsertFeedback(Feedback{UserId: users[i], ItemId: items[i], Rating: ratings[i]}); err != nil {
			t.Fatal(err)
		}
	}
	// Count ratings
	count, err := db.CountFeedback()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Get ratings
	feedback, err := db.GetFeedback()
	if err != nil {
		t.Fatal(err)
	}
	for i := range users {
		assert.Equal(t, users[i], feedback[i].UserId)
		assert.Equal(t, items[i], feedback[i].ItemId)
		assert.Equal(t, ratings[i], feedback[i].Rating)
	}
	// Get item
	for _, itemId := range items {
		item, err := db.GetItem(itemId)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, Item{ItemId: itemId, Popularity: 1}, item)
	}
	// Get feedback by user
	feedback, err = db.GetFeedbackByUser("2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(feedback))
	assert.Equal(t, "2", feedback[0].UserId)
	assert.Equal(t, "4", feedback[0].ItemId)
	assert.Equal(t, float64(6), feedback[0].Rating)
	// Get feedback by item
	feedback, err = db.GetFeedbackByItem("4")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(feedback))
	assert.Equal(t, "2", feedback[0].UserId)
	assert.Equal(t, "4", feedback[0].ItemId)
	assert.Equal(t, float64(6), feedback[0].Rating)
}

func TestDB_BatchInsertGetFeedback(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert ratings
	users := []string{"0", "1", "2", "3", "4"}
	items := []string{"0", "2", "4", "6", "8"}
	ratings := []float64{0, 3, 6, 9, 12}
	feedback := make([]Feedback, len(users))
	for i := range users {
		feedback[i].UserId = users[i]
		feedback[i].ItemId = items[i]
		feedback[i].Rating = ratings[i]
	}
	if err := db.InsertFeedbacks(feedback); err != nil {
		t.Fatal(err)
	}
	// Count ratings
	count, err := db.CountFeedback()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Get ratings
	ret, err := db.GetFeedback()
	if err != nil {
		t.Fatal(err)
	}
	for i := range users {
		assert.Equal(t, users[i], ret[i].UserId)
		assert.Equal(t, items[i], ret[i].ItemId)
		assert.Equal(t, ratings[i], ret[i].Rating)
	}
	// Get item
	for _, itemId := range items {
		item, err := db.GetItem(itemId)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, Item{ItemId: itemId, Popularity: 1}, item)
	}
	// Get feedback by user
	feedback, err = db.GetFeedbackByUser("2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(feedback))
	assert.Equal(t, "2", feedback[0].UserId)
	assert.Equal(t, "4", feedback[0].ItemId)
	assert.Equal(t, float64(6), feedback[0].Rating)
	// Get feedback by item
	feedback, err = db.GetFeedbackByItem("4")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(feedback))
	assert.Equal(t, "2", feedback[0].UserId)
	assert.Equal(t, "4", feedback[0].ItemId)
	assert.Equal(t, float64(6), feedback[0].Rating)
}

func TestDB_InsertGetItem(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert items
	items := []Item{
		{
			ItemId:    "0",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
		},
		{
			ItemId:    "2",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
		},
		{
			ItemId:    "4",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a", "b"},
		},
		{
			ItemId:    "6",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
		},
		{
			ItemId:    "8",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
		},
	}
	for _, item := range items {
		if err := db.InsertItem(item, true); err != nil {
			t.Fatal(err)
		}
	}
	// Get items
	allItems, err := db.GetItems(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, allItems)
	// Get n items with offset
	nItems, err := db.GetItems(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[1:4], nItems)
	// Get item
	for _, item := range items {
		ret, err := db.GetItem(item.ItemId)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, item, ret)
	}
	// Get items by labels
	aItems, err := db.GetItemsByLabel("a")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:3], aItems)
	bItems, err := db.GetItemsByLabel("b")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[2:], bItems)
}

func TestDB_BatchInsertGetItem(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert items
	items := []Item{
		{
			ItemId:    "0",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
		},
		{
			ItemId:    "2",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
		},
		{
			ItemId:    "4",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a", "b"},
		},
		{
			ItemId:    "6",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
		},
		{
			ItemId:    "8",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
		},
	}
	if err := db.InsertItems(items, true); err != nil {
		t.Fatal(err)
	}
	// Get items
	allItems, err := db.GetItems(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, allItems)
	// Get n items with offset
	nItems, err := db.GetItems(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[1:4], nItems)
	// Get item
	for _, item := range items {
		ret, err := db.GetItem(item.ItemId)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, item, ret)
	}
	// Get items by labels
	aItems, err := db.GetItemsByLabel("a")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:3], aItems)
	bItems, err := db.GetItemsByLabel("b")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[2:], bItems)
}

func TestDB_SetGetMeta(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Set meta
	if err = db.SetString("1", "2"); err != nil {
		t.Fatal(err)
	}
	// Get meta
	value, err := db.GetString("1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "2", value)
	// Get meta not existed
	value, err = db.GetString("NULL")
	if err == nil {
		t.Fatal(err)
	}
	assert.Equal(t, "", value)
}

func TestDB_PutGetList(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Put recommends
	items := []RecommendedItem{
		{Item{ItemId: "0"}, 0.0},
		{Item{ItemId: "1"}, 0.1},
		{Item{ItemId: "2"}, 0.2},
		{Item{ItemId: "3"}, 0.3},
		{Item{ItemId: "4"}, 0.4},
	}
	if err = db.setList(prefixRecommends, "0", items); err != nil {
		t.Fatal(err)
	}
	// Get recommends
	retItems, err := db.getList(prefixRecommends, "0", 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, retItems)
	// Get n recommends
	nItems, err := db.getList(prefixRecommends, "0", 3, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:3], nItems)
	// Get n recommends with offset
	oItems, err := db.getList(prefixRecommends, "0", 3, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[1:4], oItems)
	// Test new user
	emptyItems, err := db.getList(prefixRecommends, "1", 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0, len(emptyItems))
}

func TestDB_ToDataSet(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert data
	feedback := []Feedback{
		{UserId: "1", ItemId: "1"},
		{UserId: "1", ItemId: "2"},
		{UserId: "2", ItemId: "1"},
		{UserId: "2", ItemId: "2"},
		{UserId: "2", ItemId: "3"},
	}
	if err = db.InsertFeedbacks(feedback); err != nil {
		t.Fatal(err)
	}
	items := []Item{
		{ItemId: "1", Labels: []string{"a"}},
		{ItemId: "2", Labels: []string{"b"}},
		{ItemId: "3", Labels: []string{"c"}},
	}
	if err = db.InsertItems(items, true); err != nil {
		t.Fatal(err)
	}
	// To dataset
	dataSet, err := db.ToDataSet()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, dataSet.Count())
	assert.Equal(t, 2, dataSet.UserCount())
	assert.Equal(t, 3, dataSet.ItemCount())
	assert.Equal(t, 3, dataSet.FeatureCount())
}

func TestDB_LoadFeedbackFromCSV(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Load data
	if err = db.LoadFeedbackFromCSV("../example/data/feedback_explicit_header.csv", ",", true); err != nil {
		t.Fatal(err)
	}
	// Count feedback
	count, err := db.CountFeedback()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Check data
	feedback, err := db.GetFeedback()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		assert.Equal(t, strconv.Itoa(i), feedback[i].UserId)
		assert.Equal(t, strconv.Itoa(2*i), feedback[i].ItemId)
		assert.Equal(t, 3*i, int(feedback[i].Rating))
	}
	// Count feedback
	count, err = db.CountItems()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Check data
	items, err := db.GetItems(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		assert.Equal(t, strconv.Itoa(2*i), items[i].ItemId)
	}
}

func TestDB_LoadItemsFromCSV(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Load data
	if err = db.LoadItemsFromCSV("../example/data/items.csv", "::", false, -1, "|", -1); err != nil {
		t.Fatal(err)
	}
	// Count feedback
	count, err := db.CountItems()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Check data
	items, err := db.GetItems(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		assert.Equal(t, strconv.Itoa(1+i), items[i].ItemId)
	}
}

func TestDB_LoadItemsFromCSV_Date(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Load data
	if err = db.LoadItemsFromCSV("../example/data/item_date.csv", ",", false, 1, "|", 2); err != nil {
		t.Fatal(err)
	}
	// Count feedback
	count, err := db.CountItems()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Check data
	items, err := db.GetItems(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		assert.Equal(t, strconv.Itoa(1+i), items[i].ItemId)
		date := time.Date(2020, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, date, items[i].Timestamp)
		if i%2 == 0 {
			assert.Equal(t, []string{"a", "b", "c"}, items[i].Labels)
		} else {
			assert.Equal(t, []string{"e", "f", "g"}, items[i].Labels)
		}
	}
}

func TestDB_SaveFeedbackToCSV(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Load data
	if err = db.LoadFeedbackFromCSV("../example/data/feedback_explicit_header.csv", ",", true); err != nil {
		t.Fatal(err)
	}
	// Save data
	csvFileName := path.Join(model.TempDir, randstr.String(16))
	if err = db.SaveFeedbackToCSV(csvFileName, ",", false); err != nil {
		t.Fatal(err)
	}
	// Check data
	data := model.LoadDataFromCSV(csvFileName, ",", false)
	assert.Equal(t, 5, data.Count())
	for i := 0; i < data.Count(); i++ {
		userId, itemId, value := data.Get(i)
		userIndex, itemIndex, _ := data.GetWithIndex(i)
		assert.Equal(t, strconv.Itoa(i), userId)
		assert.Equal(t, strconv.Itoa(2*i), itemId)
		assert.Equal(t, 3*i, int(value))
		assert.Equal(t, i, userIndex)
		assert.Equal(t, i, itemIndex)
	}
	// Clean csv file
	if err = os.Remove(csvFileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_SaveItemsToCSV(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Load data
	if err = db.LoadItemsFromCSV("../example/data/item_date.csv", ",", false, 1, "|", -1); err != nil {
		t.Fatal(err)
	}
	// Save data
	csvFileName := path.Join(model.TempDir, randstr.String(16))
	if err = db.SaveItemsToCSV(csvFileName, "::", false, false, "|", false); err != nil {
		t.Fatal(err)
	}
	// Check data
	entities := model.LoadEntityFromCSV(csvFileName, "::", "|", false,
		[]string{"ItemId", "Timestamp"}, 0)
	expected := []map[string]interface{}{
		{"ItemId": "1"},
		{"ItemId": "2"},
		{"ItemId": "3"},
		{"ItemId": "4"},
		{"ItemId": "5"},
	}
	assert.Equal(t, expected, entities)
}

func TestDB_SaveItemsToCSV_Date(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Load data
	if err = db.LoadItemsFromCSV("../example/data/item_date.csv", ",", false, 1, "|", 2); err != nil {
		t.Fatal(err)
	}
	// Save data
	csvFileName := path.Join(model.TempDir, randstr.String(16))
	if err = db.SaveItemsToCSV(csvFileName, "::", false, true, "|", true); err != nil {
		t.Fatal(err)
	}
	// Check data
	entities := model.LoadEntityFromCSV(csvFileName, "::", "|", false,
		[]string{"ItemId", "Timestamp", "Labels"}, 0)
	expected := []map[string]interface{}{
		{"ItemId": "1", "Timestamp": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).String(), "Labels": []string{"a", "b", "c"}},
		{"ItemId": "2", "Timestamp": time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC).String(), "Labels": []string{"e", "f", "g"}},
		{"ItemId": "3", "Timestamp": time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC).String(), "Labels": []string{"a", "b", "c"}},
		{"ItemId": "4", "Timestamp": time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC).String(), "Labels": []string{"e", "f", "g"}},
		{"ItemId": "5", "Timestamp": time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC).String(), "Labels": []string{"a", "b", "c"}},
	}
	assert.Equal(t, expected, entities)
}

func TestDB_Popularity(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Insert feedback
	itemIds := []string{"0", "2", "4", "6", "8"}
	popularity := []float64{3, 4, 5, 6, 7}
	for i, itemId := range itemIds {
		if err := db.InsertItem(Item{ItemId: itemId}, true); err != nil {
			t.Fatal(err)
		}
		for k := 0; k < int(popularity[i]); k++ {
			if err := db.InsertFeedback(Feedback{UserId: strconv.Itoa(k), ItemId: itemId}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Check popularity
	retItems, err := db.GetItems(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, item := range retItems {
		assert.Equal(t, popularity[i], item.Popularity)
	}
}

func TestDB_GetRecommends(t *testing.T) {
	// Create database
	db, err := Open(createTempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Put neighbors
	items := []RecommendedItem{
		{Item{ItemId: "0"}, 0},
		{Item{ItemId: "1"}, 1},
		{Item{ItemId: "2"}, 2},
		{Item{ItemId: "3"}, 3},
		{Item{ItemId: "4"}, 4},
	}
	if err = db.setList(prefixRecommends, "0", items); err != nil {
		t.Fatal(err)
	}
	// Get all
	retItems, err := db.getList(prefixRecommends, "0", 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, retItems)
	// Consume 2
	nItems, err := db.ConsumeRecommends("0", 2)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:2], nItems)
	count, err := db.CountIgnore()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, count)
	// Insert feedback
	if err = db.InsertFeedback(Feedback{UserId: "0", ItemId: "2"}); err != nil {
		t.Fatal(err)
	}
	// Consume rest
	nItems, err = db.GetRecommend("0", 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[3:], nItems)
	count, err = db.CountIgnore()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, count)
}
