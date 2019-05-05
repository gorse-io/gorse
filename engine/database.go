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
	"math/rand"
	"os"
	"strconv"
	"strings"
)

const (
	bktMeta       = "meta"
	bktItems      = "items"
	bktPopular    = "popular"
	bktFeedback   = "feedback"
	bktNeighbors  = "neighbors"
	bktRecommends = "recommends"
)

func encodeInt(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func decodeInt(buf []byte) int {
	return int(binary.BigEndian.Uint64(buf))
}

// DB manages all data for the engine.
type DB struct {
	db *bolt.DB // based on BoltDB
}

func Open(path string) (*DB, error) {
	db := new(DB)
	var err error
	db.db, err = bolt.Open(path, 0666, nil)
	if err != nil {
		return nil, err
	}
	// Create buckets
	err = db.db.Update(func(tx *bolt.Tx) error {
		bucketNames := []string{bktMeta, bktItems, bktFeedback, bktRecommends, bktNeighbors, bktPopular}
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

func (db *DB) Close() error {
	return db.db.Close()
}

type Feedback struct {
	UserId   int
	ItemId   int
	Feedback float64
}

func (db *DB) InsertFeedback(userId, itemId int, feedback float64) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktFeedback))
		// Get next index
		index, err := bucket.NextSequence()
		if err != nil {
			return err
		}
		// Marshal data into bytes.
		buf, err := json.Marshal(Feedback{userId, itemId, feedback})
		if err != nil {
			return err
		}
		// Persist bytes to users bucket.
		return bucket.Put(encodeInt(int(index)), buf)
	})
	if err != nil {
		return err
	}
	return db.InsertItem(itemId)
}

func (db *DB) GetFeedback() (users []int, items []int, feedback []float64, err error) {
	err = db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktFeedback))
		return bucket.ForEach(func(k, v []byte) error {
			row := Feedback{}
			if err := json.Unmarshal(v, &row); err != nil {
				return err
			}
			users = append(users, row.UserId)
			items = append(items, row.ItemId)
			feedback = append(feedback, row.Feedback)
			return nil
		})
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return
}

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

func (db *DB) InsertItem(itemId int) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		if err := bucket.Put(encodeInt(itemId), nil); err != nil {
			return err
		}
		return nil
	})
}

func (db *DB) GetItems() ([]int, error) {
	items := make([]int, 0)
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		return bucket.ForEach(func(k, v []byte) error {
			items = append(items, decodeInt(k))
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

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

func (db *DB) GetMeta(name string) (string, error) {
	var value string
	err := db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktMeta))
		value = string(bucket.Get([]byte(name)))
		return nil
	})
	return value, err
}

func (db *DB) SetMeta(name string, val string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktMeta))
		if err := bucket.Put([]byte(name), []byte(val)); err != nil {
			return err
		}
		return nil
	})
}

type RecommendedItem struct {
	ItemId int
	Score  float64
}

func (db *DB) GetRandom(n int) ([]RecommendedItem, error) {
	// count items
	count, err := db.CountItems()
	if err != nil {
		return nil, err
	}
	n = base.Min([]int{count, n})
	threshold := float64(n) / float64(count)
	items := make([]RecommendedItem, 0)
	err = db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bktItems))
		return bucket.ForEach(func(k, v []byte) error {
			// Sample
			random := rand.Float64()
			if random < threshold && len(items) < n {
				items = append(items, RecommendedItem{ItemId: decodeInt(k)})
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (db *DB) SetRecommends(userId int, items []RecommendedItem) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktRecommends))
		// Marshal data into bytes
		buf, err := json.Marshal(items)
		if err != nil {
			return err
		}
		// Persist bytes to bucket
		return bucket.Put(encodeInt(userId), buf)
	})
}

func (db *DB) GetRecommends(userId int, n int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.View(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktRecommends))
		// Unmarshal data into bytes
		buf := bucket.Get(encodeInt(userId))
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

func (db *DB) SetNeighbors(userId int, items []RecommendedItem) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktNeighbors))
		// Marshal data into bytes
		buf, err := json.Marshal(items)
		if err != nil {
			return err
		}
		// Persist bytes to bucket
		return bucket.Put(encodeInt(userId), buf)
	})
}

func (db *DB) GetNeighbors(userId int, n int) ([]RecommendedItem, error) {
	var items []RecommendedItem
	err := db.db.View(func(tx *bolt.Tx) error {
		// Get bucket
		bucket := tx.Bucket([]byte(bktNeighbors))
		// Unmarshal data into bytes
		buf := bucket.Get(encodeInt(userId))
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

func (db *DB) ToDataSet() (*core.DataSet, error) {
	users, items, feedback, err := db.GetFeedback()
	if err != nil {
		return nil, err
	}
	return core.NewDataSet(users, items, feedback), nil
}

func (db *DB) LoadFeedbackFromCSV(fileName string, sep string, hasHeader bool) error {
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
		if err = db.InsertFeedback(userId, itemId, feedback); err != nil {
			return err
		}
	}
	return nil
}

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
		if len(fields) < 2 {
			continue
		}
		itemId, _ := strconv.Atoi(fields[0])
		if err = db.InsertItem(itemId); err != nil {
			return err
		}
	}
	return err
}

func (db *DB) SaveFeedbackToCSV(fileName string, sep string) error {
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

func (db *DB) SaveItemsToCSV(fileName string, sep string) error {
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
