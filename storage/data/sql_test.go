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
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/zhenghaoz/gorse/storage"
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

type MySQLTestSuite struct {
	baseTestSuite
}

func (suite *MySQLTestSuite) SetupSuite() {
	// create database
	databaseComm, err := sql.Open("mysql", mySqlDSN[len(storage.MySQLPrefix):])
	suite.NoError(err)
	const dbName = "gorse_data_test"
	_, err = databaseComm.Exec("DROP DATABASE IF EXISTS " + dbName)
	suite.NoError(err)
	_, err = databaseComm.Exec("CREATE DATABASE " + dbName)
	suite.NoError(err)
	err = databaseComm.Close()
	suite.NoError(err)
	// connect database
	suite.Database, err = Open(mySqlDSN+dbName, "gorse_")
	suite.NoError(err)
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
}

func (suite *MySQLTestSuite) TestInit() {
	name, err := storage.ProbeMySQLIsolationVariableName(mySqlDSN[len(storage.MySQLPrefix):])
	suite.NoError(err)
	connection := suite.Database.(*SQLDatabase).client
	assertQuery(suite.T(), connection, fmt.Sprintf("SELECT @@%s", name), "READ-UNCOMMITTED")
	assertQuery(suite.T(), connection, "SELECT @@sql_mode", "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION")
}

func TestMySQL(t *testing.T) {
	suite.Run(t, new(MySQLTestSuite))
}

type PostgresTestSuite struct {
	baseTestSuite
}

func (suite *PostgresTestSuite) SetupSuite() {
	var err error
	// create database
	databaseComm, err := sql.Open("postgres", postgresDSN+"?sslmode=disable")
	suite.NoError(err)
	const dbName = "gorse_data_test"
	_, err = databaseComm.Exec("DROP DATABASE IF EXISTS " + dbName)
	suite.NoError(err)
	_, err = databaseComm.Exec("CREATE DATABASE " + dbName)
	suite.NoError(err)
	err = databaseComm.Close()
	suite.NoError(err)
	// connect database
	suite.Database, err = Open(postgresDSN+strings.ToLower(dbName)+"?sslmode=disable", "gorse_")
	suite.NoError(err)
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
}

func TestPostgres(t *testing.T) {
	suite.Run(t, new(PostgresTestSuite))
}

type ClickHouseTestSuite struct {
	baseTestSuite
}

func (suite *ClickHouseTestSuite) SetupSuite() {
	var err error
	// create database
	databaseComm, err := sql.Open("chhttp", "http://"+clickhouseDSN[len(storage.ClickhousePrefix):])
	suite.NoError(err)
	const dbName = "gorse_data_test"
	_, err = databaseComm.Exec("DROP DATABASE IF EXISTS " + dbName)
	suite.NoError(err)
	_, err = databaseComm.Exec("CREATE DATABASE " + dbName)
	suite.NoError(err)
	err = databaseComm.Close()
	suite.NoError(err)
	// connect database
	suite.Database, err = Open(clickhouseDSN+dbName+"?mutations_sync=2", "gorse_")
	suite.NoError(err)
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
}

func TestClickHouse(t *testing.T) {
	suite.Run(t, new(ClickHouseTestSuite))
}

type SQLiteTestSuite struct {
	baseTestSuite
}

func (suite *SQLiteTestSuite) SetupSuite() {
	var err error
	// create database
	path := fmt.Sprintf("sqlite://%s/sqlite.db", suite.T().TempDir())
	suite.Database, err = Open(path, "gorse_")
	suite.NoError(err)
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
}

func (suite *SQLiteTestSuite) TearDownSuite() {
	suite.NoError(suite.Database.Close())
}

func TestSQLite(t *testing.T) {
	suite.Run(t, new(SQLiteTestSuite))
}

func assertQuery(t *testing.T, connection *sql.DB, sql string, expected string) {
	rows, err := connection.Query(sql)
	assert.NoError(t, err)
	assert.True(t, rows.Next())
	var result string
	err = rows.Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}
