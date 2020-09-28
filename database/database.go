package database

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/araddon/dateparse"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	bolt "go.etcd.io/bbolt"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	bucketMeta       = "meta"       // Bucket name for meta data
	bucketIndex      = "index"      // Bucket name for index
	bucketItems      = "items"      // Bucket name for items
	bucketFeedback   = "feedback"   // Bucket name for feedback
	BucketIgnore     = "ignored"    // Bucket name for ignored
	BucketNeighbors  = "neighbors"  // Bucket name for neighbors
	BucketRecommends = "recommends" // Bucket name for recommendations
	BucketPop        = "pop"        // Bucket name for popular items
	BucketLatest     = "latest"     // Bucket name for latest items
)

func encodeInt(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func decodeInt(buf []byte) int {
	return int(binary.BigEndian.Uint64(buf))
}

func encodeFloat(v float64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, math.Float64bits(v))
	return b
}

func decodeFloat(buf []byte) float64 {
	return math.Float64frombits(binary.BigEndian.Uint64(buf))
}

// DB manages all data for the engine.
type DB struct {
	db *bolt.DB // based on BoltDB
}

// Open a connection to the database.
func Open(path string) (*DB, error) {
	db := new(DB)
	var err error
	db.db, err = bolt.Open(path, 0666, nil)
	if err != nil {
		return nil, err
	}
	// Create buckets
	err = db.db.Update(func(tx *bolt.Tx) error {
		bucketNames := []string{bucketMeta, bucketItems, bucketIndex, bucketFeedback, BucketRecommends, BucketIgnore, BucketNeighbors}
		for _, name := range bucketNames {
			if _, err = tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Close the connection to the database.
func (db *DB) Close() error {
	return db.db.Close()
}

// FeedbackKey identifies feedback.
type FeedbackKey struct {
	UserId string
	ItemId string
}

// Feedback stores feedback.
type Feedback struct {
	FeedbackKey
	Rating float64
}

// InsertFeedback inserts a feedback into the database. If the item doesn't exist, this item will be added.
func (db *DB) InsertFeedback(userId, itemId string, feedback float64) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketFeedback))
		// Marshal data into bytes.
		key, err := json.Marshal(FeedbackKey{userId, itemId})
		if err != nil {
			return err
		}
		// Persist bytes to users bucket.
		return bucket.Put(key, encodeFloat(feedback))
	})
	if err != nil {
		return err
	}
	if err = db.InsertItem(Item{ItemId: itemId}, false); err != nil {
		return err
	}
	if err = db.RemoveFromIdentList(BucketRecommends, userId, itemId); err != nil && err != bolt.ErrBucketNotFound {
		return err
	}
	return nil
}

// GetFeedback returns all feedback in the database.
func (db *DB) GetFeedback() (users, items []string, feedback []float64, err error) {
	err = db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketFeedback))
		return bucket.ForEach(func(k, v []byte) error {
			key := FeedbackKey{}
			if err := json.Unmarshal(k, &key); err != nil {
				return err
			}
			users = append(users, key.UserId)
			items = append(items, key.ItemId)
			feedback = append(feedback, decodeFloat(v))
			return nil
		})
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return
}

// CountFeedback returns the number of feedback in the database.
func (db *DB) CountFeedback() (int, error) {
	count := 0
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketFeedback))
		count = bucket.Stats().KeyN
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Item stores meta data about item.
type Item struct {
	ItemId     string
	Popularity float64
	Timestamp  time.Time
	Labels     []string
}

