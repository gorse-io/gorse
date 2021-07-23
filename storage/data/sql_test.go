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
	mySqlDSN = env("MYSQL_URI", "mysql://root@tcp(127.0.0.1:3306)/")
	postgresDSN = env("POSTGRES_URI", "postgres://postgres:pg_pass@127.0.0.1/")
}

type testMySQL struct {
	Database
}

func (db *testMySQL) GetComm(t *testing.T) *sql.DB {
	var sqlDatabase *SQLDatabase
	var ok bool
	sqlDatabase, ok = db.Database.(*SQLDatabase)
	assert.True(t, ok)
	return sqlDatabase.client
}

func (db *testMySQL) Close(t *testing.T) {
	err := db.Database.Close()
	assert.NoError(t, err)
}

func newTestMySQLDatabase(t *testing.T, dbName string) *testMySQL {
	database := new(testMySQL)
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

func TestMySQL_CountActiveUsers(t *testing.T) {
	db := newTestMySQLDatabase(t, "TestMySQL_CountActiveUsers")
	defer db.Close(t)
	testCountActiveUsers(t, db.Database)
}

type testPostgres struct {
	Database
}

func (db *testPostgres) GetComm(t *testing.T) *sql.DB {
	var sqlDatabase *SQLDatabase
	var ok bool
	sqlDatabase, ok = db.Database.(*SQLDatabase)
	assert.True(t, ok)
	return sqlDatabase.client
}

func (db *testPostgres) Close(t *testing.T) {
	err := db.Database.Close()
	assert.NoError(t, err)
}

func newTestPostgresDatabase(t *testing.T, dbName string) *testPostgres {
	database := new(testPostgres)
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

func TestPostgres_CountActiveUsers(t *testing.T) {
	db := newTestPostgresDatabase(t, "TestPostgres_CountActiveUsers")
	defer db.Close(t)
	testCountActiveUsers(t, db.Database)
}
