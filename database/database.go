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
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/araddon/dateparse"
	badger "github.com/dgraph-io/badger/v2"
	"github.com/zhenghaoz/gorse/model"
	"log"
	"math/bits"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// Meta data
	prefixMeta = "meta/" // prefix for meta data

	// Source data
	prefixLabelIndex = "index/label/" // prefix for label index
	prefixItemIndex  = "index/item/"  // prefix for item index
	prefixUserIndex  = "index/user/"  // prefix for user index
	prefixItem       = "item/"        // prefix for items
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
type Database struct {
	db *badger.DB
}

// Open a connection to the database.
func Open(path string) (*Database, error) {
	var err error
	database := new(Database)
	if database.db, err = badger.Open(badger.DefaultOptions(path)); err != nil {
		return nil, err
	}
	return database, nil
}

// Close the connection to the database.
func (db *Database) Close() error {
	return db.db.Close()
}

// FeedbackKey identifies feedback.
type FeedbackKey struct {
	UserId string
	ItemId string
}

// Feedback stores feedback.
type Feedback struct {
	UserId string
	ItemId string
	Rating float64
}

// InsertFeedback inserts a feedback into the bucket.
func InsertFeedback(txn *badger.Txn, feedback Feedback) error {
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

// BatchInsertFeedback inserts a feedback into the bucket within a batch write.
func BatchInsertFeedback(wb *badger.WriteBatch, feedback Feedback) error {
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

// InsertFeedback inserts a feedback into the database. If the item doesn't exist, this item will be added.
func (db *Database) InsertFeedback(feedback Feedback) error {
	return db.db.Update(func(txn *badger.Txn) error {
		// Insert feedback
		if err := InsertFeedback(txn, feedback); err != nil {
			return err
		}
		// Insert item
		if err := InsertItem(txn, Item{ItemId: feedback.ItemId}, false); err != nil {
			return err
		}
		// Refresh
		if err := RefreshItemPop(txn, feedback.ItemId); err != nil {
			return err
		}
		return nil
	})
}

// InsertFeedbacks inserts multiple feedback into the database. If the item doesn't exist, this item will be added.
func (db *Database) InsertFeedbacks(feedback []Feedback) error {
	// Write feedback
	txn := db.db.NewWriteBatch()
	for _, f := range feedback {
		if err := BatchInsertFeedback(txn, f); err != nil {
			return err
		}
	}
	if err := txn.Flush(); err != nil {
		return err
	}
	return db.db.Update(func(txn *badger.Txn) error {
		// Write items
		for _, f := range feedback {
			if err := InsertItem(txn, Item{ItemId: f.ItemId}, false); err != nil {
				return err
			}
			if err := RefreshItemPop(txn, f.ItemId); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetFeedback returns all feedback in the database.
func (db *Database) GetFeedback() (feedback []Feedback, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(prefixFeedback)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
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
		return nil
	})
	return
}

func GetFeedback(txn *badger.Txn, userId string, itemId string) (feedback Feedback, err error) {
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

func (db *Database) GetFeedbackByUser(userId string) (feedback []Feedback, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := newKey([]byte(prefixUserIndex), []byte(userId), []byte("/"))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := it.Item().Key()
			itemId := extractKey(k, []byte(prefixUserIndex), []byte(userId), []byte("/"))
			f, err := GetFeedback(txn, userId, itemId)
			if err != nil {
				return err
			}
			feedback = append(feedback, f)
		}
		return nil
	})
	return
}

func (db *Database) GetFeedbackByItem(itemId string) (feedback []Feedback, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := newKey([]byte(prefixItemIndex), []byte(itemId), []byte("/"))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := it.Item().Key()
			userId := extractKey(k, []byte(prefixItemIndex), []byte(itemId), []byte("/"))
			f, err := GetFeedback(txn, userId, itemId)
			if err != nil {
				return err
			}
			feedback = append(feedback, f)
		}
		return nil
	})
	return
}

