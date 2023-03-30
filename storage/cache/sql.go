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
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"math"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	_ "github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"modernc.org/sqlite"
	_ "modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("json_contains", 2, func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		parse := func(arg driver.Value) (j []any, err error) {
			var data []byte
			switch argTyped := arg.(type) {
			case string:
				data = []byte(argTyped)
			case []byte:
				data = argTyped
			default:
				return nil, errors.Errorf("unsupported type %T", arg)
			}
			err = json.Unmarshal(data, &j)
			return
		}
		j1, err := parse(args[0])
		if err != nil {
			return nil, err
		}
		j2, err := parse(args[1])
		if err != nil {
			return nil, err
		}
		if j2 == nil {
			return false, nil
		}
		elements := make(map[any]struct{}, len(j1))
		for _, e := range j1 {
			elements[e] = struct{}{}
		}
		for _, e := range j2 {
			if _, ok := elements[e]; !ok {
				return false, nil
			}
		}
		return true, nil
	})
}

type SQLDriver int

const (
	MySQL SQLDriver = iota
	Postgres
	SQLite
)

type SQLValue struct {
	Name  string `gorm:"type:varchar(256);primaryKey"`
	Value string `gorm:"type:varchar(256);not null"`
}

type SQLSet struct {
	Name   string `gorm:"type:varchar(256);primaryKey"`
	Member string `gorm:"type:varchar(256);primaryKey"`
}

type SQLSortedSet struct {
	Name   string  `gorm:"type:varchar(256);primaryKey;index:name"`
	Member string  `gorm:"type:varchar(256);primaryKey"`
	Score  float64 `gorm:"type:double precision;not null;index:name"`
}

type Message struct {
	Name      string `gorm:"primaryKey;index:timestamp"`
	Value     string `gorm:"primaryKey"`
	Timestamp int64  `gorm:"index:timestamp"`
}

type PostgresDocument struct {
	Name       string         `gorm:"primaryKey"`
	Value      string         `gorm:"primaryKey"`
	Categories pq.StringArray `gorm:"type:text[]"`
	Score      float64
	Timestamp  time.Time
}

type SQLDocument struct {
	Name       string   `gorm:"primaryKey"`
	Value      string   `gorm:"primaryKey"`
	Categories []string `gorm:"type:text;serializer:json"`
	Score      float64
	Timestamp  time.Time
}

type SQLDatabase struct {
	storage.TablePrefix
	gormDB *gorm.DB
	client *sql.DB
	driver SQLDriver
}

func (db *SQLDatabase) Close() error {
	return db.client.Close()
}

func (db *SQLDatabase) Ping() error {
	return db.client.Ping()
}

func (db *SQLDatabase) Init() error {
	err := db.gormDB.AutoMigrate(&SQLValue{}, &SQLSet{}, &SQLSortedSet{}, &Message{})
	if err != nil {
		return errors.Trace(err)
	}
	switch db.driver {
	case Postgres:
		err = db.gormDB.AutoMigrate(&PostgresDocument{})
	case SQLite, MySQL:
		err = db.gormDB.AutoMigrate(&SQLDocument{})
	}
	return errors.Trace(err)
}

