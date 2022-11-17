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
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/zhenghaoz/gorse/storage"
)

var (
	mySqlDSN    string
	postgresDSN string
	oracleDSN   string
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
	oracleDSN = env("ORACLE_URI", "oracle://system:password@127.0.0.1:1521/XE")
}

type PostgresTestSuite struct {
	baseTestSuite
}

func (suite *PostgresTestSuite) SetupSuite() {
	var err error
	// create database
	databaseComm, err := sql.Open("postgres", postgresDSN+"?sslmode=disable")
	suite.NoError(err)
	const dbName = "gorse_cache_test"
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

type MySQLTestSuite struct {
	baseTestSuite
}

func (suite *MySQLTestSuite) SetupSuite() {
	// create database
	databaseComm, err := sql.Open("mysql", mySqlDSN[len(storage.MySQLPrefix):])
	suite.NoError(err)
	const dbName = "gorse_cache_test"
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
	err := suite.Database.Init()
	suite.NoError(err)

	name, err := storage.ProbeMySQLIsolationVariableName(mySqlDSN[len(storage.MySQLPrefix):])
	suite.NoError(err)
	connection := suite.Database.(*SQLDatabase).client
	assertQuery(suite.T(), connection, fmt.Sprintf("SELECT @@%s", name), "READ-UNCOMMITTED")
}

func TestMySQL(t *testing.T) {
	suite.Run(t, new(MySQLTestSuite))
}

type OracleTestSuite struct {
	baseTestSuite
}

func (suite *OracleTestSuite) SetupSuite() {
	var err error
	// create database
	databaseComm, err := sql.Open("oracle", oracleDSN)
	suite.NoError(err)
	const dbName = "GORSE_CACHE_TEST"
	rows, err := databaseComm.Query("select * from dba_users where username=:1", dbName)
	suite.NoError(err)
	if rows.Next() {
		// drop user if exists
		_, err = databaseComm.Exec(fmt.Sprintf("DROP USER %s CASCADE", dbName))
		suite.NoError(err)
	}
	err = rows.Close()
	suite.NoError(err)
	_, err = databaseComm.Exec(fmt.Sprintf("CREATE USER %s IDENTIFIED BY %s", dbName, dbName))
	suite.NoError(err)
	_, err = databaseComm.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES TO %s", dbName))
	suite.NoError(err)
	err = databaseComm.Close()
	suite.NoError(err)
	// connect database
	parsed, err := url.Parse(oracleDSN)
	suite.NoError(err)
	suite.Database, err = Open(fmt.Sprintf("oracle://%s:%s@%s/%s", dbName, dbName, parsed.Host, parsed.Path), "gorse_")
	suite.NoError(err)
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
}

func TestOracle(t *testing.T) {
	suite.Run(t, new(OracleTestSuite))
}

type SQLiteTestSuite struct {
	baseTestSuite
}

func (suite *SQLiteTestSuite) SetupSuite() {
	var err error
	// create database
	suite.Database, err = Open("sqlite://:memory:", "gorse_")
	suite.NoError(err)
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
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