// CountFeedback returns the number of feedback in the database.
func (db *Database) CountFeedback() (count int, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		count, err = CountPrefix(txn, []byte(prefixFeedback))
		return nil
	})
	return
}

// Item stores meta data about item.
type Item struct {
	ItemId     string
	Popularity float64
	Timestamp  time.Time
	Labels     []string
}

func RefreshItemPop(txn *badger.Txn, itemId string) error {
	// Check existence
	it, err := txn.Get(newKey([]byte(prefixItem), []byte(itemId)))
	if err != nil {
		return err
	}
	var item Item
	err = it.Value(func(val []byte) error {
		return json.Unmarshal(val, &item)
	})
	if err != nil {
		return err
	}
	// Collect feedback
	item.Popularity = 0
	iter := txn.NewIterator(badger.DefaultIteratorOptions)
	defer iter.Close()
	prefix := newKey([]byte(prefixItemIndex), []byte(itemId), []byte("/"))
	for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
		item.Popularity += 1
	}
	// Write item
	buf, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return txn.Set(newKey([]byte(prefixItem), []byte(itemId)), buf)
}

// InsertItem inserts a item into the bucket. The `override` flag indicates whether to overwrite existed item.
func InsertItem(txn *badger.Txn, item Item, override bool) error {
	// Check existence
	refresh := false
	if _, err := txn.Get(newKey([]byte(prefixItem), []byte(item.ItemId))); err == nil {
		if override {
			refresh = true
		} else {
			return nil
		}
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
		if err = txn.Set(newKey([]byte(prefixLabelIndex), []byte(tag), []byte("/"), []byte(itemId)), nil); err != nil {
			return err
		}
	}
	// Refresh pop
	if refresh {
		if err := RefreshItemPop(txn, itemId); err != nil {
			return err
		}
	}
	return nil
}

// InsertItem inserts a item into the database. The `override` flag indicates whether to overwrite existed item.
func (db *Database) InsertItem(item Item, override bool) error {
	return db.db.Update(func(txn *badger.Txn) error {
		return InsertItem(txn, item, override)
	})
}

// InsertItem inserts multiple items into the database. The `override` flag indicates whether to overwrite existed item.
func (db *Database) InsertItems(items []Item, override bool) error {
	return db.db.Update(func(txn *badger.Txn) error {
		for _, item := range items {
			if err := InsertItem(txn, item, override); err != nil {
				return err
			}
		}
		return nil
	})
}

func GetItem(txn *badger.Txn, itemId string) (item Item, err error) {
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

// GetItem gets a item from database by item ID.
func (db *Database) GetItem(itemId string) (item Item, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		item, err = GetItem(txn, itemId)
		return nil
	})
	return
}

// GetItems returns all items in the dataset.
func (db *Database) GetItems(n int, offset int) ([]Item, error) {
	if n == 0 {
		n = (1<<bits.UintSize)/2 - 1
	}
	pos := 0
	items := make([]Item, 0)
	err := db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(prefixItem)
		for it.Seek(prefix); it.ValidForPrefix(prefix) && len(items) < n; it.Next() {
			if pos < offset {
				// Skip offset
				pos += 1
			} else {
				// Read n
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
		}
		return nil
	})
	return items, err
}

// GetItemsByLabel list items with given label.
func (db *Database) GetItemsByLabel(label string) ([]Item, error) {
	items := make([]Item, 0)
	err := db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := newKey([]byte(prefixLabelIndex), []byte(label), []byte("/"))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := it.Item().Key()
			itemId := extractKey(k, []byte(prefixLabelIndex), []byte(label), []byte("/"))
			item, err := GetItem(txn, itemId)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	return items, err
}

func CountPrefix(txn *badger.Txn, prefix []byte) (int, error) {
	count := 0
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		count++
	}
	return count, nil
}

// CountItems returns the number of items in the database.
func (db *Database) CountItems() (count int, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		count, err = CountPrefix(txn, []byte(prefixItem))
		return nil
	})
	return
}

