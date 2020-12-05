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
	"testing"
)

type mockSQLDatabase struct {
	Database
}

func newMockSQLDatabase(t *testing.T, name string) *mockSQLDatabase {
	database := new(mockSQLDatabase)
	var err error
	database.Database, err = Open("ramsql://" + name)
	assert.Nil(t, err)
	err = database.Init()
	assert.Nil(t, err)
	return database
}

func (mock *mockSQLDatabase) Close(t *testing.T) {
	err := mock.Database.Close()
	assert.Nil(t, err)
}

func TestSQLDatabase_Users(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Users")
	defer db.Close(t)
	testUsers(t, db.Database)
}

func TestSQLDatabase_Feedback(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Feedback")
	defer db.Close(t)
	testFeedback(t, db.Database)
}

func TestSQLDatabase_Item(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Item")
	defer db.Close(t)
	testItems(t, db.Database)
}

func TestSQLDatabase_Ignore(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Ignore")
	defer db.Close(t)
	testIgnore(t, db.Database)
}

func TestSQLDatabase_Meta(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Meta")
	defer db.Close(t)
	testMeta(t, db.Database)
}

func TestSQLDatabase_List(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_List")
	defer db.Close(t)
	testList(t, db.Database)
}

func TestSQLDatabase_DeleteUser(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_DeleteUser")
	defer db.Close(t)
	testDeleteUser(t, db.Database)
}

func TestSQLDatabase_DeleteItem(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_DeleteItem")
	defer db.Close(t)
	testDeleteItem(t, db.Database)
}

func TestSQLDatabase_Prefix(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Prefix")
	defer db.Close(t)
	testPrefix(t, db.Database)
}