func (db *SQLDatabase) Scan(work func(string) error) error {
	var (
		valuerRows *sql.Rows
		setRows    *sql.Rows
		sortedRows *sql.Rows
		err        error
	)

	// scan values
	valuerRows, err = db.gormDB.Table(db.ValuesTable()).Select("name").Rows()
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
	setRows, err = db.gormDB.Table(db.SetsTable()).Select("name").Rows()
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
	sortedRows, err = db.gormDB.Table(db.SortedSetsTable()).Select("name").Rows()
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

func (db *SQLDatabase) Purge() error {
	tables := []any{SQLValue{}, SQLSet{}, SQLSortedSet{}, Message{}}
	for _, table := range tables {
		err := db.gormDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&table).Error
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (db *SQLDatabase) Set(ctx context.Context, values ...Value) error {
	if len(values) == 0 {
		return nil
	}
	valueSet := mapset.NewSet[string]()
	rows := make([]SQLValue, 0, len(values))
	for _, value := range values {
		if !valueSet.Contains(value.name) {
			rows = append(rows, SQLValue{
				Name:  value.name,
				Value: value.value,
			})
			valueSet.Add(value.name)
		}
	}
	err := db.gormDB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(rows).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) Get(ctx context.Context, name string) *ReturnValue {
	rs, err := db.gormDB.WithContext(ctx).Table(db.ValuesTable()).Where("name = ?", name).Select("value").Rows()
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

func (db *SQLDatabase) Delete(ctx context.Context, name string) error {
	err := db.gormDB.WithContext(ctx).Delete(&SQLValue{Name: name}).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) GetSet(ctx context.Context, key string) ([]string, error) {
	rs, err := db.gormDB.WithContext(ctx).Table(db.SetsTable()).Select("member").Where("name = ?", key).Rows()
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

func (db *SQLDatabase) SetSet(ctx context.Context, key string, members ...string) error {
	tx := db.gormDB.WithContext(ctx)
	err := tx.Delete(&SQLSet{}, "name = ?", key).Error
	if err != nil {
		return errors.Trace(err)
	}
	if len(members) == 0 {
		return nil
	}
	rows := lo.Map(members, func(member string, _ int) SQLSet {
		return SQLSet{
			Name:   key,
			Member: member,
		}
	})
	err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(rows).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) AddSet(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	rows := lo.Map(members, func(member string, _ int) SQLSet {
		return SQLSet{
			Name:   key,
			Member: member,
		}
	})
	err := db.gormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(rows).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) RemSet(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	rows := lo.Map(members, func(member string, _ int) SQLSet {
		return SQLSet{
			Name:   key,
			Member: member,
		}
	})
	err := db.gormDB.WithContext(ctx).Delete(rows).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) AddSorted(ctx context.Context, sortedSets ...SortedSet) error {
	rows := make([]SQLSortedSet, 0, len(sortedSets))
	memberSets := make(map[lo.Tuple2[string, string]]struct{})
	for _, sortedSet := range sortedSets {
		for _, member := range sortedSet.scores {
			if _, exist := memberSets[lo.Tuple2[string, string]{sortedSet.name, member.Id}]; !exist {
				rows = append(rows, SQLSortedSet{
					Name:   sortedSet.name,
					Member: member.Id,
					Score:  member.Score,
				})
				memberSets[lo.Tuple2[string, string]{sortedSet.name, member.Id}] = struct{}{}
			}
		}
	}
	if len(rows) == 0 {
		return nil
	}
	if err := db.gormDB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "member"}},
		DoUpdates: clause.AssignmentColumns([]string{"score"}),
	}).Create(rows).Error; err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (db *SQLDatabase) GetSorted(ctx context.Context, key string, begin, end int) ([]Scored, error) {
	tx := db.gormDB.WithContext(ctx).Table(db.SortedSetsTable()).
		Select("member, score").
		Where("name = ?", key).
		Order("score DESC")
	if end < begin {
		tx.Offset(begin).Limit(math.MaxInt64)
	} else {
		tx.Offset(begin).Limit(end - begin + 1)
	}
	rs, err := tx.Rows()
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

func (db *SQLDatabase) GetSortedByScore(ctx context.Context, key string, begin, end float64) ([]Scored, error) {
	rs, err := db.gormDB.WithContext(ctx).Table(db.SortedSetsTable()).
		Select("member, score").
		Where("name = ? AND score >= ? AND score <= ?", key, begin, end).
		Order("score").Rows()
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

func (db *SQLDatabase) RemSortedByScore(ctx context.Context, key string, begin, end float64) error {
	err := db.gormDB.WithContext(ctx).Delete(&SQLSortedSet{}, "name = ? AND ? <= score AND score <= ?", key, begin, end).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) SetSorted(ctx context.Context, key string, scores []Scored) error {
	tx := db.gormDB.WithContext(ctx)
	err := tx.Delete(&SQLSortedSet{}, "name = ?", key).Error
	if err != nil {
		return errors.Trace(err)
	}
	if len(scores) > 0 {
		memberSets := make(map[lo.Tuple2[string, string]]struct{})
		rows := make([]SQLSortedSet, 0, len(scores))
		for _, member := range scores {
			if _, exist := memberSets[lo.Tuple2[string, string]{key, member.Id}]; !exist {
				rows = append(rows, SQLSortedSet{
					Name:   key,
					Member: member.Id,
					Score:  member.Score,
				})
				memberSets[lo.Tuple2[string, string]{key, member.Id}] = struct{}{}
			}
		}
		err = tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}, {Name: "member"}},
			DoUpdates: clause.AssignmentColumns([]string{"score"}),
		}).Create(&rows).Error
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (db *SQLDatabase) RemSorted(ctx context.Context, members ...SetMember) error {
	if len(members) == 0 {
		return nil
	}
	rows := lo.Map(members, func(member SetMember, _ int) SQLSortedSet {
		return SQLSortedSet{
			Name:   member.name,
			Member: member.member,
		}
	})
	err := db.gormDB.WithContext(ctx).Delete(rows).Error
	return errors.Trace(err)
}

func (db *SQLDatabase) Push(ctx context.Context, name, value string) error {
	return db.gormDB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "value"}},
		DoUpdates: clause.AssignmentColumns([]string{"timestamp"}),
	}).Create(&Message{
		Name:      name,
		Value:     value,
		Timestamp: time.Now().UnixNano(),
	}).Error
}

