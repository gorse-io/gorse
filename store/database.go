package store

import (
	"encoding/binary"
	"encoding/json"
	bolt "go.etcd.io/bbolt"
)

type DB struct {
	db *bolt.DB
}

type Feedback struct {
	UserId int
	ItemId int
	Rating float64
}

func Open(path string) (*DB, error) {
	db := new(DB)
	var err error
	db.db, err = bolt.Open(path, 0666, nil)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.db.Close()
}

func (db *DB) Insert(userId, itemId int, rating float64) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("ratings"))
		// get next index
		id, err := bucket.NextSequence()
		if err != nil {
			return err
		}
		// marshal data into bytes.
		buf, err := json.Marshal(Feedback{userId, itemId, rating})
		if err != nil {
			return err
		}
		// Persist bytes to users bucket.
		return bucket.Put(itob(id), buf)
	})
}

// itob returns an 8-byte big endian representation of v.
func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}
