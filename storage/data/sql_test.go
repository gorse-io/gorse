// Copyright 2021 gorse Project Authors
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
	"strings"
	"testing"
)

var (
	mySqlDSN      string
	postgresDSN   string
	clickhouseDSN string
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
	clickhouseDSN = env("CLICKHOUSE_URI", "clickhouse://127.0.0.1:8123/")
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

func newTestMySQLDatabase(t *testing.T, dbName string) *testSQLDatabase {
	database := new(testSQLDatabase)
	var err error
	// create database
	database.Database, err = Open(mySqlDSN + "?timeout=30s&parseTime=true")
	assert.NoError(t, err)
	dbName = "gorse_" + dbName
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

func TestMySQL_Users(t *testing.T) {
	db := newTestMySQLDatabase(t, "TestMySQL_Users")
	defer db.Close(t)
	testUsers(t, db.Database)
}

func TestMySQL_Feedback(t *testing.T) {
	db := newTestMySQLDatabase(t, "TestMySQL_Feedback")
	defer db.Close(t)
	testFeedback(t, db.Database)
}

func TestMySQL_Item(t *testing.T) {
	db := newTestMySQLDatabase(t, "TestMySQL_Item")
	defer db.Close(t)
	testItems(t, db.Database)
}

func TestMySQL_DeleteUser(t *testing.T) {
	db := newTestMySQLDatabase(t, "TestMySQL_DeleteUser")
	defer db.Close(t)
	testDeleteUser(t, db.Database)
}

func TestMySQL_DeleteItem(t *testing.T) {
	db := newTestMySQLDatabase(t, "TestMySQL_DeleteItem")
	defer db.Close(t)
	testDeleteItem(t, db.Database)
}

func TestMySQL_DeleteFeedback(t *testing.T) {
	db := newTestMySQLDatabase(t, "TestMySQL_DeleteFeedback")
	defer db.Close(t)
	testDeleteFeedback(t, db.Database)
}

func TestMySQL_Measurements(t *testing.T) {
	db := newTestMySQLDatabase(t, "TestMySQL_Measurements")
	defer db.Close(t)
	testMeasurements(t, db.Database)
}

func TestMySQL_TimeLimit(t *testing.T) {
	db := newTestMySQLDatabase(t, "TestMySQL_TimeLimit")
	defer db.Close(t)
	testTimeLimit(t, db.Database)
}

func TestMySQL_GetClickThroughRate(t *testing.T) {
	db := newTestMySQLDatabase(t, "TestMySQL_GetClickThroughRate")
	defer db.Close(t)
	testGetClickThroughRate(t, db.Database)
}

func newTestPostgresDatabase(t *testing.T, dbName string) *testSQLDatabase {
	database := new(testSQLDatabase)
	var err error
	// create database
	database.Database, err = Open(postgresDSN + "?sslmode=disable&TimeZone=UTC")
	assert.NoError(t, err)
	dbName = "gorse_" + dbName
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

func TestPostgres_Users(t *testing.T) {
	db := newTestPostgresDatabase(t, "TestPostgres_Users")
	defer db.Close(t)
	testUsers(t, db.Database)
}

func TestPostgres_Feedback(t *testing.T) {
	db := newTestPostgresDatabase(t, "TestPostgres_Feedback")
	defer db.Close(t)
	testFeedback(t, db.Database)
}

func TestPostgres_Item(t *testing.T) {
	db := newTestPostgresDatabase(t, "TestPostgres_Item")
	defer db.Close(t)
	testItems(t, db.Database)
}

func TestPostgres_DeleteUser(t *testing.T) {
	db := newTestPostgresDatabase(t, "TestPostgres_DeleteUser")
	defer db.Close(t)
	testDeleteUser(t, db.Database)
}

func TestPostgres_DeleteItem(t *testing.T) {
	db := newTestPostgresDatabase(t, "TestPostgres_DeleteItem")
	defer db.Close(t)
	testDeleteItem(t, db.Database)
}

func TestPostgres_DeleteFeedback(t *testing.T) {
	db := newTestPostgresDatabase(t, "TestPostgres_DeleteFeedback")
	defer db.Close(t)
	testDeleteFeedback(t, db.Database)
}

func TestPostgres_Measurements(t *testing.T) {
	db := newTestPostgresDatabase(t, "TestPostgres_Measurements")
	defer db.Close(t)
	testMeasurements(t, db.Database)
}

func TestPostgres_TimeLimit(t *testing.T) {
	db := newTestPostgresDatabase(t, "TestPostgres_TimeLimit")
	defer db.Close(t)
	testTimeLimit(t, db.Database)
}

func TestPostgres_GetClickThroughRate(t *testing.T) {
	db := newTestPostgresDatabase(t, "TestPostgres_GetClickThroughRate")
	defer db.Close(t)
	testGetClickThroughRate(t, db.Database)
}

func newTestClickHouseDatabase(t *testing.T, dbName string) *testSQLDatabase {
	database := new(testSQLDatabase)
	var err error
	// create database
	database.Database, err = Open(clickhouseDSN)
	assert.NoError(t, err)
	dbName = "gorse_" + dbName
	databaseComm := database.GetComm(t)
	_, err = databaseComm.Exec("DROP DATABASE IF EXISTS " + dbName)
	assert.NoError(t, err)
	_, err = databaseComm.Exec("CREATE DATABASE " + dbName)
	assert.NoError(t, err)
	err = database.Database.Close()
	assert.NoError(t, err)
	// connect database
	database.Database, err = Open(clickhouseDSN + dbName + "?mutations_sync=2")
	assert.NoError(t, err)
	// create schema
	err = database.Init()
	assert.NoError(t, err)
	return database
}

func TestClickHouse_Users(t *testing.T) {
	db := newTestClickHouseDatabase(t, "TestClickHouse_Users")
	defer db.Close(t)
	testUsers(t, db.Database)
}

func TestClickHouse_Feedback(t *testing.T) {
	db := newTestClickHouseDatabase(t, "TestClickHouse_Feedback")
	defer db.Close(t)
	testFeedback(t, db.Database)
}

func TestClickHouse_Item(t *testing.T) {
	db := newTestClickHouseDatabase(t, "TestClickHouse_Item")
	defer db.Close(t)
	testItems(t, db.Database)
}

func TestClickHouse_DeleteUser(t *testing.T) {
	db := newTestClickHouseDatabase(t, "TestClickHouse_DeleteUser")
	defer db.Close(t)
	testDeleteUser(t, db.Database)
}

func TestClickHouse_DeleteItem(t *testing.T) {
	db := newTestClickHouseDatabase(t, "TestClickHouse_DeleteItem")
	defer db.Close(t)
	testDeleteItem(t, db.Database)
}

func TestClickHouse_DeleteFeedback(t *testing.T) {
	db := newTestClickHouseDatabase(t, "TestClickHouse_DeleteFeedback")
	defer db.Close(t)
	testDeleteFeedback(t, db.Database)
}

func TestClickHouse_Measurements(t *testing.T) {
	db := newTestClickHouseDatabase(t, "TestClickHouse_Measurements")
	defer db.Close(t)
	testMeasurements(t, db.Database)
}

func TestClickHouse_TimeLimit(t *testing.T) {
	db := newTestClickHouseDatabase(t, "TestClickHouse_TimeLimit")
	defer db.Close(t)
	testTimeLimit(t, db.Database)
}

func TestClickHouse_GetClickThroughRate(t *testing.T) {
	db := newTestClickHouseDatabase(t, "TestClickHouse_GetClickThroughRate")
	defer db.Close(t)
	testGetClickThroughRate(t, db.Database)
}
