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
			"name VARCHAR(128) PRIMARY KEY, " +
			"value VARCHAR(128) NOT NULL" +
			")"); err != nil {
			return errors.Trace(err)
		}

		if _, err := db.client.Exec("CREATE TABLE IF NOT EXISTS sets (" +
			"name VARCHAR(128) NOT NULL," +
			"member VARCHAR(128) NOT NULL," +
			"PRIMARY KEY (name, member)" +
			")"); err != nil {
			return errors.Trace(err)
		}

		if _, err := db.client.Exec("CREATE TABLE IF NOT EXISTS sorted_sets (" +
			"name VARCHAR(128) NOT NULL," +
			"member VARCHAR(128) NOT NULL," +
			"score DOUBLE PRECISION NOT NULL," +
			"PRIMARY KEY (name, member)" +
			")"); err != nil {
			return errors.Trace(err)
		}
		if _, err := db.client.Exec("CREATE INDEX IF NOT EXISTS sorted_sets_index ON sorted_sets(name, score)"); err != nil {
			return errors.Trace(err)
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
	builder.WriteString("INSERT INTO values(name, value) VALUES ")
	for i, value := range values {
		if i > 0 {
			builder.WriteRune(',')
		}
		builder.WriteString(fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
		args = append(args, value.name, value.value)
	}
	builder.WriteString("ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value")
	_, err := db.client.Exec(builder.String(), args...)
	return errors.Trace(err)
}

func (db *SQLDatabase) Get(name string) *ReturnValue {
	rs, err := db.client.Query("SELECT value FROM values WHERE name = $1", name)
	if err != nil {
		return &ReturnValue{err: err}
	}
	defer rs.Close()
	if rs.Next() {
		var value string
		err := rs.Scan(&value)
		if err != nil {
			return &ReturnValue{err: err}
		}
		return &ReturnValue{value: value}
	}
	return &ReturnValue{err: errors.Annotate(ErrObjectNotExist, name)}
}

func (db *SQLDatabase) Delete(name string) error {
	_, err := db.client.Exec("DELETE FROM values WHERE name = $1", name)
	return errors.Trace(err)
}

func (db *SQLDatabase) Exists(names ...string) ([]int, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var builder strings.Builder
	var args []interface{}
	builder.WriteString("SELECT name FROM values WHERE name IN (")
	for i, name := range names {
		if i > 0 {
			builder.WriteRune(',')
		}
		builder.WriteString(fmt.Sprintf("$%d", len(args)+1))
		args = append(args, name)
	}
	builder.WriteString(")")
	rs, err := db.client.Query(builder.String(), args...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	nameSet := strset.New()
	for rs.Next() {
		var name string
		if err = rs.Scan(&name); err != nil {
			return nil, errors.Trace(err)
		}
		nameSet.Add(name)
	}
	return lo.Map(names, func(name string, _ int) int {
		if nameSet.Has(name) {
			return 1
		} else {
			return 0
		}
	}), nil
}

func (db *SQLDatabase) GetSet(key string) ([]string, error) {
	rs, err := db.client.Query("SELECT member FROM sets WHERE name = $1", key)
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
	if _, err = txn.Exec("DELETE FROM sets WHERE name = $1", key); err != nil {
		return errors.Trace(err)
	}
	if len(members) > 0 {
		var args []interface{}
		var builder strings.Builder
		builder.WriteString("INSERT INTO sets (name, member) VALUES ")
		for i, member := range members {
			if i > 0 {
				builder.WriteRune(',')
			}
			builder.WriteString(fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
			args = append(args, key, member)
		}
		builder.WriteString("ON CONFLICT (name, member) DO NOTHING")
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
	builder.WriteString("INSERT INTO sets (name, member) VALUES ")
	for i, member := range members {
		if i > 0 {
			builder.WriteRune(',')
		}
		builder.WriteString(fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
		args = append(args, key, member)
	}
	builder.WriteString("ON CONFLICT (name, member) DO NOTHING")
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
		builder.WriteString(fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
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
	for _, sortedSet := range sortedSets {
		for _, member := range sortedSet.scores {
			if len(args) > 0 {
				builder.WriteRune(',')
			}
			builder.WriteString(fmt.Sprintf("($%d,$%d,$%d)", len(args)+1, len(args)+2, len(args)+3))
			args = append(args, sortedSet.name, member.Id, member.Score)
		}
	}
	builder.WriteString("ON CONFLICT (name, member) DO UPDATE SET score = EXCLUDED.score")
	if len(args) == 0 {
		return nil
	}
	if _, err := db.client.Exec(builder.String(), args...); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (db *SQLDatabase) GetSortedScores(members ...SetMember) ([]float64, error) {
	if len(members) == 0 {
		return nil, nil
	}
	var args []interface{}
	var builder strings.Builder
	builder.WriteString("SELECT name, member, score FROM sorted_sets WHERE (name, member) IN (")
	for _, member := range members {
		if len(args) > 0 {
			builder.WriteRune(',')
		}
		builder.WriteString(fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
		args = append(args, member.name, member.member)
	}
	builder.WriteString(")")
	rs, err := db.client.Query(builder.String(), args...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	memberMap := make(map[SetMember]float64)
	for rs.Next() {
		var member SetMember
		var score float64
		if err = rs.Scan(&member.name, &member.member, &score); err != nil {
			return nil, errors.Trace(err)
		}
		memberMap[member] = score
	}
	return lo.Map(members, func(member SetMember, _ int) float64 {
		return memberMap[member]
	}), nil
}

func (db *SQLDatabase) GetSorted(key string, begin, end int) ([]Scored, error) {
	var rs *sql.Rows
	var err error
	if end < begin {
		rs, err = db.client.Query("SELECT member, score FROM sorted_sets WHERE name = $1 ORDER BY score DESC OFFSET $2", key, begin)
	} else {
		rs, err = db.client.Query("SELECT member, score FROM sorted_sets WHERE name = $1 ORDER BY score DESC OFFSET $2 LIMIT $3", key, begin, end-begin+1)
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
	rs, err := db.client.Query("SELECT member, score FROM sorted_sets WHERE name = $1 AND $2 <= score AND score <= $3 ORDER BY score",
		key, begin, end)
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
	_, err := db.client.Exec("DELETE FROM sorted_sets WHERE name = $1 AND $2 <= score AND score <= $3", key, begin, end)
	return errors.Trace(err)
}

func (db *SQLDatabase) SetSorted(key string, scores []Scored) error {
	txn, err := db.client.Begin()
	if err != nil {
		return errors.Trace(err)
	}
	if _, err = txn.Exec("DELETE FROM sorted_sets WHERE name = $1", key); err != nil {
		return errors.Trace(err)
	}
	if len(scores) > 0 {
		var args []interface{}
		var builder strings.Builder
		builder.WriteString("INSERT INTO sorted_sets (name, member, score) VALUES ")
		for i, member := range scores {
			if i > 0 {
				builder.WriteRune(',')
			}
			builder.WriteString(fmt.Sprintf("($%d,$%d,$%d)", len(args)+1, len(args)+2, len(args)+3))
			args = append(args, key, member.Id, member.Score)
		}
		builder.WriteString("ON CONFLICT (name, member) DO UPDATE SET score = EXCLUDED.score")
		if _, err = txn.Exec(builder.String(), args...); err != nil {
			return errors.Trace(err)
		}
	}
	return txn.Commit()
}

func (db *SQLDatabase) RemSorted(key, member string) error {
	_, err := db.client.Exec("DELETE FROM sorted_sets WHERE (name, member) IN (($1, $2))", key, member)
	return errors.Trace(err)
}
