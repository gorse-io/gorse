package engine

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/core"
	"os"
	"path"
	"testing"
)

func TestDB_InsertGetFeedback(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_feedback.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Insert feedback
	users := []int{0, 1, 2, 3, 4}
	items := []int{0, 2, 4, 6, 8}
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
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_feedback.db")); err != nil {
		t.Fatal(err)
	}
}

func TestDB_InsertGetItem(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_items.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Insert feedback
	items := []int{0, 2, 4, 6, 8}
	for _, itemId := range items {
		if err := db.InsertItem(itemId); err != nil {
			t.Fatal(err)
		}
	}
	// Count feedback
	count, err := db.CountItems()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Get feedback
	retItems, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, retItems)
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_items.db")); err != nil {
		t.Fatal(err)
	}
}

func TestDB_SetGetMeta(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_meta.db"))
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
	assert.Equal(t, "", value)
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_meta.db")); err != nil {
		t.Fatal(err)
	}
}

func TestDB_GetRandom(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_random.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Insert feedback
	items := []int{0, 2, 4, 6, 8}
	for _, itemId := range items {
		if err := db.InsertItem(itemId); err != nil {
			t.Fatal(err)
		}
	}
	// Sample all
	retItems, err := db.GetRandom(10)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []RecommendedItem{{ItemId: 0}, {ItemId: 2}, {ItemId: 4}, {ItemId: 6}, {ItemId: 8}}, retItems)
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
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_random.db")); err != nil {
		t.Fatal(err)
	}
}

func TestDB_SetGetRecommends(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_recommends.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Put recommends
	items := []RecommendedItem{{0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 4}}
	if err = db.SetRecommends(0, items); err != nil {
		t.Fatal(err)
	}
	// Get recommends
	retItems, err := db.GetRecommends(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, retItems)
	// Get n recommends
	nItems, err := db.GetRecommends(0, 3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:3], nItems)
	// Test new user
	if _, err = db.GetRecommends(1, 0); err == nil {
		t.Fatal("error is expected for new user")
	}
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_recommends.db")); err != nil {
		t.Fatal(err)
	}
}

func TestDB_SetGetNeighbors(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_neighbors.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Put neighbors
	items := []RecommendedItem{{0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 4}}
	if err = db.SetNeighbors(0, items); err != nil {
		t.Fatal(err)
	}
	// Get neighbors
	retItems, err := db.GetNeighbors(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, retItems)
	// Get n neighbors
	nItems, err := db.GetNeighbors(0, 3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:3], nItems)
	// Test new user
	if _, err = db.GetNeighbors(1, 0); err == nil {
		t.Fatal("error is expected for new user")
	}
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_neighbors.db")); err != nil {
		t.Fatal(err)
	}
}

func TestDB_SetGetPopular(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_popular.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Put neighbors
	items := []RecommendedItem{{0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 4}}
	if err = db.SetPopular(items); err != nil {
		t.Fatal(err)
	}
	// Get neighbors
	retItems, err := db.GetPopular(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, retItems)
	// Get n neighbors
	nItems, err := db.GetPopular(3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:3], nItems)
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_popular.db")); err != nil {
		t.Fatal(err)
	}
}

func TestDB_LoadFeedbackFromCSV(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_load_feedback.db"))
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
	users, items, feedback, err := db.GetFeedback()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		assert.Equal(t, i, users[i])
		assert.Equal(t, 2*i, items[i])
		assert.Equal(t, 3*i, int(feedback[i]))
	}
	// Count feedback
	count, err = db.CountItems()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Check data
	items, err = db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		assert.Equal(t, 2*i, items[i])
	}
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_load_feedback.db")); err != nil {
		t.Fatal(err)
	}
}

func TestDB_LoadItemsFromCSV(t *testing.T) {
	// Create database
	db, err := Open(path.Join(core.TempDir, "/test_load_items.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Load data
	if err = db.LoadItemsFromCSV("../example/file_data/items.csv", "::", false); err != nil {
		t.Fatal(err)
	}
	// Count feedback
	count, err := db.CountItems()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, count)
	// Check data
	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		assert.Equal(t, 1+i, items[i])
	}
	// Clean database
	if err = os.Remove(path.Join(core.TempDir, "/test_load_items.db")); err != nil {
		t.Fatal(err)
	}
}
