package engine

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	bolt "go.etcd.io/bbolt"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
)

const (
	bktMeta         = "meta"
	bktItems        = "items"
	bktPopular      = "popular"
	bktFeedback     = "feedback"
	bktNeighbors    = "neighbors"
	bktRecommends   = "recommends"
	bktUserFeedback = "user_feedback"
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
		bucketNames := []string{bktMeta, bktItems, bktFeedback, bktRecommends, bktNeighbors, bktPopular, bktUserFeedback}
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

type FeedbackKey struct {
	UserId string
	ItemId string
}

// InsertFeedback inserts a feedback into the database.
func (db *DB) InsertFeedback(userId, itemId string, feedback float64) error {
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
	if err = db.InsertItem(itemId); err != nil {
		return err
	}
	return db.InsertUserFeedback(userId, itemId, feedback)
}

// InsertMultiFeedback inserts multiple feedback into the database.
func (db *DB) InsertMultiFeedback(userId, itemId []string, feedback []float64) error {
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
	if err = db.InsertMultiItems(itemId); err != nil {
		return err
	}
	return db.InsertMultiUserFeedback(userId, itemId, feedback)
}

// InsertUserFeedback inserts a feedback into the user feedback bucket of the database.
func (db *DB) InsertUserFeedback(userId, itemId string, feedback float64) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktUserFeedback))
		// Get user's bucket
		userBucket, err := bucket.CreateBucketIfNotExists([]byte(userId))
		if err != nil {
			return err
		}
		// Persist bytes to users bucket.
		return userBucket.Put([]byte(itemId), encodeFloat(feedback))
	})
	return err
}

// InsertMultiUserFeedback inserts multiple feedback into the user feedback bucket of the database.
func (db *DB) InsertMultiUserFeedback(userId, itemId []string, feedback []float64) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktUserFeedback))
		for i := range feedback {
			// Get user's bucket
			userBucket, err := bucket.CreateBucketIfNotExists([]byte(userId[i]))
			if err != nil {
				return err
			}
			// Persist bytes to users bucket.
			if err = userBucket.Put([]byte(itemId[i]), encodeFloat(feedback[i])); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// GetFeedback returns all feedback in the database.
func (db *DB) GetFeedback() (users, items []string, feedback []float64, err error) {
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

func (db *DB) GetUserFeedback(userId int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktUserFeedback))
		// Get user's bucket
		userBucket := bucket.Bucket(encodeInt(userId))
		if userBucket == nil {
			return bolt.ErrBucketNotFound
		}
		return userBucket.ForEach(func(k, v []byte) error {
			itemId := string(k)
			feedback := decodeFloat(v)
			items = append(items, RecommendedItem{itemId, feedback})
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

// InsertMultiItems inserts multiple items into the database.
func (db *DB) InsertMultiItems(itemId []string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		for _, v := range itemId {
			if err := bucket.Put([]byte(v), nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// InsertItem inserts a item into the database.
func (db *DB) InsertItem(itemId string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		return bucket.Put([]byte(itemId), nil)
	})
}

// GetItems returns all items in the dataset.
func (db *DB) GetItems() ([]string, error) {
	items := make([]string, 0)
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		return bucket.ForEach(func(k, v []byte) error {
			items = append(items, string(k))
			return nil
		})
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
		bucket := tx.Bucket([]byte(bktMeta))
		value = string(bucket.Get([]byte(name)))
		return nil
	})
	return value, err
}

// SetMeta sets the value of a metadata.
func (db *DB) SetMeta(name string, val string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktMeta))
		return bucket.Put([]byte(name), []byte(val))
	})
}

// RecommendedItem is the structure for a recommended item.
type RecommendedItem struct {
	ItemId string  // identifier
	Score  float64 // score
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
				items = append(items, RecommendedItem{ItemId: string(k)})
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
func (db *DB) SetRecommends(userId string, items []RecommendedItem) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktRecommends))
		// Marshal data into bytes
		buf, err := json.Marshal(items)
		if err != nil {
			return err
		}
		// Persist bytes to bucket
		return bucket.Put([]byte(userId), buf)
	})
}

// GetRecommends gets n recommendations for a user.
func (db *DB) GetRecommends(userId string, n int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.View(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktRecommends))
		// Unmarshal data into bytes
		buf := bucket.Get([]byte(userId))
		if buf == nil {
			return fmt.Errorf("no recommends for user %v", userId)
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

// SetNeighbors sets neighbors for a item.
func (db *DB) SetNeighbors(itemId string, items []RecommendedItem) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktNeighbors))
		// Marshal data into bytes
		buf, err := json.Marshal(items)
		if err != nil {
			return err
		}
		// Persist bytes to bucket
		return bucket.Put([]byte(itemId), buf)
	})
}

// GetNeighbors gets n neighbors for a item.
func (db *DB) GetNeighbors(ItemId string, n int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.View(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktNeighbors))
		// Unmarshal data into bytes
		buf := bucket.Get([]byte(ItemId))
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

// SetPopular sets popular items in the database.
func (db *DB) SetPopular(items []RecommendedItem) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktPopular))
		// Marshal data into bytes
		buf, err := json.Marshal(items)
		if err != nil {
			return err
		}
		// Persist bytes to bucket
		return bucket.Put(encodeInt(0), buf)
	})
}

// GetPopular returns popular items from the database.
func (db *DB) GetPopular(n int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.View(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktPopular))
		// Unmarshal data into bytes
		buf := bucket.Get(encodeInt(0))
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
	return db.InsertMultiFeedback(users, items, feedbacks)
}

// LoadItemsFromCSV imports items from a CSV file into the database.
func (db *DB) LoadItemsFromCSV(fileName string, sep string, hasHeader bool) error {
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
		itemId := fields[0]
		if err = db.InsertItem(itemId); err != nil {
			return err
		}
	}
	return err
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
func (db *DB) SaveItemsToCSV(fileName string, sep string, header bool) error {
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
	for _, itemId := range items {
		if _, err := file.WriteString(fmt.Sprintln(itemId)); err != nil {
			return err
		}
	}
	return nil
}
