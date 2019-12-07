package engine

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
	bktGlobal        = "global"        // Bucket name for global data
	bktItems         = "items"         // Bucket name for items
	bktFeedback      = "feedback"      // Bucket name for feedback
	BucketNeighbors  = "neighbors"     // Bucket name for neighbors
	BucketRecommends = "recommends"    // Bucket name for recommendations
	bktUserFeedback  = "user_feedback" // Bucket name for user feedback
)

const (
	ListPop    = "pop"    // List name for popular items
	ListLatest = "latest" // List name for latest items
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
		bucketNames := []string{bktGlobal, bktItems, bktFeedback, BucketRecommends, BucketNeighbors, bktUserFeedback}
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
	UserId int
	ItemId int
}

// Feedback stores feedback.
type Feedback struct {
	FeedbackKey
	Rating float64
}

// InsertFeedback inserts a feedback into the database.
func (db *DB) InsertFeedback(userId, itemId int, feedback float64) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktFeedback))
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
	if err = db.InsertItem(itemId, nil); err != nil {
		return err
	}
	return db.InsertUserFeedback(userId, itemId, feedback)
}

// InsertMultiFeedback inserts multiple feedback into the database.
func (db *DB) InsertMultiFeedback(userId, itemId []int, feedback []float64) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktFeedback))
		for i := range feedback {
			// Marshal data into bytes.
			key, err := json.Marshal(FeedbackKey{userId[i], itemId[i]})
			if err != nil {
				return err
			}
			// Persist bytes to users bucket.
			if err = bucket.Put(key, encodeFloat(feedback[i])); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// collect and insert unique items
	itemSet := make(map[int]bool)
	items := make([]int, 0)
	for _, id := range itemId {
		if _, exist := itemSet[id]; !exist {
			itemSet[id] = true
			items = append(items, id)
		}
	}
	if err = db.InsertItems(items, nil); err != nil {
		return err
	}
	return db.InsertMultiUserFeedback(userId, itemId, feedback)
}

// InsertUserFeedback inserts a feedback into the user feedback bucket of the database.
func (db *DB) InsertUserFeedback(userId, itemId int, feedback float64) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktUserFeedback))
		// Get user's bucket
		userBucket, err := bucket.CreateBucketIfNotExists(encodeInt(userId))
		if err != nil {
			return err
		}
		// Persist bytes to users bucket.
		return userBucket.Put(encodeInt(itemId), encodeFloat(feedback))
	})
	return err
}

// InsertMultiUserFeedback inserts multiple feedback into the user feedback bucket of the database.
func (db *DB) InsertMultiUserFeedback(userId, itemId []int, feedback []float64) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktUserFeedback))
		for i := range feedback {
			// Get user's bucket
			userBucket, err := bucket.CreateBucketIfNotExists(encodeInt(userId[i]))
			if err != nil {
				return err
			}
			// Persist bytes to users bucket.
			if err = userBucket.Put(encodeInt(itemId[i]), encodeFloat(feedback[i])); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// GetFeedback returns all feedback in the database.
func (db *DB) GetFeedback() (users []int, items []int, feedback []float64, err error) {
	err = db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktFeedback))
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

// GetUsers get all user IDs.
func (db *DB) GetUsers() ([]int, error) {
	var users []int
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktUserFeedback))
		return bucket.ForEach(func(k, v []byte) error {
			if v == nil {
				users = append(users, decodeInt(k))
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return users, nil
}

// GetUserFeedback get a user's feedback.
func (db *DB) GetUserFeedback(userId int) ([]Feedback, error) {
	var items []Feedback
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktUserFeedback))
		// Get user's bucket
		userBucket := bucket.Bucket(encodeInt(userId))
		if userBucket == nil {
			return bolt.ErrBucketNotFound
		}
		return userBucket.ForEach(func(k, v []byte) error {
			itemId := decodeInt(k)
			feedback := decodeFloat(v)
			items = append(items, Feedback{FeedbackKey{userId, itemId}, feedback})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// CountFeedback returns the number of feedback in the database.
func (db *DB) CountFeedback() (int, error) {
	count := 0
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktFeedback))
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
	Id         int
	Popularity float64
	Timestamp  time.Time
}

// InsertItems inserts multiple items into the database.
func (db *DB) InsertItems(itemId []int, timestamps []time.Time) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		for i, v := range itemId {
			// Retrieve old item
			item := Item{Id: v}
			buf := bucket.Get(encodeInt(v))
			if buf != nil {
				if err := json.Unmarshal(buf, &item); err != nil {
					return err
				}
			}
			// Update timestamp
			if timestamps != nil {
				item.Timestamp = timestamps[i]
			}
			// Marshal data into bytes
			buf, err := json.Marshal(item)
			if err != nil {
				return err
			}
			if err := bucket.Put(encodeInt(v), buf); err != nil {
				return err
			}
		}
		return nil
	})
}

// InsertItem inserts a item into the database.
func (db *DB) InsertItem(itemId int, timestamp *time.Time) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		// Retrieve old item
		item := Item{Id: itemId}
		buf := bucket.Get(encodeInt(itemId))
		if buf != nil {
			if err := json.Unmarshal(buf, &item); err != nil {
				return err
			}
		}
		// Update timestamp
		if timestamp != nil {
			item.Timestamp = *timestamp
		}
		// Marshal data into bytes
		buf, err := json.Marshal(item)
		if err != nil {
			return err
		}
		return bucket.Put(encodeInt(itemId), buf)
	})
}