// InsertItem inserts a item into the database.
func (db *DB) InsertItem(item Item, override bool) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		var err error
		bucket := tx.Bucket([]byte(bucketItems))
		// Check existence
		if !override && bucket.Get([]byte(item.ItemId)) != nil {
			return nil
		}
		// Write item
		itemId := item.ItemId
		buf, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if err = bucket.Put([]byte(itemId), buf); err != nil {
			return err
		}
		// Write index
		indexBucket := tx.Bucket([]byte(bucketIndex))
		for _, tag := range item.Labels {
			labelBucket, err := indexBucket.CreateBucketIfNotExists([]byte(tag))
			if err != nil {
				return err
			}
			if err = labelBucket.Put([]byte(itemId), nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetItem gets a item from database by item ID.
func (db *DB) GetItem(itemId string) (Item, error) {
	item := Item{}
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketItems))
		buf := bucket.Get([]byte(itemId))
		if buf == nil {
			return fmt.Errorf("item %v not found", itemId)
		}
		if err := json.Unmarshal(buf, &item); err != nil {
			return err
		}
		return nil
	})
	return item, err
}

// GetItems returns all items in the dataset.
func (db *DB) GetItems(n int, offset int) ([]Item, error) {
	items := make([]Item, 0)
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketItems))
		if n == 0 {
			n = bucket.Stats().KeyN
		}
		// Skip offset
		cursor := bucket.Cursor()
		cursor.First()
		first := true
		for i := 0; i < offset; i++ {
			var key []byte
			if first {
				key, _ = cursor.First()
				first = false
			} else {
				key, _ = cursor.Next()
			}
			if key == nil {
				return nil
			}
		}
		// Read n
		for i := 0; i < n; i++ {
			var key, value []byte
			if first {
				key, value = cursor.First()
				first = false
			} else {
				key, value = cursor.Next()
			}
			if key == nil {
				return nil
			}
			item := Item{ItemId: string(key)}
			if value != nil {
				err := json.Unmarshal(value, &item)
				if err != nil {
					return err
				}
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetItemsByLabel list items with given label.
func (db *DB) GetItemsByLabel(label string) ([]Item, error) {
	items := make([]Item, 0)
	err := db.db.View(func(tx *bolt.Tx) error {
		itemBucket := tx.Bucket([]byte(bucketItems))
		indexBucket := tx.Bucket([]byte(bucketIndex))
		labelBucket := indexBucket.Bucket([]byte(label))
		if labelBucket == nil {
			return bolt.ErrBucketNotFound
		}
		return labelBucket.ForEach(func(k, v []byte) error {
			buf := itemBucket.Get(k)
			var item Item
			if err := json.Unmarshal(buf, &item); err != nil {
				return err
			}
			items = append(items, item)
			return nil
		})
	})
	return items, err
}

// GetItem gets items from database by item IDs.
func (db *DB) GetItemsByID(id []string) ([]Item, error) {
	items := make([]Item, len(id))
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketItems))
		for i, v := range id {
			item := Item{}
			buf := bucket.Get([]byte(v))
			if buf == nil {
				return fmt.Errorf("item %v not found", v)
			}
			if err := json.Unmarshal(buf, &item); err != nil {
				return err
			}
			items[i] = item
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// CountItems returns the number of items in the database.
func (db *DB) CountItems() (int, error) {
	count := 0
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketItems))
		count = bucket.Stats().KeyN
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountIgnore returns the number of ignored items.
func (db *DB) CountIgnore() (int, error) {
	count := 0
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketIgnore))
		count = bucket.Stats().KeyN
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetMeta gets the value of a metadata.
func (db *DB) GetMeta(name string) (string, error) {
	var value string
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketMeta))
		value = string(bucket.Get([]byte(name)))
		return nil
	})
	return value, err
}

// SetMeta sets the value of a metadata.
func (db *DB) SetMeta(name string, val string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketMeta))
		return bucket.Put([]byte(name), []byte(val))
	})
}

// RecommendedItem is the structure for a recommended item.
type RecommendedItem struct {
	Item
	Score float64 // score
}

