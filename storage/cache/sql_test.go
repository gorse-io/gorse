// Copyright 2022 gorse Project Authors
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

package cache

import (
	"database/sql"
	"github.com/stretchr/testify/assert"
	"os"
	"runtime"
	"strings"
	"testing"
)

var (
	mySqlDSN    string
	postgresDSN string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	mySqlDSN = env("MYSQL_URI", "mysql://root:password@tcp(127.0.0.1:3306)/")
	postgresDSN = env("POSTGRES_URI", "postgres://gorse:gorse_pass@127.0.0.1/")
}

type testSQLDatabase struct {
	Database
}

func (db *testSQLDatabase) GetComm(t *testing.T) *sql.DB {
	var sqlDatabase *SQLDatabase
	var ok bool
	sqlDatabase, ok = db.Database.(*SQLDatabase)
	assert.True(t, ok)
	return sqlDatabase.client
}

func (db *testSQLDatabase) Close(t *testing.T) {
	err := db.Database.Close()
	assert.NoError(t, err)
}

func newTestPostgresDatabase(t *testing.T) *testSQLDatabase {
	// retrieve test name
	var testName string
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		splits := strings.Split(details.Name(), ".")
		testName = splits[len(splits)-1]
	} else {
		t.Fatalf("failed to retrieve test name")
	}

	database := new(testSQLDatabase)
	var err error
	// create database
	database.Database, err = Open(postgresDSN + "?sslmode=disable&TimeZone=UTC")
	assert.NoError(t, err)
	dbName := "gorse_" + testName
	databaseComm := database.GetComm(t)
	_, err = databaseComm.Exec("DROP DATABASE IF EXISTS " + dbName)
	assert.NoError(t, err)
	_, err = databaseComm.Exec("CREATE DATABASE " + dbName)
	assert.NoError(t, err)
	err = database.Database.Close()
	assert.NoError(t, err)
	// connect database
	database.Database, err = Open(postgresDSN + strings.ToLower(dbName) + "?sslmode=disable&TimeZone=UTC")
	assert.NoError(t, err)
	// create schema
	err = database.Init()
	assert.NoError(t, err)
	return database
}

func TestPostgres_Meta(t *testing.T) {
	db := newTestPostgresDatabase(t)
	defer db.Close(t)
	testMeta(t, db.Database)
}

func TestPostgres_Sort(t *testing.T) {
	db := newTestPostgresDatabase(t)
	defer db.Close(t)
	testSort(t, db.Database)
}

func TestPostgres_Set(t *testing.T) {
	db := newTestPostgresDatabase(t)
	defer db.Close(t)
	testSet(t, db.Database)
}

func newTestMySQLDatabase(t *testing.T) *testSQLDatabase {
	// retrieve test name
	var testName string
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		splits := strings.Split(details.Name(), ".")
		testName = splits[len(splits)-1]
	} else {
		t.Fatalf("failed to retrieve test name")
	}

	database := new(testSQLDatabase)
	var err error
	// create database
	database.Database, err = Open(mySqlDSN + "?timeout=30s&parseTime=true")
	assert.NoError(t, err)
	dbName := "gorse_" + testName
	databaseComm := database.GetComm(t)
	_, err = databaseComm.Exec("DROP DATABASE IF EXISTS " + dbName)
	assert.NoError(t, err)
	_, err = databaseComm.Exec("CREATE DATABASE " + dbName)
	assert.NoError(t, err)
	err = database.Database.Close()
	assert.NoError(t, err)
	// connect database
	database.Database, err = Open(mySqlDSN + dbName + "?timeout=30s&parseTime=true")
	assert.NoError(t, err)
	// create schema
	err = database.Init()
	assert.NoError(t, err)
	return database
}

func TestMySQL_Meta(t *testing.T) {
	db := newTestMySQLDatabase(t)
	defer db.Close(t)
	testMeta(t, db.Database)
}

func TestMySQL_Sort(t *testing.T) {
	db := newTestMySQLDatabase(t)
	defer db.Close(t)
	testSort(t, db.Database)
}

func TestMySQL_Set(t *testing.T) {
	db := newTestMySQLDatabase(t)
	defer db.Close(t)
	testSet(t, db.Database)
}
