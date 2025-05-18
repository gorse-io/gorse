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
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
	"github.com/lib/pq"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"modernc.org/sqlite"
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
		if args[0] == nil || args[1] == nil {
			return nil, nil
		}
		j1, err := parse(args[0])
		if err != nil {
			return nil, err
		}
		j2, err := parse(args[1])
		if err != nil {
			return nil, err
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
	Collection string `gorm:"primaryKey"`
	Subset     string `gorm:"primaryKey"`
	Id         string `gorm:"primaryKey"`
	IsHidden   bool
	Categories pq.StringArray `gorm:"type:text[]"`
	Score      float64
	Timestamp  time.Time
}

type SQLDocument struct {
	Collection string `gorm:"primaryKey"`
	Subset     string `gorm:"primaryKey"`
	Id         string `gorm:"primaryKey"`
	IsHidden   bool
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
	err := db.gormDB.AutoMigrate(&SQLValue{}, &SQLSet{}, &SQLSortedSet{}, &Message{}, &TimeSeriesPoint{})
	if err != nil {
		return errors.Trace(err)
	}
	switch db.driver {
	case Postgres:
		err = db.gormDB.AutoMigrate(&PostgresDocument{})
		if err != nil {
			return errors.Trace(err)
		}
		// create extension btree_gin
		err = db.gormDB.Exec("CREATE EXTENSION IF NOT EXISTS btree_gin").Error
		if err != nil {
			return errors.Trace(err)
		}
		// create index
		err = db.gormDB.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_collection_subset_categories ON %s USING GIN (collection, subset, categories)", db.DocumentTable())).Error
		if err != nil {
			return errors.Trace(err)
		}
		err = db.gormDB.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_collection_id ON %s (collection, id)", db.DocumentTable())).Error
		if err != nil {
			return errors.Trace(err)
		}
	case MySQL:
		err = db.gormDB.AutoMigrate(&SQLDocument{})
		if err != nil {
			return errors.Trace(err)
		}
		// create index
		err = db.gormDB.Exec(fmt.Sprintf("ALTER TABLE %s ADD INDEX idx_collection_subset_categories (collection, subset, (CAST(categories AS CHAR(255) ARRAY)))", db.DocumentTable())).Error
		if err != nil {
			if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1061 {
				// ignore duplicate index error
			} else {
				return errors.Trace(err)
			}
		}
		err = db.gormDB.Exec(fmt.Sprintf("ALTER TABLE %s ADD INDEX idx_collection_id (collection, id)", db.DocumentTable())).Error
		if err != nil {
			if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1061 {
				// ignore duplicate index error
				err = nil
			} else {
				return errors.Trace(err)
			}
		}
	case SQLite:
		err = db.gormDB.AutoMigrate(&SQLDocument{})
	}
	return errors.Trace(err)
}

func (db *SQLDatabase) Scan(work func(string) error) error {
	var (
		valuerRows *sql.Rows
		setRows    *sql.Rows
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
	return nil
}

func (db *SQLDatabase) Purge() error {
	tables := []any{SQLValue{}, SQLSet{}, SQLSortedSet{}, Message{}, SQLDocument{}}
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

func (db *SQLDatabase) AddScores(ctx context.Context, collection, subset string, documents []Score) error {
	var rows any
	switch db.driver {
	case Postgres:
		rows = lo.Map(documents, func(document Score, _ int) PostgresDocument {
			return PostgresDocument{
				Collection: collection,
				Subset:     subset,
				Id:         document.Id,
				Score:      document.Score,
				IsHidden:   document.IsHidden,
				Categories: document.Categories,
				Timestamp:  document.Timestamp,
			}
		})
	case SQLite, MySQL:
		rows = lo.Map(documents, func(document Score, _ int) SQLDocument {
			return SQLDocument{
				Collection: collection,
				Subset:     subset,
				Id:         document.Id,
				Score:      document.Score,
				IsHidden:   document.IsHidden,
				Categories: document.Categories,
				Timestamp:  document.Timestamp,
			}
		})
	}
	db.gormDB.WithContext(ctx).Table(db.DocumentTable()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "collection"}, {Name: "subset"}, {Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"score", "categories", "timestamp"}),
	}).Create(rows)
	return nil
}

