package data

import (
	"github.com/stretchr/testify/assert"
	"log"
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
		cursor, data, err = db.GetUsers(cursor, 2)
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
		cursor, data, err = db.GetItems(cursor, 2)
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
		cursor, data, err = db.GetFeedback("click", cursor, 2)
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

func testUsers(t *testing.T, db Database) {
	// Insert users
	for i := 0; i < 10; i++ {
		if err := db.InsertUser(User{UserId: strconv.Itoa(i), Labels: []string{strconv.Itoa(i + 100)}}); err != nil {
			t.Fatal(err)
		}
	}
	// Get users
	users := getUsers(t, db)
	assert.Equal(t, 10, len(users))
	for i, user := range users {
		assert.Equal(t, strconv.Itoa(i), user.UserId)
		assert.Equal(t, []string{strconv.Itoa(i + 100)}, user.Labels)
	}
	// Get this user
	if user, err := db.GetUser("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, "0", user.UserId)
	}
	// Delete this user
	err := db.DeleteUser("0")
	assert.Nil(t, err)
	_, err = db.GetUser("0")
	assert.NotNil(t, err)
}

func testFeedback(t *testing.T, db Database) {
	// users that already exists
	err := db.InsertUser(User{"0", []string{"a"}})
	assert.Nil(t, err)
	// items that already exists
	err = db.InsertItem(Item{ItemId: "0", Labels: []string{"b"}})
	assert.Nil(t, err)
	// Insert ret
	feedback := []Feedback{
		{FeedbackKey{"click", "0", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "1", "2"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "2", "4"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "3", "6"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "4", "8"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
	}
	err = db.InsertFeedback(feedback[0], true, true)
	assert.Nil(t, err)
	err = db.BatchInsertFeedback(feedback[1:], true, true)
	assert.Nil(t, err)
	// idempotent
	err = db.InsertFeedback(feedback[0], true, true)
	assert.Nil(t, err)
	err = db.BatchInsertFeedback(feedback[1:], true, true)
	assert.Nil(t, err)
	// other type
	err = db.InsertFeedback(Feedback{FeedbackKey: FeedbackKey{"like", "0", "2"}}, true, true)
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
	// check users that already exists
	user, err := db.GetUser("0")
	assert.Nil(t, err)
	assert.Equal(t, User{"0", []string{"a"}}, user)
	// check items that already exists
	item, err := db.GetItem("0")
	assert.Nil(t, err)
	assert.Equal(t, Item{ItemId: "0", Labels: []string{"b"}}, item)
	// Get ret by user
	ret, err = db.GetUserFeedback("click", "2")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "2", ret[0].UserId)
	assert.Equal(t, "4", ret[0].ItemId)
	// Get ret by item
	ret, err = db.GetItemFeedback("click", "4")
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
	// Delete item
	err = db.DeleteItem("0")
	assert.Nil(t, err)
	_, err = db.GetItem("0")
	assert.NotNil(t, err)
}

func testDeleteUser(t *testing.T, db Database) {
	// Insert ret
	feedback := []Feedback{
		{FeedbackKey{"click", "0", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "0", "2"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "0", "4"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "0", "6"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "0", "8"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
	}
	if err := db.BatchInsertFeedback(feedback, true, true); err != nil {
		t.Fatal(err)
	}
	// Delete user
	if err := db.DeleteUser("0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetUser("0"); err == nil {
		t.Fatal("failed to delete user")
	}
	if ret, err := db.GetUserFeedback("click", "0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 0, len(ret))
	}
	if _, ret, err := db.GetFeedback("click", "", 100); err != nil {
		t.Fatal(err)
	} else {
		assert.Empty(t, ret)
	}
}

func testDeleteItem(t *testing.T, db Database) {
	// Insert ret
	feedbacks := []Feedback{
		{FeedbackKey{"click", "0", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "1", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "2", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "3", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey{"click", "4", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)},
	}
	if err := db.BatchInsertFeedback(feedbacks, true, true); err != nil {
		t.Fatal(err)
	}
	// Delete item
	if err := db.DeleteItem("0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetItem("0"); err == nil {
		t.Fatal("failed to delete item")
	}
	if ret, err := db.GetItemFeedback("click", "0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 0, len(ret))
	}
	if _, ret, err := db.GetFeedback("click", "", 100); err != nil {
		log.Fatal(err)
	} else {
		assert.Empty(t, ret)
	}
}
