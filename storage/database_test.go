package storage

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

func getUsers(t *testing.T, db Database) []User {
	users := make([]User, 0)
	var err error
	var data []User
	cursor := ""
	for {
		cursor, data, err = db.GetUsers("", cursor, 2)
		assert.Nil(t, err)
		users = append(users, data...)
		if cursor == "" {
			if _, ok := db.(*Redis); !ok {
				assert.LessOrEqual(t, len(data), 2)
			}
			return users
		} else {
			if _, ok := db.(*Redis); !ok {
				assert.Equal(t, 2, len(data))
			}
		}
	}
}

func getItems(t *testing.T, db Database) []Item {
	items := make([]Item, 0)
	var err error
	var data []Item
	cursor := ""
	for {
		cursor, data, err = db.GetItems("", cursor, 2)
		assert.Nil(t, err)
		items = append(items, data...)
		if cursor == "" {
			if _, ok := db.(*Redis); !ok {
				assert.LessOrEqual(t, len(data), 2)
			}
			return items
		} else {
			if _, ok := db.(*Redis); !ok {
				assert.Equal(t, 2, len(data))
			}
		}
	}
}

func getFeedback(t *testing.T, db Database) []Feedback {
	feedback := make([]Feedback, 0)
	var err error
	var data []Feedback
	cursor := ""
	for {
		cursor, data, err = db.GetFeedback(cursor, 2)
		assert.Nil(t, err)
		feedback = append(feedback, data...)
		if cursor == "" {
			if _, ok := db.(*Redis); !ok {
				assert.LessOrEqual(t, len(data), 2)
			}
			return feedback
		} else {
			if _, ok := db.(*Redis); !ok {
				assert.Equal(t, 2, len(data))
			}
		}
	}
}

func getLabels(t *testing.T, db Database) []string {
	labels := make([]string, 0)
	var err error
	var data []string
	cursor := ""
	for {
		cursor, data, err = db.GetLabels("", cursor, 2)
		assert.Nil(t, err)
		labels = append(labels, data...)
		if cursor == "" {
			if _, ok := db.(*Redis); !ok {
				assert.LessOrEqual(t, len(data), 2)
			}
			return labels
		} else {
			if _, ok := db.(*Redis); !ok {
				assert.Equal(t, 2, len(data))
			}
		}
	}
}

func testUsers(t *testing.T, db Database) {
	// Insert users
	for i := 0; i < 10; i++ {
		if err := db.InsertUser(User{UserId: strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
	}
	// Get users
	users := getUsers(t, db)
	assert.Equal(t, 10, len(users))
	for i, user := range users {
		assert.Equal(t, strconv.Itoa(i), user.UserId)
	}
	// Get this user
	if user, err := db.GetUser("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, "0", user.UserId)
	}
	// Delete this user
	if err := db.DeleteUser("0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetUser("0"); err == nil {
		t.Fatal(err)
	}
}

func testFeedback(t *testing.T, db Database) {
	// Insert ret
	feedback := []Feedback{
		{"0", "0"},
		{"1", "2"},
		{"2", "4"},
		{"3", "6"},
		{"4", "8"},
	}
	err := db.InsertFeedback(feedback[0])
	assert.Nil(t, err)
	err = db.BatchInsertFeedback(feedback[1:])
	assert.Nil(t, err)
	// idempotent
	err = db.InsertFeedback(feedback[0])
	assert.Nil(t, err)
	err = db.BatchInsertFeedback(feedback[1:])
	assert.Nil(t, err)
	// Get feedback
	ret := getFeedback(t, db)
	assert.Equal(t, feedback, ret)
	// Get items
	items := getItems(t, db)
	assert.Equal(t, 5, len(items))
	for i, item := range items {
		assert.Equal(t, strconv.Itoa(i*2), item.ItemId)
	}
	// Get users
	users := getUsers(t, db)
	assert.Equal(t, 5, len(users))
	for i, user := range users {
		assert.Equal(t, strconv.Itoa(i), user.UserId)
	}
	// Get ret by user
	ret, err = db.GetUserFeedback("2")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "2", ret[0].UserId)
	assert.Equal(t, "4", ret[0].ItemId)
	// Get ret by item
	ret, err = db.GetItemFeedback("4")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "2", ret[0].UserId)
	assert.Equal(t, "4", ret[0].ItemId)
}

func testItems(t *testing.T, db Database) {
	// Items
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
	// Insert item
	err := db.InsertItem(items[0])
	assert.Nil(t, err)
	err = db.BatchInsertItem(items[1:])
	assert.Nil(t, err)
	// Get items
	totalItems := getItems(t, db)
	assert.Equal(t, items, totalItems)
	// Get item
	for _, item := range items {
		ret, err := db.GetItem(item.ItemId)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, item, ret)
	}
	// Get labels
	labels := getLabels(t, db)
	assert.Equal(t, []string{"a", "b"}, labels)
	// Get items by labels
	labelAItems, err := db.GetLabelItems("a")
	assert.Nil(t, err)
	assert.Equal(t, items[:3], labelAItems)
	labelBItems, err := db.GetLabelItems("b")
	assert.Nil(t, err)
	assert.Equal(t, items[2:], labelBItems)
	// Delete item
	err = db.DeleteItem("0")
	assert.Nil(t, err)
	_, err = db.GetItem("0")
	assert.NotNil(t, err)
}