// GetItem gets a item from database by item ID.
func (db *DB) GetItem(itemId int) (Item, error) {
	item := Item{}
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		buf := bucket.Get(encodeInt(itemId))
		if buf == nil {
			return fmt.Errorf("item %d not found", itemId)
		}
		if err := json.Unmarshal(buf, &item); err != nil {
			return err
		}
		return nil
	})
	return item, err
}

// GetItems returns all items in the dataset.
func (db *DB) GetItems() ([]Item, error) {
	items := make([]Item, 0)
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		return bucket.ForEach(func(k, v []byte) error {
			item := Item{Id: decodeInt(k)}
			if v != nil {
				err := json.Unmarshal(v, &item)
				if err != nil {
					return err
				}
			}
			items = append(items, item)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetItem gets items from database by item IDs.
func (db *DB) GetItemsByID(id []int) ([]Item, error) {
	items := make([]Item, len(id))
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		for i, v := range id {
			item := Item{}
			buf := bucket.Get(encodeInt(v))
			if buf == nil {
				return fmt.Errorf("item %d not found", v)
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
		bucket := tx.Bucket([]byte(bktItems))
		count = bucket.Stats().KeyN
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountUsers returns the number of users in the database.
func (db *DB) CountUsers() (int, error) {
	count := 0
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktUserFeedback))
		count = bucket.Stats().InlineBucketN
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
		bucket := tx.Bucket([]byte(bktGlobal))
		value = string(bucket.Get([]byte(name)))
		return nil
	})
	return value, err
}

// SetMeta sets the value of a metadata.
func (db *DB) SetMeta(name string, val string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktGlobal))
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
		bucket := tx.Bucket([]byte(bktItems))
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
func (db *DB) PutIdentList(bucketName string, id int, items []RecommendedItem) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bucketName))
		// Marshal data into bytes
		buf, err := json.Marshal(items)
		if err != nil {
			return err
		}
		// Persist bytes to bucket
		return bucket.Put(encodeInt(id), buf)
	})
}

// GetRecommends gets n recommendations for a user.
func (db *DB) GetIdentList(bucketName string, id int, n int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.View(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bucketName))
		// Unmarshal data into bytes
		buf := bucket.Get(encodeInt(id))
		if buf == nil {
			return fmt.Errorf("%v not found", id)
		}
		return json.Unmarshal(buf, &items)
	})
	if err != nil {
		return nil, err
	}
	if n > 0 && n < len(items) {
		items = items[:n]
	}
	return items, nil
}

// UpdatePopularity update popularity of items.
func (db *DB) UpdatePopularity(itemId []int, popularity []float64) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktItems))
		for i, id := range itemId {
			// Unmarshal data from bytes
			item := Item{Id: id}
			buf := bucket.Get(encodeInt(id))
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
			if err = bucket.Put(encodeInt(id), buf); err != nil {
				return err
			}
		}
		return nil
	})
}

// PutList saves a list into the database.
func (db *DB) PutList(name string, items []RecommendedItem) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktGlobal))
		// Marshal data into bytes
		buf, err := json.Marshal(items)
		if err != nil {
			return err
		}
		// Persist bytes to bucket
		return bucket.Put([]byte(name), buf)
	})
}

// GetList gets a list from the database.
func (db *DB) GetList(name string, n int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.View(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktGlobal))
		// Unmarshal data into bytes
		buf := bucket.Get([]byte(name))
		return json.Unmarshal(buf, &items)
	})
	if err != nil {
		return nil, err
	}
	if n > 0 && n < len(items) {
		items = items[:n]
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
	users := make([]int, 0)
	items := make([]int, 0)
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
		userId, _ := strconv.Atoi(fields[0])
		itemId, _ := strconv.Atoi(fields[1])
		feedback := 0.0
		if len(fields) > 2 {
			feedback, _ = strconv.ParseFloat(fields[2], 32)
		}
		users = append(users, userId)
		items = append(items, itemId)
		feedbacks = append(feedbacks, feedback)
	}
	return db.InsertMultiFeedback(users, items, feedbacks)
}

// LoadItemsFromCSV imports items from a CSV file into the database.
func (db *DB) LoadItemsFromCSV(fileName string, sep string, hasHeader bool, dateColumn int) error {
	// Open file
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// Read CSV file
	scanner := bufio.NewScanner(file)
	itemIds := make([]int, 0)
	timestamps := make([]time.Time, 0)
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
		itemId, _ := strconv.Atoi(fields[0])
		itemIds = append(itemIds, itemId)
		// Parse date
		if dateColumn > 0 && dateColumn < len(fields) {
			t, err := dateparse.ParseAny(fields[dateColumn])
			if err != nil && len(fields[dateColumn]) > 0 {
				return err
			}
			timestamps = append(timestamps, t)
		}
	}
	// Insert
	if dateColumn <= 0 {
		timestamps = nil
	}
	return db.InsertItems(itemIds, timestamps)
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
func (db *DB) SaveItemsToCSV(fileName string, sep string, header bool, date bool) error {
	// Open file
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	// Save items
	items, err := db.GetItems()
	if err != nil {
		return err
	}
	if date {
		for _, item := range items {
			if _, err := file.WriteString(fmt.Sprintf("%d%s%s\n", item.Id, sep, item.Timestamp.String())); err != nil {
				return err
			}
		}
	} else {
		for _, item := range items {
			if _, err := file.WriteString(fmt.Sprintln(item.Id)); err != nil {
				return err
			}
		}
	}
	return nil
}