// GetRandom returns random items.
func (db *DB) GetRandom(n int) ([]RecommendedItem, error) {
	// count items
	count, err := db.CountItems()
	if err != nil {
		return nil, err
	}
	n = base.Min([]int{count, n})
	// generate random indices
	selected := make(map[int]bool)
	for len(selected) < n {
		randomIndex := rand.Intn(count)
		selected[randomIndex] = true
	}
	items := make([]RecommendedItem, 0)
	err = db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketItems))
		ptr := 0
		return bucket.ForEach(func(k, v []byte) error {
			// Sample
			if _, exist := selected[ptr]; exist {
				item := Item{}
				if err := json.Unmarshal(v, &item); err != nil {
					return err
				}
				items = append(items, RecommendedItem{Item: item})
			}
			ptr++
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// SetRecommends sets recommendations for a user.
func (db *DB) PutIdentList(bucketName string, id string, items []RecommendedItem) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bucketName))
		// Delete sub-bucket if exists
		var sBucket *bolt.Bucket
		var err error
		if sBucket = bucket.Bucket([]byte(id)); sBucket != nil {
			if err = bucket.DeleteBucket([]byte(id)); err != nil {
				return err
			}
		}
		// Create sub-bukcet
		if sBucket, err = bucket.CreateBucket([]byte(id)); err != nil {
			return err
		}
		// Persist list to bucket
		for i, item := range items {
			// Marshal data into bytes
			buf, err := json.Marshal(item)
			if err != nil {
				return err
			}
			if err = sBucket.Put(encodeInt(i), buf); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetRecommends gets n recommendations for a user.
func (db *DB) GetIdentList(bucketName string, id string, n int, offset int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.View(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bucketName))
		// Get sub-bucket
		sBucket := bucket.Bucket([]byte(id))
		if sBucket == nil {
			return bolt.ErrBucketNotFound
		}
		// Unmarshal data into bytes
		if n == 0 {
			n = sBucket.Stats().KeyN
		}
		for i := offset; i < offset+n; i++ {
			buf := sBucket.Get(encodeInt(i))
			if buf == nil {
				n++
				continue
			}
			var item RecommendedItem
			if err := json.Unmarshal(buf, &item); err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// AppendIdentList items into a list.
func (db *DB) AppendIdentList(bucketName string, id string, items []RecommendedItem) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bucketName))
		// Create sub-bukcet
		var sBucket *bolt.Bucket
		var err error
		if sBucket, err = bucket.CreateBucketIfNotExists([]byte(id)); err != nil {
			return err
		}
		// Locate start
		key, _ := sBucket.Cursor().Last()
		start := 0
		if key != nil {
			start = decodeInt(key) + 1
		}
		// Persist list to bucket
		for i, item := range items {
			// Marshal data into bytes
			buf, err := json.Marshal(item)
			if err != nil {
				return err
			}
			if err = sBucket.Put(encodeInt(start+i), buf); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveFromIdentList remove item from list.
func (db *DB) RemoveFromIdentList(bucketName string, id string, itemId string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bucketName))
		// Get sub bucket
		sBucket := bucket.Bucket([]byte(id))
		if sBucket == nil {
			return bolt.ErrBucketNotFound
		}
		// Search item
		cursor := sBucket.Cursor()
		first := true
		for {
			var key, value []byte
			if first {
				key, value = cursor.First()
				first = false
			} else {
				key, value = cursor.Next()
			}
			if key == nil {
				break
			}
			var item RecommendedItem
			if err := json.Unmarshal(value, &item); err != nil {
				return err
			}
			if item.ItemId == itemId {
				return cursor.Delete()
			}
		}
		return nil
	})
}

// ConsumeRecommends get recommendations and remove items from list.
func (db *DB) ConsumeRecommends(id string, n int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(BucketRecommends))
		// Get sub-bucket
		sBucket := bucket.Bucket([]byte(id))
		if sBucket == nil {
			return bolt.ErrBucketNotFound
		}
		// Load items
		cursor := sBucket.Cursor()
		first := true
		for i := 0; i < n; i++ {
			var key, value []byte
			if first {
				key, value = cursor.First()
				first = false
			} else {
				key, value = cursor.Next()
			}
			if err := cursor.Delete(); err != nil {
				return err
			}
			if key == nil {
				break
			}
			var item RecommendedItem
			if err := json.Unmarshal(value, &item); err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	if err = db.AppendIdentList(BucketIgnore, id, items); err != nil {
		return nil, err
	}
	return items, err
}

