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
	"github.com/stretchr/testify/assert"
	"github.com/thanhpk/randstr"
	"os"
	"path"
	"testing"
)

type mockBadger struct {
	Database
	DataFolder string
}

func newMockBadger(t *testing.T) *mockBadger {
	db := new(mockBadger)
	// Create folder
	db.DataFolder = path.Join(os.TempDir(), randstr.String(16))
	err := os.Mkdir(db.DataFolder, 0777)
	assert.Nil(t, err)
	// Create database
	db.Database, err = Open(bagderPrefix + db.DataFolder)
	assert.Nil(t, err)
	return db
}

func (db mockBadger) Close(t *testing.T) {
	err := db.Database.Close()
	assert.Nil(t, err)
	err = os.RemoveAll(db.DataFolder)
	assert.Nil(t, err)
}

func TestBadger_Users(t *testing.T) {
	db := newMockBadger(t)
	defer db.Close(t)
	testUsers(t, db.Database)
}

func TestBadger_Feedback(t *testing.T) {
	db := newMockBadger(t)
	defer db.Close(t)
	testFeedback(t, db.Database)
}

func TestBagder_Item(t *testing.T) {
	db := newMockBadger(t)
	defer db.Close(t)
	testItems(t, db.Database)
}

func TestBadger_Ignore(t *testing.T) {
	db := newMockBadger(t)
	defer db.Close(t)
	testIgnore(t, db.Database)
}

func TestBadger_Meta(t *testing.T) {
	db := newMockBadger(t)
	defer db.Close(t)
	testMeta(t, db.Database)
}

func TestBadger_List(t *testing.T) {
	db := newMockBadger(t)
	defer db.Close(t)
	testList(t, db.Database)
}

func TestBadger_DeleteUser(t *testing.T) {
	db := newMockBadger(t)
	defer db.Close(t)
	testDeleteUser(t, db.Database)
}

func TestBadger_DeleteItem(t *testing.T) {
	db := newMockBadger(t)
	defer db.Close(t)
	testDeleteItem(t, db.Database)
}