func testIgnore(t *testing.T, db Database) {
	// Insert ignore
	ignores := []string{"0", "1", "2", "3", "4", "5"}
	if err := db.InsertUserIgnore("0", ignores); err != nil {
		t.Fatal(err)
	}
	// idempotent
	if err := db.InsertUserIgnore("0", ignores); err != nil {
		t.Fatal(err)
	}
	// Count ignore
	if count, err := db.CountUserIgnore("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, len(ignores), count)
	}
	// Get ignore
	if ret, err := db.GetUserIgnore("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, ignores, ret)
	}
}

func testMeta(t *testing.T, db Database) {
	// Set meta string
	if err := db.SetString("1", "2"); err != nil {
		t.Fatal(err)
	}
	// Get meta string
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
	// Set meta int
	if err = db.SetInt("1", 2); err != nil {
		t.Fatal(err)
	}
	// Get meta int
	if value, err := db.GetInt("1"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 2, value)
	}
}

func testList(t *testing.T, db Database) {
	type ListOperator struct {
		Set func(label string, items []RecommendedItem) error
		Get func(label string, n int, offset int) ([]RecommendedItem, error)
	}
	operators := []ListOperator{
		{db.SetRecommend, db.GetRecommend},
		{db.SetLatest, db.GetLatest},
		{db.SetPop, db.GetPop},
		{db.SetNeighbors, db.GetNeighbors},
	}
	for _, operator := range operators {
		// Put items
		items := []RecommendedItem{
			{"0", 0.0},
			{"1", 0.1},
			{"2", 0.2},
			{"3", 0.3},
			{"4", 0.4},
		}
		err := operator.Set("0", items)
		assert.Nil(t, err)
		// Get items
		totalItems, err := operator.Get("0", 0, 0)
		assert.Nil(t, err)
		assert.Equal(t, items, totalItems)
		// Get n items
		headItems, err := operator.Get("0", 3, 0)
		assert.Nil(t, err)
		assert.Equal(t, items[:3], headItems)
		// Get n items with offset
		offsetItems, err := operator.Get("0", 3, 1)
		assert.Nil(t, err)
		assert.Equal(t, items[1:4], offsetItems)
		// Get empty
		noItems, err := operator.Get("1", 0, 0)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(noItems))
		// test overwrite
		overwriteItems := []RecommendedItem{
			{"10", 0.0},
			{"11", 0.1},
			{"12", 0.2},
			{"13", 0.3},
			{"14", 0.4},
		}
		err = operator.Set("0", overwriteItems)
		assert.Nil(t, err)
		totalItems, err = operator.Get("0", 0, 0)
		assert.Nil(t, err)
		assert.Equal(t, overwriteItems, totalItems)
	}
}

func testDeleteUser(t *testing.T, db Database) {
	// Insert ret
	feedback := []Feedback{
		{"0", "0"},
		{"0", "2"},
		{"0", "4"},
		{"0", "6"},
		{"0", "8"},
	}
	if err := db.BatchInsertFeedback(feedback); err != nil {
		t.Fatal(err)
	}
	// Insert ignore
	if err := db.InsertUserIgnore("0", []string{"0", "1", "2"}); err != nil {
		t.Fatal(err)
	}
	// Delete user
	if err := db.DeleteUser("0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetUser("0"); err == nil {
		t.Fatal("failed to delete user")
	}
	if ret, err := db.GetUserFeedback("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 0, len(ret))
	}
	if ret, err := db.GetUserIgnore("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 0, len(ret))
	}
}

func testDeleteItem(t *testing.T, db Database) {
	// Insert ret
	feedbacks := []Feedback{
		{"0", "0"},
		{"1", "0"},
		{"2", "0"},
		{"3", "0"},
		{"4", "0"},
	}
	if err := db.BatchInsertFeedback(feedbacks); err != nil {
		t.Fatal(err)
	}
	// Delete item
	if err := db.DeleteItem("0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetItem("0"); err == nil {
		t.Fatal("failed to delete item")
	}
	if ret, err := db.GetItemFeedback("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 0, len(ret))
	}
}

func testPrefix(t *testing.T, db Database) {
	// insert items
	items := []Item{
		{ItemId: "a1", Labels: []string{"a1"}},
		{ItemId: "a2", Labels: []string{"a2"}},
		{ItemId: "a3", Labels: []string{"a3"}},
		{ItemId: "b1", Labels: []string{"b1"}},
		{ItemId: "b2", Labels: []string{"b2"}},
		{ItemId: "b3", Labels: []string{"b3"}},
	}
	err := db.BatchInsertItem(items)
	assert.Nil(t, err)
	// get items with prefix
	cursor, retItems, err := db.GetItems("a", "", 10)
	assert.Equal(t, 0, len(cursor))
	assert.Equal(t, items[0:3], retItems)
	assert.Nil(t, err)
	// get labels with prefix
	cursor, labels, err := db.GetLabels("a", "", 10)
	assert.Equal(t, 0, len(cursor))
	assert.Equal(t, []string{"a1", "a2", "a3"}, labels)
	assert.Nil(t, err)
	// insert users
	users := []User{
		{"a1"},
		{"a2"},
		{"a3"},
		{"b1"},
		{"b2"},
		{"b3"},
	}
	for _, user := range users {
		err = db.InsertUser(user)
		assert.Nil(t, err)
	}
	// get users with prefix
	cursor, retUsers, err := db.GetUsers("a", "", 10)
	assert.Equal(t, 0, len(cursor))
	assert.Equal(t, users[0:3], retUsers)
	assert.Nil(t, err)
}