func (db *SQLDatabase) SearchScores(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Score, error) {
	tx := db.gormDB.WithContext(ctx).
		Model(&PostgresDocument{}).
		Select("id, score, categories, timestamp").
		Where("collection = ? and subset = ? and is_hidden = false", collection, subset)
	if len(query) > 0 {
		switch db.driver {
		case Postgres:
			tx.Where("categories @> ?", pq.StringArray(query))
		case SQLite, MySQL:
			q, err := json.Marshal(query)
			if err != nil {
				return nil, errors.Trace(err)
			}
			tx.Where("JSON_CONTAINS(categories,?)", string(q))
		}
	}
	tx.Order("score desc").Offset(begin)
	if end != -1 {
		tx.Limit(end - begin)
	} else {
		tx.Limit(math.MaxInt64)
	}
	rows, err := tx.Rows()
	if err != nil {
		return nil, errors.Trace(err)
	}
	documents := make([]Score, 0, 10)
	for rows.Next() {
		switch db.driver {
		case Postgres:
			var document PostgresDocument
			if err = rows.Scan(&document.Id, &document.Score, &document.Categories, &document.Timestamp); err != nil {
				return nil, errors.Trace(err)
			}
			documents = append(documents, Score{
				Id:         document.Id,
				Score:      document.Score,
				Categories: document.Categories,
				Timestamp:  document.Timestamp,
			})
		case SQLite, MySQL:
			var document Score
			if err = db.gormDB.ScanRows(rows, &document); err != nil {
				return nil, errors.Trace(err)
			}
			document.Timestamp = document.Timestamp.In(time.UTC)
			documents = append(documents, document)
		}
	}
	return documents, nil
}

func (db *SQLDatabase) UpdateScores(ctx context.Context, collections []string, subset *string, id string, patch ScorePatch) error {
	if len(collections) == 0 {
		return nil
	}
	if patch.Score == nil && patch.IsHidden == nil && patch.Categories == nil {
		return nil
	}
	tx := db.gormDB.WithContext(ctx).
		Model(&PostgresDocument{}).
		Where("collection in (?) and id = ?", collections, id)
	if subset != nil {
		tx.Where("subset = ?", subset)
	}
	if patch.Score != nil {
		tx.Update("score", *patch.Score)
	}
	if patch.IsHidden != nil {
		tx.Update("is_hidden", *patch.IsHidden)
	}
	if patch.Categories != nil {
		switch db.driver {
		case Postgres:
			tx.Update("categories", pq.StringArray(patch.Categories))
		case SQLite, MySQL:
			q, err := json.Marshal(patch.Categories)
			if err != nil {
				return errors.Trace(err)
			}
			tx.Update("categories", string(q))
		}
	}
	return tx.Error
}

func (db *SQLDatabase) DeleteScores(ctx context.Context, collections []string, condition ScoreCondition) error {
	if err := condition.Check(); err != nil {
		return errors.Trace(err)
	}
	var builder strings.Builder
	builder.WriteString("collection in (?)")
	var args []any
	args = append(args, collections)
	if condition.Subset != nil {
		builder.WriteString(" and subset = ?")
		args = append(args, *condition.Subset)
	}
	if condition.Id != nil {
		builder.WriteString(" and id = ?")
		args = append(args, *condition.Id)
	}
	if condition.Before != nil {
		builder.WriteString(" and timestamp < ?")
		args = append(args, *condition.Before)
	}
	return db.gormDB.WithContext(ctx).Delete(&SQLDocument{}, append([]any{builder.String()}, args...)...).Error
}