// UpdatePopularity update popularity of items.
func (db *DB) UpdatePopularity(itemId []string, popularity []float64) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bucketItems))
		for i, id := range itemId {
			// Unmarshal data from bytes
			item := Item{ItemId: id}
			buf := bucket.Get([]byte(id))
			if buf != nil {
				if err := json.Unmarshal(buf, &item); err != nil {
					return err
				}
			}
			item.Popularity = popularity[i]
			buf, err := json.Marshal(item)
			if err != nil {
				return err
			}
			if err = bucket.Put([]byte(id), buf); err != nil {
				return err
			}
		}
		return nil
	})
}

// PutList saves a list into the database.
func (db *DB) PutList(name string, items []RecommendedItem) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		var bucket *bolt.Bucket
		var err error
		// Delete bucket if exists
		if bucket = tx.Bucket([]byte(name)); bucket != nil {
			if err = tx.DeleteBucket([]byte(name)); err != nil {
				return err
			}
		}
		// Create bukcet
		if bucket, err = tx.CreateBucket([]byte(name)); err != nil {
			return err
		}
		// Persist list to bucket
		for i, item := range items {
			// Marshal data into bytes
			buf, err := json.Marshal(item)
			if err != nil {
				return err
			}
			if err = bucket.Put(encodeInt(i), buf); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetList gets a list from the database.
func (db *DB) GetList(name string, n int, offset int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.View(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(name))
		// Unmarshal data into bytes
		if n == 0 {
			n = bucket.Stats().KeyN
		}
		for i := offset; i < offset+n; i++ {
			buf := bucket.Get(encodeInt(i))
			if buf == nil {
				break
			}
			var item RecommendedItem
			if err := json.Unmarshal(buf, &item); err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ToDataSet creates a dataset from the database.
func (db *DB) ToDataSet() (*core.DataSet, error) {
	users, items, feedback, err := db.GetFeedback()
	if err != nil {
		return nil, err
	}
	return core.NewDataSet(users, items, feedback), nil
}

// LoadFeedbackFromCSV import feedback from a CSV file into the database.
func (db *DB) LoadFeedbackFromCSV(fileName string, sep string, hasHeader bool) error {
	users := make([]string, 0)
	items := make([]string, 0)
	feedbacks := make([]float64, 0)
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
		users = append(users, userId)
		items = append(items, itemId)
		feedbacks = append(feedbacks, feedback)
	}
	for i := range users {
		if err = db.InsertFeedback(users[i], items[i], feedbacks[i]); err != nil {
			return err
		}
	}
	return nil
}

// LoadItemsFromCSV imports items from a CSV file into the database.
func (db *DB) LoadItemsFromCSV(fileName string, sep string, hasHeader bool, dateColumn int, labelSep string, labelColumn int) error {
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
		// Insert
		if err = db.InsertItem(item, true); err != nil {
			return err
		}
	}
	return nil
}

// SaveFeedbackToCSV exports feedback from the database into a CSV file.
func (db *DB) SaveFeedbackToCSV(fileName string, sep string, header bool) error {
	// Open file
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	// Save feedback
	users, items, feedback, err := db.GetFeedback()
	if err != nil {
		return err
	}
	for i := range users {
		if _, err = file.WriteString(fmt.Sprintf("%v%v%v%v%v\n", users[i], sep, items[i], sep, feedback[i])); err != nil {
			return err
		}
	}
	return nil
}

// SaveItemsToCSV exports items from the database into a CSV file.
func (db *DB) SaveItemsToCSV(fileName string, sep string, header bool, date bool, labelSep string, label bool) error {
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
