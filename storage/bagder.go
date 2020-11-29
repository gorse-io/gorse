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
package storage

import (
	"encoding/json"
	"github.com/dgraph-io/badger/v2"
	"math/bits"
	"strconv"
)

const (
	// Meta data
	prefixMeta = "meta/" // prefix for meta data

	// Source data
	prefixLabelIndex = "index/label/" // prefix for label index
	prefixItemIndex  = "index/item/"  // prefix for item index
	prefixUserIndex  = "index/user/"  // prefix for user index
	prefixLabel      = "label/"       // prefix for labels
	prefixItem       = "item/"        // prefix for items
	prefixUser       = "user/"        // prefix for users
	prefixFeedback   = "feedback/"    // prefix for feedback
	prefixIgnore     = "ignored/"     // prefix for ignored

	// Derived data
	prefixNeighbors  = "neighbors/"  // prefix for neighbors
	prefixRecommends = "recommends/" // prefix for recommendations
	prefixPop        = "populars/"   // prefix for popular items
	prefixLatest     = "latest/"     // prefix for latest items
)

func newKey(parts ...[]byte) []byte {
	k := make([]byte, 0)
	for _, p := range parts {
		k = append(k, p...)
	}
	return k
}

func extractKey(key []byte, prefix ...[]byte) string {
	prefixSize := 0
	for _, p := range prefix {
		prefixSize += len(p)
	}
	return string(key[prefixSize:])
}

// Database manages all data.
type Badger struct {
	db *badger.DB
}

func (db *Badger) Init() error {
	return nil
}

// Close the connection to the database.
func (db *Badger) Close() error {
	return db.db.Close()
}

// txnInsertFeedback inserts a feedback into the bucket.
func txnInsertFeedback(txn *badger.Txn, feedback Feedback) error {
	// Marshal key
	key, err := json.Marshal(FeedbackKey{feedback.UserId, feedback.ItemId})
	if err != nil {
		return err
	}
	// Marshal value
	value, err := json.Marshal(feedback)
	if err != nil {
		return err
	}
	// Insert feedback
	if err := txn.Set(newKey([]byte(prefixFeedback), key), value); err != nil {
		return err
	}
	// Insert user
	if err := txn.Set(newKey([]byte(prefixUser), []byte(feedback.UserId)), nil); err != nil {
		return err
	}
	// Insert user index
	if err := txn.Set(newKey([]byte(prefixUserIndex), []byte(feedback.UserId), []byte("/"), []byte(feedback.ItemId)), nil); err != nil {
		return err
	}
	// Insert item index
	if err := txn.Set(newKey([]byte(prefixItemIndex), []byte(feedback.ItemId), []byte("/"), []byte(feedback.UserId)), nil); err != nil {
		return err
	}
	return nil
}

// wbBatchInsertFeedback inserts a feedback into the bucket within a batch write.
func wbBatchInsertFeedback(wb *badger.WriteBatch, feedback Feedback) error {
	// Marshal key
	key, err := json.Marshal(FeedbackKey{feedback.UserId, feedback.ItemId})
	if err != nil {
		return err
	}
	// Marshal value
	value, err := json.Marshal(feedback)
	if err != nil {
		return err
	}
	// Insert feedback
	if err := wb.Set(newKey([]byte(prefixFeedback), key), value); err != nil {
		return err
	}
	// Insert user
	if err := wb.Set(newKey([]byte(prefixUser), []byte(feedback.UserId)), nil); err != nil {
		return err
	}
	// Insert user index
	if err := wb.Set(newKey([]byte(prefixUserIndex), []byte(feedback.UserId), []byte("/"), []byte(feedback.ItemId)), nil); err != nil {
		return err
	}
	// Insert item index
	if err := wb.Set(newKey([]byte(prefixItemIndex), []byte(feedback.ItemId), []byte("/"), []byte(feedback.UserId)), nil); err != nil {
		return err
	}
	return nil
}

// txnInsertFeedback inserts a feedback into the database. If the item doesn't exist, this item will be added.
func (db *Badger) InsertFeedback(feedback Feedback) error {
	return db.db.Update(func(txn *badger.Txn) error {
		// Insert feedback
		if err := txnInsertFeedback(txn, feedback); err != nil {
			return err
		}
		// Insert item
		if err := insertItem(txn, Item{ItemId: feedback.ItemId}, false); err != nil {
			return err
		}
		return nil
	})
}

