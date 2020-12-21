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
	"database/sql"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

var (
	user    string
	pass    string
	prot    string
	addr    string
	netAddr string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	user = env("MYSQL_TEST_USER", "root")
	pass = env("MYSQL_TEST_PASS", "")
	prot = env("MYSQL_TEST_PROT", "tcp")
	addr = env("MYSQL_TEST_ADDR", "127.0.0.1:3306")
	netAddr = fmt.Sprintf("%s(%s)", prot, addr)
}

type testSQLDatabase struct {
	Database
}

func (db *testSQLDatabase) GetComm(t *testing.T) *sql.DB {
	var sqlDatabase *SQLDatabase
	var ok bool
	sqlDatabase, ok = db.Database.(*SQLDatabase)
	assert.True(t, ok)
	return sqlDatabase.db
}

func (db *testSQLDatabase) Close(t *testing.T) {
	err := db.Database.Close()
	assert.Nil(t, err)
}

func newTestSQLDatabase(t *testing.T, dbName string) *testSQLDatabase {
	database := new(testSQLDatabase)
	var err error
	// create database
	database.Database, err = Open(fmt.Sprintf("mysql://%s:%s@%s/?timeout=30s&parseTime=true", user, pass, netAddr))
	assert.Nil(t, err)
	dbName = "gorse_" + dbName
	databaseComm := database.GetComm(t)
	_, err = databaseComm.Exec("DROP DATABASE IF EXISTS " + dbName)
	assert.Nil(t, err)
	_, err = databaseComm.Exec("CREATE DATABASE " + dbName)
	assert.Nil(t, err)
	// connect database
	database.Database, err = Open(fmt.Sprintf("mysql://%s:%s@%s/%s?timeout=30s&parseTime=true", user, pass, netAddr, dbName))
	assert.Nil(t, err)
	// create schema
	err = database.Init()
	assert.Nil(t, err)
	return database
}

func TestSQLDatabase_Users(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_Users")
	defer db.Close(t)
	testUsers(t, db.Database)
}

func TestSQLDatabase_Feedback(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_Feedback")
	defer db.Close(t)
	testFeedback(t, db.Database)
}

func TestSQLDatabase_Item(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_Item")
	defer db.Close(t)
	testItems(t, db.Database)
}

func TestSQLDatabase_Ignore(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_Ignore")
	defer db.Close(t)
	testIgnore(t, db.Database)
}

func TestSQLDatabase_Meta(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_Meta")
	defer db.Close(t)
	testMeta(t, db.Database)
}

func TestSQLDatabase_List(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_List")
	defer db.Close(t)
	testList(t, db.Database)
}

func TestSQLDatabase_DeleteUser(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_DeleteUser")
	defer db.Close(t)
	testDeleteUser(t, db.Database)
}

func TestSQLDatabase_DeleteItem(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_DeleteItem")
	defer db.Close(t)
	testDeleteItem(t, db.Database)
}

func TestSQLDatabase_Prefix(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_Prefix")
	defer db.Close(t)
	testPrefix(t, db.Database)
}