func (db *SQLDatabase) ScanScores(ctx context.Context, callback func(collection, id, subset string, timestamp time.Time) error) error {
	rows, err := db.gormDB.WithContext(ctx).Table(db.DocumentTable()).Select("collection, id, subset, timestamp").Rows()
	if err != nil {
		return errors.Trace(err)
	}
	defer rows.Close()
	for rows.Next() {
		var collection, id, subset string
		var timestamp time.Time
		if err = rows.Scan(&collection, &id, &subset, &timestamp); err != nil {
			return errors.Trace(err)
		}
		if err = callback(collection, id, subset, timestamp); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (db *SQLDatabase) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	if len(points) == 0 {
		return nil
	}
	return db.gormDB.WithContext(ctx).Table(db.PointsTable()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "timestamp"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(points).Error
}

func (db *SQLDatabase) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time, duration time.Duration) ([]TimeSeriesPoint, error) {
	var points []TimeSeriesPoint
	if db.driver == Postgres {
		if err := db.gormDB.WithContext(ctx).
			Raw(fmt.Sprintf("SELECT name, bucket_timestamp AS timestamp, value FROM ("+
				"SELECT *, TO_TIMESTAMP((EXTRACT(epoch FROM timestamp)::int / ?) * ?) AS bucket_timestamp,"+
				"ROW_NUMBER() OVER (PARTITION BY (EXTRACT(epoch FROM timestamp)::int / ?) ORDER BY timestamp DESC) AS rn "+
				"FROM %s WHERE name = ? and timestamp >= ? and timestamp <= ?) AS t WHERE rn = 1",
				db.PointsTable()), int(duration.Seconds()), int(duration.Seconds()), int(duration.Seconds()), name, begin, end).
			Scan(&points).Error; err != nil {
			return nil, errors.Trace(err)
		}
	} else if db.driver == MySQL {
		if err := db.gormDB.WithContext(ctx).
			Raw(fmt.Sprintf("SELECT name, bucket_timestamp AS timestamp, value FROM("+
				"SELECT *, FROM_UNIXTIME(FLOOR(UNIX_TIMESTAMP(timestamp) / ?) * ?) AS bucket_timestamp,"+
				"ROW_NUMBER() OVER (PARTITION BY FLOOR(UNIX_TIMESTAMP(timestamp) / ?) ORDER BY timestamp DESC) AS rn "+
				"FROM %s WHERE name = ? and timestamp >= ? and timestamp <= ?) AS t WHERE rn = 1;",
				db.PointsTable()), int(duration.Seconds()), int(duration.Seconds()), int(duration.Seconds()), name, begin, end).
			Scan(&points).Error; err != nil {
			return nil, errors.Trace(err)
		}
	} else if db.driver == SQLite {
		rows, err := db.gormDB.WithContext(ctx).
			Raw(fmt.Sprintf("select name, bucket_timestamp as timestamp, value from ("+
				"select *, datetime(strftime('%%s', substr(timestamp, 0, 20)) / ? * ?, 'unixepoch') as bucket_timestamp,"+
				"row_number() over (partition by strftime('%%s', substr(timestamp, 0, 20)) / ? order by timestamp desc) as rn "+
				"from %s where name = ? and timestamp >= ? and timestamp <= ?) where rn = 1",
				db.PointsTable()), int(duration.Seconds()), int(duration.Seconds()), int(duration.Seconds()), name, begin, end).
			Rows()
		if err != nil {
			return nil, errors.Trace(err)
		}
		defer rows.Close()
		for rows.Next() {
			var point TimeSeriesPoint
			var timestamp string
			if err := rows.Scan(&point.Name, &timestamp, &point.Value); err != nil {
				return nil, errors.Trace(err)
			}
			point.Timestamp, err = dateparse.ParseAny(timestamp)
			if err != nil {
				return nil, errors.Trace(err)
			}
			points = append(points, point)
		}
	}
	return points, nil
}