// CountIgnore returns the number of ignored items.
func (db *Database) CountIgnore() (count int, err error) {
	err = db.db.View(func(txn *badger.Txn) error {
		count, err = CountPrefix(txn, []byte(prefixIgnore))
		return nil
	})
	return
}

// GetString gets the value of a metadata.
func (db *Database) GetString(name string) (string, error) {
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
func (db *Database) SetString(name string, val string) error {
	return db.db.Update(func(txn *badger.Txn) error {
		return txn.Set(newKey([]byte(prefixMeta), []byte(name)), []byte(val))
	})
}

// RecommendedItem is the structure for a recommended item.
type RecommendedItem struct {
	Item
	Score float64 // score
}

// SetRecommends sets recommendations for a user.
func (db *Database) setList(prefix string, listId string, items []RecommendedItem) error {
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
func (db *Database) getList(prefix string, listId string, n int, offset int,
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

func (db *Database) SetNeighbors(itemId string, items []RecommendedItem) error {
	return db.setList(prefixNeighbors, itemId, items)
}

func (db *Database) SetPop(items []RecommendedItem) error {
	return db.setList(prefixPop, "", items)
}

func (db *Database) SetLatest(items []RecommendedItem) error {
	return db.setList(prefixLatest, "", items)
}

func (db *Database) SetRecommend(userId string, items []RecommendedItem) error {
	return db.setList(prefixRecommends, userId, items)
}

func (db *Database) GetNeighbors(itemId string, n int, offset int) ([]RecommendedItem, error) {
	return db.getList(prefixNeighbors, itemId, n, offset, nil)
}

func (db *Database) GetPop(n int, offset int) ([]RecommendedItem, error) {
	return db.getList(prefixPop, "", n, offset, nil)
}

func (db *Database) GetLatest(n int, offset int) ([]RecommendedItem, error) {
	return db.getList(prefixLatest, "", n, offset, nil)
}

func (db *Database) GetRecommend(userId string, n int, offset int) ([]RecommendedItem, error) {
	return db.getList(prefixRecommends, userId, n, offset, func(txn *badger.Txn, listId string, item RecommendedItem) bool {
		buf, err := json.Marshal(FeedbackKey{listId, item.ItemId})
		if err != nil {
			panic(err)
		}
		_, errIgnore := txn.Get(newKey([]byte(prefixIgnore), []byte(listId), []byte("/"), []byte(item.ItemId)))
		_, errFeedback := txn.Get(newKey([]byte(prefixFeedback), buf))
		if errIgnore == badger.ErrKeyNotFound && errFeedback == badger.ErrKeyNotFound {
			return true
		} else if errIgnore != nil && errIgnore != badger.ErrKeyNotFound {
			panic(errIgnore)
		} else if errFeedback != nil && errFeedback != badger.ErrKeyNotFound {
			panic(errFeedback)
		}
		return false
	})
}

func (db *Database) ConsumeRecommends(userId string, n int) ([]RecommendedItem, error) {
	items, err := db.GetRecommend(userId, n, 0)
	if err == nil {
		err = db.db.Update(func(txn *badger.Txn) error {
			for _, item := range items {
				if err := txn.Set(newKey([]byte(prefixIgnore), []byte(userId), []byte("/"), []byte(item.ItemId)), nil); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return items, err
}

// ToDataSet creates a dataset from the database.
func (db *Database) ToDataSet() (*model.DataSet, error) {
	// Count feedback
	count, err := db.CountFeedback()
	if err != nil {
		return nil, err
	}
	// Fetch ratings
	users, items, ratings := make([]string, count), make([]string, count), make([]float64, count)
	feedback, err := db.GetFeedback()
	if err != nil {
		return nil, err
	}
	for i := range feedback {
		users[i] = feedback[i].UserId
		items[i] = feedback[i].ItemId
		ratings[i] = feedback[i].Rating
	}
	dataset := model.NewDataSet(users, items, ratings)
	// Fetch features
	featuredItems, err := db.GetItems(0, 0)
	if err != nil {
		return nil, err
	}
	convertedItems := make([]map[string]interface{}, len(featuredItems))
	for i, item := range featuredItems {
		cvt := make(map[string]interface{})
		cvt["ItemId"] = item.ItemId
		cvt["Label"] = item.Labels
		convertedItems[i] = cvt
	}
	dataset.SetItemFeature(convertedItems, []string{"Label"}, "ItemId")
	return dataset, nil
}

// LoadFeedbackFromCSV import feedback from a CSV file into the database.
func (db *Database) LoadFeedbackFromCSV(fileName string, sep string, hasHeader bool) error {
	feedbacks := make([]Feedback, 0)
	// Open file
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// Read CSV file
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Ignore header
		if hasHeader {
			hasHeader = false
			continue
		}
		fields := strings.Split(line, sep)
		// Ignore empty line
		if len(fields) < 2 {
			continue
		}
		userId := fields[0]
		itemId := fields[1]
		feedback := 0.0
		if len(fields) > 2 {
			feedback, _ = strconv.ParseFloat(fields[2], 32)
		}
		feedbacks = append(feedbacks, Feedback{userId, itemId, feedback})
	}
	return db.InsertFeedbacks(feedbacks)
}

// LoadItemsFromCSV imports items from a CSV file into the database.
func (db *Database) LoadItemsFromCSV(fileName string, sep string, hasHeader bool, dateColumn int, labelSep string, labelColumn int) error {
	// Open file
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// Read CSV file
	scanner := bufio.NewScanner(file)
	items := make([]Item, 0)
	for scanner.Scan() {
		line := scanner.Text()
		// Ignore header
		if hasHeader {
			hasHeader = false
			continue
		}
		fields := strings.Split(line, sep)
		// Ignore empty line
		if len(fields) < 1 {
			continue
		}
		item := Item{ItemId: fields[0]}
		// Parse date
		if dateColumn > 0 && dateColumn < len(fields) {
			t, err := dateparse.ParseAny(fields[dateColumn])
			if err != nil && len(fields[dateColumn]) > 0 {
				return err
			}
			item.Timestamp = t
		}
		// Parse labels
		if labelColumn > 0 && labelColumn < len(fields) {
			item.Labels = strings.Split(fields[labelColumn], labelSep)
		}
		items = append(items, item)
	}
	return db.InsertItems(items, true)
}

// SaveFeedbackToCSV exports feedback from the database into a CSV file.
func (db *Database) SaveFeedbackToCSV(fileName string, sep string, header bool) error {
	// Open file
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	// Save feedback
	feedback, err := db.GetFeedback()
	if err != nil {
		return err
	}
	for i := range feedback {
		if _, err = file.WriteString(fmt.Sprintf("%v%v%v%v%v\n", feedback[i].UserId, sep, feedback[i].ItemId, sep, feedback[i].Rating)); err != nil {
			return err
		}
	}
	return nil
}

// SaveItemsToCSV exports items from the database into a CSV file.
func (db *Database) SaveItemsToCSV(fileName string, sep string, header bool, date bool, labelSep string, label bool) error {
	// Open file
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	// Save items
	items, err := db.GetItems(0, 0)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, err = file.WriteString(item.ItemId); err != nil {
			return err
		}
		if date {
			if _, err = file.WriteString(fmt.Sprintf("%s%v", sep, item.Timestamp)); err != nil {
				return err
			}
		}
		if label {
			for i, label := range item.Labels {
				if i == 0 {
					if _, err = file.WriteString(sep); err != nil {
						return err
					}
				} else {
					if _, err = file.WriteString(labelSep); err != nil {
						return err
					}
				}
				if _, err = file.WriteString(fmt.Sprintf("%v", label)); err != nil {
					return err
				}
			}
		}
		if _, err = file.WriteString("\n"); err != nil {
			return err
		}
	}
	return nil
}
