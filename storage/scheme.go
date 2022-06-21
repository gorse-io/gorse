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

package storage

import (
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
)

const (
	MySQLPrefix      = "mysql://"
	MongoPrefix      = "mongodb://"
	MongoSrvPrefix   = "mongodb+srv://"
	PostgresPrefix   = "postgres://"
	ClickhousePrefix = "clickhouse://"
	SQLitePrefix     = "sqlite://"
	RedisPrefix      = "redis://"
	OraclePrefix     = "oracle://"
)

func AppendMySQLParams(dsn string, params map[string]string) (string, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", errors.Trace(err)
	}
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	for key, value := range params {
		if _, exist := cfg.Params[key]; !exist {
			cfg.Params[key] = value
		}
	}
	return cfg.FormatDSN(), nil
}

func ProbeMySQLIsolationVariableName(dsn string) (string, error) {
	connection, err := sql.Open("mysql", dsn)
	if err != nil {
		return "", errors.Trace(err)
	}
	defer connection.Close()
	rows, err := connection.Query("SHOW VARIABLES LIKE '%isolation%'")
	if err != nil {
		return "", errors.Trace(err)
	}
	defer rows.Close()
	var name, value string
	if rows.Next() {
		if err = rows.Scan(&name, &value); err != nil {
			return "", errors.Trace(err)
		}
	}
	return name, nil
}
