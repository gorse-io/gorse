package database

import (
	"github.com/stretchr/testify/assert"
	"github.com/thanhpk/randstr"
	"github.com/zhenghaoz/gorse/core"
	"os"
	"path"
	"strconv"
	"testing"
	"time"
)

func TestDB_InsertGetFeedback(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Insert feedback
	users := []string{"0", "1", "2", "3", "4"}
	items := []string{"0", "2", "4", "6", "8"}
	feedback := []float64{0, 3, 6, 9, 12}
	for i := range users {
		if err := db.InsertFeedback(users[i], items[i], feedback[i]); err != nil {
			t.Fatal(err)
		}
	}
	// Count feedback
	count, err := db.CountFeedback()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Get feedback
	retUsers, retItems, retFeedback, err := db.GetFeedback()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, users, retUsers)
	assert.Equal(t, items, retItems)
	assert.Equal(t, feedback, retFeedback)
	// Get item
	for _, itemId := range items {
		item, err := db.GetItem(itemId)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, Item{ItemId: itemId}, item)
	}
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_GetItem(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
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
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_SetGetMeta(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Set meta
	if err = db.SetMeta("1", "2"); err != nil {
		t.Fatal(err)
	}
	// Get meta
	value, err := db.GetMeta("1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "2", value)
	// Get meta not existed
	value, err = db.GetMeta("NULL")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "", value)
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_GetRandom(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Insert feedback
	items := []string{"0", "2", "4", "6", "8"}
	stamps := []time.Time{
		time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
		time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
		time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
		time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
		time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
	}
	for i, itemId := range items {
		if err := db.InsertItem(Item{ItemId: itemId, Timestamp: stamps[i]}, true); err != nil {
			t.Fatal(err)
		}
	}
	// Test multiple times
	for i := 0; i < 3; i++ {
		// Sample all
		retItems, err := db.GetRandom(10)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, []RecommendedItem{
			{Item: Item{ItemId: "0", Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)}},
			{Item: Item{ItemId: "2", Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)}},
			{Item: Item{ItemId: "4", Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)}},
			{Item: Item{ItemId: "6", Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)}},
			{Item: Item{ItemId: "8", Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)}}}, retItems)
		// Sample part
		items1, err := db.GetRandom(3)
		if err != nil {
			t.Fatal(err)
		}
		items2, err := db.GetRandom(3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 3, len(items1))
		assert.Equal(t, 3, len(items2))
		assert.NotEqual(t, items1, items2)
	}
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_PutGetIdentList(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Put recommends
	items := []RecommendedItem{
		{Item{ItemId: "0"}, 0.0},
		{Item{ItemId: "1"}, 0.1},
		{Item{ItemId: "2"}, 0.2},
		{Item{ItemId: "3"}, 0.3},
		{Item{ItemId: "4"}, 0.4},
	}
	if err = db.PutIdentList(BucketRecommends, "0", items); err != nil {
		t.Fatal(err)
	}
	// Get recommends
	retItems, err := db.GetIdentList(BucketRecommends, "0", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, retItems)
	// Get n recommends
	nItems, err := db.GetIdentList(BucketRecommends, "0", 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:3], nItems)
	// Get n recommends with offset
	oItems, err := db.GetIdentList(BucketRecommends, "0", 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[1:4], oItems)
	// Test new user
	if _, err = db.GetIdentList(BucketRecommends, "1", 0, 0); err == nil {
		t.Fatal("error is expected for new user")
	}
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_PutGetList(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Put neighbors
	items := []RecommendedItem{
		{Item{ItemId: "0"}, 0},
		{Item{ItemId: "1"}, 1},
		{Item{ItemId: "2"}, 2},
		{Item{ItemId: "3"}, 3},
		{Item{ItemId: "4"}, 4},
	}
	if err = db.PutList(BucketPop, items); err != nil {
		t.Fatal(err)
	}
	// Get neighbors
	retItems, err := db.GetList(BucketPop, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, retItems)
	// Get n neighbors
	nItems, err := db.GetList(BucketPop, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:3], nItems)
	// Get n neighbors with offset
	offsetItems, err := db.GetList(BucketPop, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[1:4], offsetItems)
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_ToDataSet(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Load data
	if err = db.LoadFeedbackFromCSV("../example/file_data/feedback_explicit_header.csv", ",", true); err != nil {
		t.Fatal(err)
	}
	// To dataset
	dataSet, err := db.ToDataSet()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, dataSet.Count())
	assert.Equal(t, 5, dataSet.UserCount())
	assert.Equal(t, 5, dataSet.ItemCount())
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_LoadFeedbackFromCSV(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Load data
	if err = db.LoadFeedbackFromCSV("../example/file_data/feedback_explicit_header.csv", ",", true); err != nil {
		t.Fatal(err)
	}
	// Count feedback
	count, err := db.CountFeedback()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Check data
	users, itemIds, feedback, err := db.GetFeedback()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		assert.Equal(t, strconv.Itoa(i), users[i])
		assert.Equal(t, strconv.Itoa(2*i), itemIds[i])
		assert.Equal(t, 3*i, int(feedback[i]))
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
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_LoadItemsFromCSV(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Load data
	if err = db.LoadItemsFromCSV("../example/file_data/items.csv", "::", false, -1, "|", -1); err != nil {
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
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_LoadItemsFromCSV_Date(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Load data
	if err = db.LoadItemsFromCSV("../example/file_data/item_date.csv", ",", false, 1, "|", 2); err != nil {
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
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_SaveFeedbackToCSV(t *testing.T) {
	// Create database
	databaseFileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(databaseFileName)
	if err != nil {
		t.Fatal(err)
	}
	// Load data
	if err = db.LoadFeedbackFromCSV("../example/file_data/feedback_explicit_header.csv", ",", true); err != nil {
		t.Fatal(err)
	}
	// Save data
	csvFileName := path.Join(core.TempDir, randstr.String(16))
	if err = db.SaveFeedbackToCSV(csvFileName, ",", false); err != nil {
		t.Fatal(err)
	}
	// Check data
	data := core.LoadDataFromCSV(csvFileName, ",", false)
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
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(databaseFileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_SaveItemsToCSV(t *testing.T) {
	// Create database
	databaseFileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(databaseFileName)
	if err != nil {
		t.Fatal(err)
	}
	// Load data
	if err = db.LoadItemsFromCSV("../example/file_data/item_date.csv", ",", false, 1, "|", -1); err != nil {
		t.Fatal(err)
	}
	// Save data
	csvFileName := path.Join(core.TempDir, randstr.String(16))
	if err = db.SaveItemsToCSV(csvFileName, "::", false, false, "|", false); err != nil {
		t.Fatal(err)
	}
	// Check data
	entities := core.LoadEntityFromCSV(csvFileName, "::", "|", false,
		[]string{"ItemId", "Timestamp"}, 0)
	expected := []map[string]interface{}{
		{"ItemId": 1},
		{"ItemId": 2},
		{"ItemId": 3},
		{"ItemId": 4},
		{"ItemId": 5},
	}
	assert.Equal(t, expected, entities)
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(databaseFileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_SaveItemsToCSV_Date(t *testing.T) {
	// Create database
	databaseFileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(databaseFileName)
	if err != nil {
		t.Fatal(err)
	}
	// Load data
	if err = db.LoadItemsFromCSV("../example/file_data/item_date.csv", ",", false, 1, "|", 2); err != nil {
		t.Fatal(err)
	}
	// Save data
	csvFileName := path.Join(core.TempDir, randstr.String(16))
	if err = db.SaveItemsToCSV(csvFileName, "::", false, true, "|", true); err != nil {
		t.Fatal(err)
	}
	// Check data
	entities := core.LoadEntityFromCSV(csvFileName, "::", "|", false,
		[]string{"ItemId", "Timestamp", "Labels"}, 0)
	expected := []map[string]interface{}{
		{"ItemId": 1, "Timestamp": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).String(), "Labels": []string{"a", "b", "c"}},
		{"ItemId": 2, "Timestamp": time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC).String(), "Labels": []string{"e", "f", "g"}},
		{"ItemId": 3, "Timestamp": time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC).String(), "Labels": []string{"a", "b", "c"}},
		{"ItemId": 4, "Timestamp": time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC).String(), "Labels": []string{"e", "f", "g"}},
		{"ItemId": 5, "Timestamp": time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC).String(), "Labels": []string{"a", "b", "c"}},
	}
	assert.Equal(t, expected, entities)
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(databaseFileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_UpdatePopularity(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Insert feedback
	itemIds := []string{"0", "2", "4", "6", "8"}
	popularity := []float64{3, 4, 5, 6, 7}
	for _, itemId := range itemIds {
		if err := db.InsertItem(Item{ItemId: itemId}, true); err != nil {
			t.Fatal(err)
		}
	}
	// Update popularity
	if err := db.UpdatePopularity(itemIds, popularity); err != nil {
		t.Fatal(err)
	}
	// Check popularity
	retItems, err := db.GetItems(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, item := range retItems {
		assert.Equal(t, popularity[i], item.Popularity)
	}
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_ConsumeRecommends(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Put neighbors
	items := []RecommendedItem{
		{Item{ItemId: "0"}, 0},
		{Item{ItemId: "1"}, 1},
		{Item{ItemId: "2"}, 2},
		{Item{ItemId: "3"}, 3},
		{Item{ItemId: "4"}, 4},
	}
	if err = db.PutIdentList(BucketRecommends, "0", items); err != nil {
		t.Fatal(err)
	}
	// Get all
	retItems, err := db.GetIdentList(BucketRecommends, "0", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, retItems)
	// Consume n
	nItems, err := db.ConsumeRecommends("0", 3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:3], nItems)
	count, err := db.CountIgnore()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 4, count)
	// Consume n
	nItems, err = db.ConsumeRecommends("0", 3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[3:], nItems)
	count, err = db.CountIgnore()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 6, count)
	// Close database
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}

func TestDB_RemoveFromIdentList(t *testing.T) {
	// Create database
	fileName := path.Join(core.TempDir, randstr.String(16))
	db, err := Open(fileName)
	if err != nil {
		t.Fatal(err)
	}
	// Put neighbors
	items := []RecommendedItem{
		{Item{ItemId: "0"}, 0},
		{Item{ItemId: "1"}, 1},
		{Item{ItemId: "2"}, 2},
		{Item{ItemId: "3"}, 3},
		{Item{ItemId: "4"}, 4},
	}
	if err = db.PutIdentList(BucketRecommends, "0", items); err != nil {
		t.Fatal(err)
	}
	// Remove item
	if err = db.RemoveFromIdentList(BucketRecommends, "0", "2"); err != nil {
		t.Fatal(err)
	}
	dItems, err := db.GetIdentList(BucketRecommends, "0", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 4, len(dItems))
	// Clean database
	if err = os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
}