// BatchInsertFeedback inserts multiple feedback into the database. If the item doesn't exist, this item will be added.
func (db *Badger) BatchInsertFeedback(feedback []Feedback) error {
	itemSet := make(map[string]interface{})
	// Write feedback
	txn := db.db.NewWriteBatch()
	for _, f := range feedback {
		if err := wbBatchInsertFeedback(txn, f); err != nil {
			return err
		}
		if _, exist := itemSet[f.ItemId]; !exist {
			itemSet[f.ItemId] = nil
		}
	}
	if err := txn.Flush(); err != nil {
		return err
	}
	return db.db.Update(func(txn *badger.Txn) error {
		for itemId := range itemSet {
			if err := insertItem(txn, Item{ItemId: itemId}, false); err != nil {
				return err
			}
		}
		return nil
	})
}

// txnGetFeedback returns all feedback in the database.
func (db *Badger) GetFeedback(cursor string, n int) (string, []Feedback, error) {
	feedback := make([]Feedback, 0)
	rawCursor := []byte(cursor)
	if len(cursor) == 0 {
		rawCursor = []byte(prefixFeedback)
	}
	err := db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(rawCursor); it.ValidForPrefix([]byte(prefixFeedback)) && len(feedback) < n; it.Next() {
			rawCursor = it.Item().Key()
			item := it.Item()
			err := item.Value(func(v []byte) error {
				var f Feedback
				if err := json.Unmarshal(v, &f); err != nil {
					return err
				}
				feedback = append(feedback, f)
				return nil
			})
			if err != nil {
				return err
			}
		}
		if !it.ValidForPrefix([]byte(prefixFeedback)) {
			rawCursor = nil
		} else {
			rawCursor = it.Item().Key()
		}
		return nil
	})
	return string(rawCursor), feedback, err
}

func txnGetFeedback(txn *badger.Txn, userId string, itemId string) (feedback Feedback, err error) {
	feedbackKey := FeedbackKey{UserId: userId, ItemId: itemId}
	key, err := json.Marshal(feedbackKey)
	if err != nil {
		return Feedback{}, err
	}
	item, err := txn.Get(newKey([]byte(prefixFeedback), key))
	if err != nil {
		return Feedback{}, err
	}
	err = item.Value(func(val []byte) error {
		return json.Unmarshal(val, &feedback)
	})
	return feedback, err
}

func (db *Badger) GetUserFeedback(userId string) (feedback []Feedback, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := newKey([]byte(prefixUserIndex), []byte(userId), []byte("/"))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := it.Item().Key()
			itemId := extractKey(k, []byte(prefixUserIndex), []byte(userId), []byte("/"))
			f, err := txnGetFeedback(txn, userId, itemId)
			if err != nil {
				return err
			}
			feedback = append(feedback, f)
		}
		return nil
	})
	return
}

func (db *Badger) GetItemFeedback(itemId string) (feedback []Feedback, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := newKey([]byte(prefixItemIndex), []byte(itemId), []byte("/"))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := it.Item().Key()
			userId := extractKey(k, []byte(prefixItemIndex), []byte(itemId), []byte("/"))
			f, err := txnGetFeedback(txn, userId, itemId)
			if err != nil {
				return err
			}
			feedback = append(feedback, f)
		}
		return nil
	})
	return
}

// InsertUser insert a user into database.
func (db *Badger) InsertUser(user User) error {
	return db.db.Update(func(txn *badger.Txn) error {
		return txn.Set(newKey([]byte(prefixUser), []byte(user.UserId)), nil)
	})
}

func txnDeleteByPrefix(txn *badger.Txn, prefix []byte) error {
	deleteKeys := func(keysForDelete [][]byte) error {
		for _, key := range keysForDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	}
	opts := badger.DefaultIteratorOptions
	opts.AllVersions = false
	opts.PrefetchValues = false
	it := txn.NewIterator(opts)
	defer it.Close()
	keysForDelete := make([][]byte, 0, 0)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		key := it.Item().KeyCopy(nil)
		keysForDelete = append(keysForDelete, key)
	}
	if err := deleteKeys(keysForDelete); err != nil {
		return err
	}
	return nil
}

func (db *Badger) DeleteUser(userId string) error {
	return db.db.Update(func(txn *badger.Txn) error {
		if err := txn.Delete(newKey([]byte(prefixUser), []byte(userId))); err != nil {
			return err
		}
		if err := txnDeleteByPrefix(txn, newKey([]byte(prefixUserIndex), []byte(userId))); err != nil {
			return err
		}
		return txnDeleteByPrefix(txn, newKey([]byte(prefixIgnore), []byte(userId)))
	})
}

func (db *Badger) GetUser(userId string) (user User, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		if _, err := txn.Get(newKey([]byte(prefixUser), []byte(userId))); err != nil {
			return err
		} else {
			user.UserId = userId
		}
		return nil
	})
	return
}