func (db *SQLDatabase) Pop(ctx context.Context, name string) (string, error) {
	var message Message
	err := db.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := db.gormDB.Order("timestamp").First(&message, "name = ?", name).Error; err != nil {
			return err
		}
		if err := db.gormDB.Delete(&message).Error; err != nil {
			return err
		}
		return nil
	})
	if err == gorm.ErrRecordNotFound {
		return "", io.EOF
	}
	return message.Value, err
}

func (db *SQLDatabase) Remain(ctx context.Context, name string) (count int64, err error) {
	err = db.gormDB.WithContext(ctx).Model(&Message{}).Where("name = ?", name).Count(&count).Error
	return
}

func (db *SQLDatabase) AddDocuments(ctx context.Context, name string, documents ...Document) error {
	var rows any
	switch db.driver {
	case Postgres:
		rows = lo.Map(documents, func(document Document, _ int) PostgresDocument {
			return PostgresDocument{
				Name:       name,
				Value:      document.Value,
				Score:      document.Score,
				Categories: document.Categories,
				Timestamp:  document.Timestamp,
			}
		})
	case SQLite, MySQL:
		rows = lo.Map(documents, func(document Document, _ int) SQLDocument {
			return SQLDocument{
				Name:       name,
				Value:      document.Value,
				Score:      document.Score,
				Categories: document.Categories,
				Timestamp:  document.Timestamp,
			}
		})
	}
	db.gormDB.WithContext(ctx).Table(db.DocumentTable()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "value"}},
		DoUpdates: clause.AssignmentColumns([]string{"score", "categories", "timestamp"}),
	}).Create(rows)
	return nil
}

func (db *SQLDatabase) SearchDocuments(ctx context.Context, name string, query []string, begin, end int) ([]Document, error) {
	if len(query) == 0 {
		return nil, nil
	}
	tx := db.gormDB.WithContext(ctx).Model(&PostgresDocument{}).Select("value, score, categories, timestamp")
	switch db.driver {
	case Postgres:
		tx = tx.Where("name = ? and categories @> ?", name, pq.StringArray(query))
	case SQLite, MySQL:
		q, err := json.Marshal(query)
		if err != nil {
			return nil, errors.Trace(err)
		}
		tx = tx.Where("name = ? and JSON_CONTAINS(categories,?)", name, string(q))
	}
	tx = tx.Order("score desc").Offset(begin)
	if end != -1 {
		tx = tx.Limit(end - begin)
	} else {
		tx = tx.Limit(math.MaxInt64)
	}
	rows, err := tx.Rows()
	if err != nil {
		return nil, errors.Trace(err)
	}
	documents := make([]Document, 0, 10)
	for rows.Next() {
		switch db.driver {
		case Postgres:
			var document PostgresDocument
			if err = rows.Scan(&document.Value, &document.Score, &document.Categories, &document.Timestamp); err != nil {
				return nil, errors.Trace(err)
			}
			documents = append(documents, Document{
				Value:      document.Value,
				Score:      document.Score,
				Categories: document.Categories,
				Timestamp:  document.Timestamp,
			})
		case SQLite, MySQL:
			var document Document
			if err = db.gormDB.ScanRows(rows, &document); err != nil {
				return nil, errors.Trace(err)
			}
			documents = append(documents, document)
		}
	}
	return documents, nil
}

func (db *SQLDatabase) UpdateDocuments(ctx context.Context, names []string, value string, categories []string) error {
	if len(names) == 0 {
		return nil
	}
	tx := db.gormDB.WithContext(ctx).Model(&PostgresDocument{}).Where("name in (?) and value = ?", names, value)
	switch db.driver {
	case Postgres:
		tx = tx.Update("categories", pq.StringArray(categories))
	case SQLite, MySQL:
		q, err := json.Marshal(categories)
		if err != nil {
			return errors.Trace(err)
		}
		tx = tx.Update("categories", string(q))
	}
	return tx.Error
}

func (db *SQLDatabase) DeleteDocuments(ctx context.Context, name string, condition DocumentCondition) error {
	if err := condition.Check(); err != nil {
		return errors.Trace(err)
	}
	var builder strings.Builder
	builder.WriteString("name = ?")
	var args []any
	args = append(args, name)
	if condition.Value != nil {
		builder.WriteString(" and value = ?")
		args = append(args, *condition.Value)
	}
	if condition.Before != nil {
		builder.WriteString(" and timestamp < ?")
		args = append(args, *condition.Before)
	}
	return db.gormDB.WithContext(ctx).Delete(&SQLDocument{}, append([]any{builder.String()}, args...)...).Error
}
