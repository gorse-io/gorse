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
	"github.com/chewxy/math32"
	_ "github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
	_ "github.com/lib/pq"
	"github.com/samber/lo"
	"github.com/scylladb/go-set/strset"
	"strings"
)

type SQLDriver int

const (
	MySQL SQLDriver = iota
	Postgres
)

type SQLDatabase struct {
	client *sql.DB
	driver SQLDriver
}

func (db *SQLDatabase) Close() error {
	return db.client.Close()
}

func (db *SQLDatabase) Init() error {
	switch db.driver {
	case Postgres:
		if _, err := db.client.Exec("CREATE TABLE IF NOT EXISTS values (" +
			"name VARCHAR(256) PRIMARY KEY, " +
			"value VARCHAR(256) NOT NULL" +
			")"); err != nil {
			return errors.Trace(err)
		}

		if _, err := db.client.Exec("CREATE TABLE IF NOT EXISTS sets (" +
			"name VARCHAR(256) NOT NULL," +
			"member VARCHAR(256) NOT NULL," +
			"PRIMARY KEY (name, member)" +
			")"); err != nil {
			return errors.Trace(err)
		}

		if _, err := db.client.Exec("CREATE TABLE IF NOT EXISTS sorted_sets (" +
			"name VARCHAR(256) NOT NULL," +
			"member VARCHAR(256) NOT NULL," +
			"score DOUBLE PRECISION NOT NULL," +
			"PRIMARY KEY (name, member)" +
			")"); err != nil {
			return errors.Trace(err)
		}
		if _, err := db.client.Exec("CREATE INDEX IF NOT EXISTS sorted_sets_index ON sorted_sets(name, score)"); err != nil {
			return errors.Trace(err)
		}
	case MySQL:
		if _, err := db.client.Exec("CREATE TABLE IF NOT EXISTS `values` (" +
			"name VARCHAR(256) PRIMARY KEY, " +
			"value VARCHAR(256) NOT NULL" +
			")"); err != nil {
			return errors.Trace(err)
		}

		if _, err := db.client.Exec("CREATE TABLE IF NOT EXISTS sets (" +
			"name VARCHAR(256) NOT NULL," +
			"member VARCHAR(256) NOT NULL," +
			"PRIMARY KEY (name, member)" +
			")"); err != nil {
			return errors.Trace(err)
		}

		if _, err := db.client.Exec("CREATE TABLE IF NOT EXISTS sorted_sets (" +
			"name VARCHAR(256) NOT NULL," +
			"member VARCHAR(256) NOT NULL," +
			"score DOUBLE PRECISION NOT NULL," +
			"PRIMARY KEY (name, member)," +
			"INDEX (name, score)" +
			")"); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (db *SQLDatabase) Scan(work func(string) error) error {
	var (
		valuerRows *sql.Rows
		setRows    *sql.Rows
		sortedRows *sql.Rows
		err        error
	)

	// scan values
	switch db.driver {
	case Postgres:
		valuerRows, err = db.client.Query("SELECT name FROM values")
	case MySQL:
		valuerRows, err = db.client.Query("SELECT name FROM `values`")
	}
	if err != nil {
		return errors.Trace(err)
	}
	defer valuerRows.Close()
	for valuerRows.Next() {
		var key string
		if err = valuerRows.Scan(&key); err != nil {
			return errors.Trace(err)
		}
		if err = work(key); err != nil {
			return errors.Trace(err)
		}
	}

	// scan sets
	setRows, err = db.client.Query("SELECT name FROM sets")
	if err != nil {
		return errors.Trace(err)
	}
	defer setRows.Close()
	var prevKey string
	for setRows.Next() {
		var key string
		if err = setRows.Scan(&key); err != nil {
			return errors.Trace(err)
		}
		if key != prevKey {
			if err = work(key); err != nil {
				return errors.Trace(err)
			}
			prevKey = key
		}
	}

	// scan sorted sets
	sortedRows, err = db.client.Query("SELECT name FROM sorted_sets")
	if err != nil {
		return errors.Trace(err)
	}
	defer sortedRows.Close()
	prevKey = ""
	for sortedRows.Next() {
		var key string
		if err = sortedRows.Scan(&key); err != nil {
			return errors.Trace(err)
		}
		if key != prevKey {
			if err = work(key); err != nil {
				return errors.Trace(err)
			}
			prevKey = key
		}
	}
	return nil
}

func (db *SQLDatabase) Set(values ...Value) error {
	if len(values) == 0 {
		return nil
	}
	var builder strings.Builder
	var args []interface{}
	switch db.driver {
	case Postgres:
		builder.WriteString("INSERT INTO values(name, value) VALUES ")
	case MySQL:
		builder.WriteString("INSERT INTO `values`(name, value) VALUES ")
	}
	valueSet := strset.New()
	for _, value := range values {
		if !valueSet.Has(value.name) {
			if len(args) > 0 {
				builder.WriteRune(',')
			}
			switch db.driver {
			case Postgres:
				builder.WriteString(fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
			case MySQL:
				builder.WriteString("(?,?)")
			}
			args = append(args, value.name, value.value)
			valueSet.Add(value.name)
		}
	}
	switch db.driver {
	case Postgres:
		builder.WriteString("ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value")
	case MySQL:
		builder.WriteString("ON DUPLICATE KEY UPDATE value = VALUES(value)")
	}
	_, err := db.client.Exec(builder.String(), args...)
	return errors.Trace(err)
}

func (db *SQLDatabase) Get(name string) *ReturnValue {
	var rs *sql.Rows
	var err error
	switch db.driver {
	case Postgres:
		rs, err = db.client.Query("SELECT value FROM values WHERE name = $1", name)
	case MySQL:
		rs, err = db.client.Query("SELECT value FROM `values` WHERE name = ?", name)
	}
	if err != nil {
		return &ReturnValue{err: errors.Trace(err)}
	}
	defer rs.Close()
	if rs.Next() {
		var value string
		err := rs.Scan(&value)
		if err != nil {
			return &ReturnValue{err: errors.Trace(err)}
		}
		return &ReturnValue{value: value}
	}
	return &ReturnValue{err: errors.Annotate(ErrObjectNotExist, name)}
}

func (db *SQLDatabase) Delete(name string) error {
	var err error
	switch db.driver {
	case Postgres:
		_, err = db.client.Exec("DELETE FROM values WHERE name = $1", name)
	case MySQL:
		_, err = db.client.Exec("DELETE FROM `values` WHERE name = ?", name)
	}
	return errors.Trace(err)
}

func (db *SQLDatabase) GetSet(key string) ([]string, error) {
	var rs *sql.Rows
	var err error
	switch db.driver {
	case Postgres:
		rs, err = db.client.Query("SELECT member FROM sets WHERE name = $1", key)
	case MySQL:
		rs, err = db.client.Query("SELECT member FROM sets WHERE name = ?", key)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rs.Close()
	var members []string
	for rs.Next() {
		var member string
		if err = rs.Scan(&member); err != nil {
			return nil, errors.Trace(err)
		}
		members = append(members, member)
	}
	return members, nil
}

func (db *SQLDatabase) SetSet(key string, members ...string) error {
	txn, err := db.client.Begin()
	if err != nil {
		return errors.Trace(err)
	}
	switch db.driver {
	case Postgres:
		_, err = txn.Exec("DELETE FROM sets WHERE name = $1", key)
	case MySQL:
		_, err = txn.Exec("DELETE FROM sets WHERE name = ?", key)
	}
	if err != nil {
		return errors.Trace(err)
	}
	if len(members) > 0 {
		var args []interface{}
		var builder strings.Builder
		switch db.driver {
		case Postgres:
			builder.WriteString("INSERT INTO sets (name, member) VALUES ")
		case MySQL:
			builder.WriteString("INSERT IGNORE sets (name, member) VALUES ")
		}
		for i, member := range members {
			if i > 0 {
				builder.WriteRune(',')
			}
			switch db.driver {
			case Postgres:
				builder.WriteString(fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
			case MySQL:
				builder.WriteString("(?,?)")
			}
			args = append(args, key, member)
		}
		if db.driver == Postgres {
			builder.WriteString("ON CONFLICT (name, member) DO NOTHING")
		}
		if _, err = txn.Exec(builder.String(), args...); err != nil {
			return errors.Trace(err)
		}
	}
	return txn.Commit()
}

func (db *SQLDatabase) AddSet(key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	var args []interface{}
	var builder strings.Builder
	switch db.driver {
	case Postgres:
		builder.WriteString("INSERT INTO sets (name, member) VALUES ")
	case MySQL:
		builder.WriteString("INSERT IGNORE sets (name, member) VALUES ")
	}
	for i, member := range members {
		if i > 0 {
			builder.WriteRune(',')
		}
		switch db.driver {
		case Postgres:
			builder.WriteString(fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
		case MySQL:
			builder.WriteString("(?,?)")
		}
		args = append(args, key, member)
	}
	if db.driver == Postgres {
		builder.WriteString("ON CONFLICT (name, member) DO NOTHING")
	}
	if _, err := db.client.Exec(builder.String(), args...); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (db *SQLDatabase) RemSet(key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	var args []interface{}
	var builder strings.Builder
	builder.WriteString("DELETE FROM sets WHERE (name, member) IN (")
	for i, member := range members {
		if i > 0 {
			builder.WriteRune(',')
		}
		switch db.driver {
		case Postgres:
			builder.WriteString(fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
		case MySQL:
			builder.WriteString("(?,?)")
		}
		args = append(args, key, member)
	}
	builder.WriteString(")")
	if _, err := db.client.Exec(builder.String(), args...); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (db *SQLDatabase) AddSorted(sortedSets ...SortedSet) error {
	var args []interface{}
	var builder strings.Builder
	builder.WriteString("INSERT INTO sorted_sets (name, member, score) VALUES ")
	memberSets := make(map[lo.Tuple2[string, string]]struct{})
	for _, sortedSet := range sortedSets {
		for _, member := range sortedSet.scores {
			if _, exist := memberSets[lo.Tuple2[string, string]{sortedSet.name, member.Id}]; !exist {
				if len(args) > 0 {
					builder.WriteRune(',')
				}
				switch db.driver {
				case Postgres:
					builder.WriteString(fmt.Sprintf("($%d,$%d,$%d)", len(args)+1, len(args)+2, len(args)+3))
				case MySQL:
					builder.WriteString("(?,?,?)")
				}
				args = append(args, sortedSet.name, member.Id, member.Score)
				memberSets[lo.Tuple2[string, string]{sortedSet.name, member.Id}] = struct{}{}
			}
		}
	}
	switch db.driver {
	case Postgres:
		builder.WriteString("ON CONFLICT (name, member) DO UPDATE SET score = EXCLUDED.score")
	case MySQL:
		builder.WriteString("ON DUPLICATE KEY UPDATE score = VALUES(score)")
	}
	if len(args) == 0 {
		return nil
	}
	if _, err := db.client.Exec(builder.String(), args...); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (db *SQLDatabase) GetSorted(key string, begin, end int) ([]Scored, error) {
	var rs *sql.Rows
	var err error
	if end < begin {
		switch db.driver {
		case Postgres:
			rs, err = db.client.Query("SELECT member, score FROM sorted_sets WHERE name = $1 ORDER BY score DESC OFFSET $2", key, begin)
		case MySQL:
			rs, err = db.client.Query("SELECT member, score FROM sorted_sets WHERE name = ? ORDER BY score DESC LIMIT ?, ?", key, begin, math32.MaxInt64)
		}
	} else {
		switch db.driver {
		case Postgres:
			rs, err = db.client.Query("SELECT member, score FROM sorted_sets WHERE name = $1 ORDER BY score DESC OFFSET $2 LIMIT $3", key, begin, end-begin+1)
		case MySQL:
			rs, err = db.client.Query("SELECT member, score FROM sorted_sets WHERE name = ? ORDER BY score DESC LIMIT ?, ?", key, begin, end-begin+1)
		}
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rs.Close()
	var members []Scored
	for rs.Next() {
		var member Scored
		if err = rs.Scan(&member.Id, &member.Score); err != nil {
			return nil, errors.Trace(err)
		}
		members = append(members, member)
	}
	return members, nil
}

func (db *SQLDatabase) GetSortedByScore(key string, begin, end float64) ([]Scored, error) {
	var rs *sql.Rows
	var err error
	switch db.driver {
	case Postgres:
		rs, err = db.client.Query("SELECT member, score FROM sorted_sets WHERE name = $1 AND $2 <= score AND score <= $3 ORDER BY score", key, begin, end)
	case MySQL:
		rs, err = db.client.Query("SELECT member, score FROM sorted_sets WHERE name = ? AND ? <= score AND score <= ? ORDER BY score", key, begin, end)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rs.Close()
	var members []Scored
	for rs.Next() {
		var member Scored
		if err = rs.Scan(&member.Id, &member.Score); err != nil {
			return nil, errors.Trace(err)
		}
		members = append(members, member)
	}
	return members, nil
}

func (db *SQLDatabase) RemSortedByScore(key string, begin, end float64) error {
	var err error
	switch db.driver {
	case Postgres:
		_, err = db.client.Exec("DELETE FROM sorted_sets WHERE name = $1 AND $2 <= score AND score <= $3", key, begin, end)
	case MySQL:
		_, err = db.client.Exec("DELETE FROM sorted_sets WHERE name = ? AND ? <= score AND score <= ?", key, begin, end)
	}
	return errors.Trace(err)
}

func (db *SQLDatabase) SetSorted(key string, scores []Scored) error {
	txn, err := db.client.Begin()
	if err != nil {
		return errors.Trace(err)
	}
	switch db.driver {
	case Postgres:
		_, err = txn.Exec("DELETE FROM sorted_sets WHERE name = $1", key)
	case MySQL:
		_, err = txn.Exec("DELETE FROM sorted_sets WHERE name = ?", key)
	}
	if err != nil {
		return errors.Trace(err)
	}
	if len(scores) > 0 {
		var args []interface{}
		var builder strings.Builder
		builder.WriteString("INSERT INTO sorted_sets (name, member, score) VALUES ")
		memberSets := make(map[lo.Tuple2[string, string]]struct{})
		for _, member := range scores {
			if _, exist := memberSets[lo.Tuple2[string, string]{key, member.Id}]; !exist {
				if len(args) > 0 {
					builder.WriteRune(',')
				}
				switch db.driver {
				case Postgres:
					builder.WriteString(fmt.Sprintf("($%d,$%d,$%d)", len(args)+1, len(args)+2, len(args)+3))
				case MySQL:
					builder.WriteString("(?,?,?)")
				}
				args = append(args, key, member.Id, member.Score)
				memberSets[lo.Tuple2[string, string]{key, member.Id}] = struct{}{}
			}
		}
		switch db.driver {
		case Postgres:
			builder.WriteString("ON CONFLICT (name, member) DO UPDATE SET score = EXCLUDED.score")
		case MySQL:
			builder.WriteString("ON DUPLICATE KEY UPDATE score = VALUES(score)")
		}
		if _, err = txn.Exec(builder.String(), args...); err != nil {
			return errors.Trace(err)
		}
	}
	return txn.Commit()
}

func (db *SQLDatabase) RemSorted(members ...SetMember) error {
	if len(members) == 0 {
		return nil
	}
	var args []interface{}
	var builder strings.Builder
	builder.WriteString("DELETE FROM sorted_sets WHERE (name, member) IN (")
	for i, member := range members {
		if i > 0 {
			builder.WriteRune(',')
		}
		switch db.driver {
		case Postgres:
			builder.WriteString(fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
		case MySQL:
			builder.WriteString("(?,?)")
		}
		args = append(args, member.name, member.member)
	}
	builder.WriteString(")")
	if _, err := db.client.Exec(builder.String(), args...); err != nil {
		return errors.Trace(err)
	}
	return nil
}
