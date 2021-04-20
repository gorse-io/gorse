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
package local

import (
	"github.com/dgraph-io/badger/v2"
)

const (
	prefixMeta = "meta/" // prefix for meta data
)

const (
	NodeName = "node_name"
)

func newKey(parts ...[]byte) []byte {
	k := make([]byte, 0)
	for _, p := range parts {
		k = append(k, p...)
	}
	return k
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