func (db *Badger) GetUsers(cursor string, n int) (string, []User, error) {
	users := make([]User, 0)
	rawCursor := []byte(cursor)
	if len(cursor) == 0 {
		rawCursor = []byte(prefixUser)
	}
	err := db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(rawCursor); it.ValidForPrefix([]byte(prefixUser)) && len(users) < n; it.Next() {
			rawCursor = it.Item().Key()
			userId := extractKey(it.Item().Key(), []byte(prefixUser))
			users = append(users, User{UserId: userId})
		}
		if !it.ValidForPrefix([]byte(prefixUser)) {
			rawCursor = nil
		} else {
			rawCursor = it.Item().Key()
		}
		return nil
	})
	return string(rawCursor), users, err
}

func txnInsertLabel(txn *badger.Txn, label string) error {
	return txn.Set(newKey([]byte(prefixLabel), []byte(label)), nil)
}

// InsertItem inserts a item into the bucket. The `override` flag indicates whether to overwrite existed item.
func insertItem(txn *badger.Txn, item Item, override bool) error {
	// Check existence
	if _, err := txn.Get(newKey([]byte(prefixItem), []byte(item.ItemId))); err == nil && !override {
		return nil
	}
	// Write item
	itemId := item.ItemId
	buf, err := json.Marshal(item)
	if err != nil {
		return err
	}
	if err = txn.Set(newKey([]byte(prefixItem), []byte(itemId)), buf); err != nil {
		return err
	}
	// Write index
	for _, tag := range item.Labels {
		if err = txnInsertLabel(txn, tag); err != nil {
			return err
		}
		if err = txn.Set(newKey([]byte(prefixLabelIndex), []byte(tag), []byte("/"), []byte(itemId)), nil); err != nil {
			return err
		}
	}
	return nil
}

// insertItem inserts a item into the database. The `override` flag indicates whether to overwrite existed item.
func (db *Badger) InsertItem(item Item) error {
	return db.db.Update(func(txn *badger.Txn) error {
		return insertItem(txn, item, true)
	})
}

// insertItem inserts multiple items into the database. The `override` flag indicates whether to overwrite existed item.
func (db *Badger) BatchInsertItem(items []Item) error {
	return db.db.Update(func(txn *badger.Txn) error {
		for _, item := range items {
			if err := insertItem(txn, item, true); err != nil {
				return err
			}
		}
		return nil
	})
}

func txnGetItem(txn *badger.Txn, itemId string) (item Item, err error) {
	var it *badger.Item
	it, err = txn.Get(newKey([]byte(prefixItem), []byte(itemId)))
	if err != nil {
		return
	}
	err = it.Value(func(val []byte) error {
		return json.Unmarshal(val, &item)
	})
	return
}

func (db *Badger) DeleteItem(itemId string) error {
	return db.db.Update(func(txn *badger.Txn) error {
		if err := txn.Delete(newKey([]byte(prefixItem), []byte(itemId))); err != nil {
			return err
		}
		return txnDeleteByPrefix(txn, newKey([]byte(prefixItemIndex), []byte(itemId)))
	})
}

// txnGetItem gets a item from database by item ID.
func (db *Badger) GetItem(itemId string) (item Item, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		item, err = txnGetItem(txn, itemId)
		return err
	})
	return
}

// GetItems returns all items in the dataset.
func (db *Badger) GetItems(cursor string, n int) (string, []Item, error) {
	items := make([]Item, 0)
	rawCursor := []byte(cursor)
	if len(cursor) == 0 {
		rawCursor = []byte(prefixItem)
	}
	err := db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(rawCursor); it.ValidForPrefix([]byte(prefixItem)) && len(items) < n; it.Next() {
			// Read n
			rawCursor = it.Item().Key()
			err := it.Item().Value(func(v []byte) error {
				var item Item
				if err := json.Unmarshal(v, &item); err != nil {
					return err
				}
				items = append(items, item)
				return nil
			})
			if err != nil {
				return err
			}
		}
		if !it.ValidForPrefix([]byte(prefixItem)) {
			rawCursor = nil
		} else {
			rawCursor = it.Item().Key()
		}
		return nil
	})
	return string(rawCursor), items, err
}

func (db *Badger) GetLabels(cursor string, n int) (string, []string, error) {
	labels := make([]string, 0)
	rawCursor := []byte(cursor)
	if len(cursor) == 0 {
		rawCursor = []byte(prefixLabel)
	}
	err := db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(rawCursor); it.ValidForPrefix([]byte(prefixLabel)) && len(labels) < n; it.Next() {
			rawCursor = it.Item().Key()
			label := extractKey(it.Item().Key(), []byte(prefixLabel))
			labels = append(labels, label)
		}
		if !it.ValidForPrefix([]byte(prefixItem)) {
			rawCursor = nil
		} else {
			rawCursor = it.Item().Key()
		}
		return nil
	})
	return string(rawCursor), labels, err
}

