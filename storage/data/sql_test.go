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
package data

import (
	"database/sql"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

var (
	sqlUri string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	sqlUri = env("MYSQL_URI", "mysql://root@tcp(127.0.0.1:3306)/")
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
	database.Database, err = Open(sqlUri + "?timeout=30s&parseTime=true")
	assert.Nil(t, err)
	dbName = "gorse_" + dbName
	databaseComm := database.GetComm(t)
	_, err = databaseComm.Exec("DROP DATABASE IF EXISTS " + dbName)
	assert.Nil(t, err)
	_, err = databaseComm.Exec("CREATE DATABASE " + dbName)
	assert.Nil(t, err)
	err = database.Database.Close()
	assert.Nil(t, err)
	// connect database
	database.Database, err = Open(sqlUri + dbName + "?timeout=30s&parseTime=true")
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

func TestSQLDatabase_DeleteFeedback(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_DeleteFeedback")
	defer db.Close(t)
	testDeleteFeedback(t, db.Database)
}

func TestSQLDatabase_Measurements(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_Measurements")
	defer db.Close(t)
	testMeasurements(t, db.Database)
}

func TestSQLDatabase_TimeLimit(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_TimeLimit")
	defer db.Close(t)
	testTimeLimit(t, db.Database)
}

func TestSQLDatabase_GetClickThroughRate(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_GetClickThroughRate")
	defer db.Close(t)
	testGetClickThroughRate(t, db.Database)
}

func TestSQLDatabase_CountActiveUsers(t *testing.T) {
	db := newTestSQLDatabase(t, "TestSQLDatabase_CountActiveUsers")
	defer db.Close(t)
	testCountActiveUsers(t, db.Database)
}