// GetLabelItems list items with given label.
func (db *Badger) GetLabelItems(label string) ([]Item, error) {
	items := make([]Item, 0)
	err := db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := newKey([]byte(prefixLabelIndex), []byte(label), []byte("/"))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := it.Item().Key()
			itemId := extractKey(k, []byte(prefixLabelIndex), []byte(label), []byte("/"))
			item, err := txnGetItem(txn, itemId)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	return items, err
}

func txnCountPrefix(txn *badger.Txn, prefix []byte) (int, error) {
	count := 0
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		count++
	}
	return count, nil
}

func (db *Badger) InsertUserIgnore(userId string, items []string) error {
	return db.db.Update(func(txn *badger.Txn) error {
		for _, itemId := range items {
			if err := insertIgnore(txn, userId, itemId); err != nil {
				return err
			}
		}
		return nil
	})
}

func (db *Badger) GetUserIgnore(userId string) (ignored []string, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(prefixIgnore)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			itemId := extractKey(it.Item().Key(), []byte(prefixIgnore), []byte(userId), []byte("/"))
			ignored = append(ignored, itemId)
		}
		return nil
	})
	return
}

func (db *Badger) CountUserIgnore(userId string) (count int, err error) {
	prefix := newKey([]byte(prefixIgnore), []byte(userId))
	err = db.db.View(func(txn *badger.Txn) error {
		count, err = txnCountPrefix(txn, prefix)
		return nil
	})
	return
}

// GetString gets the value of a metadata.
func (db *Badger) GetString(name string) (string, error) {
	var value string
	err := db.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(newKey([]byte(prefixMeta), []byte(name)))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			value = string(val)
			return nil
		})
	})
	return value, err
}

// SetString sets the value of a metadata.
func (db *Badger) SetString(name string, val string) error {
	return db.db.Update(func(txn *badger.Txn) error {
		return txn.Set(newKey([]byte(prefixMeta), []byte(name)), []byte(val))
	})
}

func (db *Badger) GetInt(name string) (int, error) {
	val, err := db.GetString(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(val)
}

func (db *Badger) SetInt(name string, val int) error {
	return db.SetString(name, strconv.Itoa(val))
}

// SetRecommends sets recommendations for a user.
func (db *Badger) setList(prefix string, listId string, items []RecommendedItem) error {
	return db.db.Update(func(txn *badger.Txn) error {
		for i, item := range items {
			buf, err := json.Marshal(item)
			if err != nil {
				return err
			}
			if err = txn.Set(newKey([]byte(prefix), []byte(listId), []byte("/"), []byte(strconv.Itoa(i))), buf); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetRecommends gets n recommendations for a user.
func (db *Badger) getList(prefix string, listId string, n int, offset int,
	filter func(txn *badger.Txn, listId string, item RecommendedItem) bool) ([]RecommendedItem, error) {
	var items []RecommendedItem
	if n == 0 {
		n = (1<<bits.UintSize)/2 - 1
	}
	err := db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := newKey([]byte(prefix), []byte(listId), []byte("/"))
		pos := 0
		for it.Seek(prefix); it.ValidForPrefix(prefix) && len(items) < n; it.Next() {
			var item RecommendedItem
			err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &item)
			})
			if err != nil {
				return err
			}
			if filter == nil || filter(txn, listId, item) {
				if pos < offset {
					pos++
				} else {
					items = append(items, item)
				}
			}
		}
		return nil
	})
	return items, err
}

func (db *Badger) SetNeighbors(itemId string, items []RecommendedItem) error {
	return db.setList(prefixNeighbors, itemId, items)
}

func (db *Badger) SetPop(label string, items []RecommendedItem) error {
	return db.setList(prefixPop, label, items)
}

func (db *Badger) SetLatest(label string, items []RecommendedItem) error {
	return db.setList(prefixLatest, label, items)
}

func (db *Badger) SetRecommend(userId string, items []RecommendedItem) error {
	return db.setList(prefixRecommends, userId, items)
}

func (db *Badger) GetNeighbors(itemId string, n int, offset int) ([]RecommendedItem, error) {
	return db.getList(prefixNeighbors, itemId, n, offset, nil)
}

func (db *Badger) GetPop(label string, n int, offset int) ([]RecommendedItem, error) {
	return db.getList(prefixPop, label, n, offset, nil)
}

func (db *Badger) GetLatest(label string, n int, offset int) ([]RecommendedItem, error) {
	return db.getList(prefixLatest, label, n, offset, nil)
}

func (db *Badger) GetRecommend(userId string, n int, offset int) ([]RecommendedItem, error) {
	return db.getList(prefixRecommends, userId, n, offset, nil)
}

func insertIgnore(txn *badger.Txn, userId string, itemId string) error {
	return txn.Set(newKey([]byte(prefixIgnore), []byte(userId), []byte("/"), []byte(itemId)), nil)
}
