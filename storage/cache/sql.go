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
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/araddon/dateparse"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-sql-driver/mysql"
	"github.com/gorse-io/gorse/storage"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	semconv "go.opentelemetry.io/otel/semconv/v1.8.0"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	Register([]string{storage.PostgresPrefix, storage.PostgreSQLPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		database := new(SQLDatabase)
		database.driver = Postgres
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		option := storage.NewOptions(opts...)
		var err error
		if database.client, err = otelsql.Open("postgres", path,
			otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
			otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
		); err != nil {
			return nil, errors.WithStack(err)
		}
		storage.ApplySQLPool(database.client, option)
		database.gormDB, err = gorm.Open(postgres.New(postgres.Config{
			DriverName: "postgres",
			Conn:       database.client,
		}), storage.NewGORMConfig(tablePrefix))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return database, nil
	})
	Register([]string{storage.MySQLPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		name := path[len(storage.MySQLPrefix):]
		option := storage.NewOptions(opts...)
		// probe isolation variable name
		isolationVarName, err := storage.ProbeMySQLIsolationVariableName(name)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		// append parameters
		if name, err = storage.AppendMySQLParams(name, map[string]string{
			isolationVarName: fmt.Sprintf("'%s'", option.IsolationLevel),
			"parseTime":      "true",
		}); err != nil {
			return nil, errors.WithStack(err)
		}
		// connect to database
		database := new(SQLDatabase)
		database.driver = MySQL
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		if database.client, err = otelsql.Open("mysql", name,
			otelsql.WithAttributes(semconv.DBSystemMySQL),
			otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
		); err != nil {
			return nil, errors.WithStack(err)
		}
		storage.ApplySQLPool(database.client, option)
		database.gormDB, err = gorm.Open(gormmysql.New(gormmysql.Config{Conn: database.client}), storage.NewGORMConfig(tablePrefix))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return database, nil
	})
	Register([]string{storage.SQLitePrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		dataSourceName := path[len(storage.SQLitePrefix):]
		// append parameters
		var err error
		if dataSourceName, err = storage.AppendURLParams(dataSourceName, []lo.Tuple2[string, string]{
			{"_pragma", "busy_timeout(10000)"},
			{"_pragma", "journal_mode(wal)"},
		}); err != nil {
			return nil, errors.WithStack(err)
		}
		// connect to database
		database := new(SQLDatabase)
		database.driver = SQLite
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		if database.client, err = otelsql.Open("sqlite", dataSourceName,
			otelsql.WithAttributes(semconv.DBSystemSqlite),
			otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
		); err != nil {
			return nil, errors.WithStack(err)
		}
		database.gormDB, err = gorm.Open(sqlite.Dialector{Conn: database.client}, storage.NewGORMConfig(tablePrefix))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return database, nil
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
	err := db.gormDB.AutoMigrate(&SQLValue{}, &TimeSeriesPoint{})
	if err != nil {
		return errors.WithStack(err)
	}
	switch db.driver {
	case Postgres:
		err = db.gormDB.AutoMigrate(&PostgresDocument{})
		if err != nil {
			return errors.WithStack(err)
		}
		// create extension btree_gin
		err = db.gormDB.Exec("CREATE EXTENSION IF NOT EXISTS btree_gin").Error
		if err != nil {
			return errors.WithStack(err)
		}
		// create index
		err = db.gormDB.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_collection_subset_categories ON %s USING GIN (collection, subset, categories)", db.DocumentTable())).Error
		if err != nil {
			return errors.WithStack(err)
		}
		err = db.gormDB.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_collection_id ON %s (collection, id)", db.DocumentTable())).Error
		if err != nil {
			return errors.WithStack(err)
		}
	case MySQL:
		err = db.gormDB.AutoMigrate(&SQLDocument{})
		if err != nil {
			return errors.WithStack(err)
		}
		// detect MariaDB
		var isMariaDB bool
		var version string
		if err = db.gormDB.Raw("SELECT VERSION()").Scan(&version).Error; err == nil {
			isMariaDB = strings.Contains(version, "MariaDB")
		}
		// create index (skip categories index for MariaDB since it doesn't support CAST AS ARRAY)
		if !isMariaDB {
			err = db.gormDB.Exec(fmt.Sprintf("ALTER TABLE %s ADD INDEX idx_collection_subset_categories (collection, subset, (CAST(categories AS CHAR(255) ARRAY)))", db.DocumentTable())).Error
			if err != nil {
				if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1061 {
					// ignore duplicate index error
				} else {
					return errors.WithStack(err)
				}
			}
		}
		err = db.gormDB.Exec(fmt.Sprintf("ALTER TABLE %s ADD INDEX idx_collection_id (collection, id)", db.DocumentTable())).Error
		if err != nil {
			if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1061 {
				// ignore duplicate index error
				err = nil
			} else {
				return errors.WithStack(err)
			}
		}
	case SQLite:
		err = db.gormDB.AutoMigrate(&SQLDocument{})
	}
	if err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(db.initGeoIndexes())
}

func (db *SQLDatabase) Scan(work func(string) error) error {
	var (
		valuerRows *sql.Rows
		err        error
	)

	// scan values
	valuerRows, err = db.gormDB.Table(db.ValuesTable()).Select("name").Rows()
	if err != nil {
		return errors.WithStack(err)
	}
	defer valuerRows.Close()
	for valuerRows.Next() {
		var key string
		if err = valuerRows.Scan(&key); err != nil {
			return errors.WithStack(err)
		}
		if err = work(key); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (db *SQLDatabase) Purge() error {
	if err := db.purgeGeoIndexes(); err != nil {
		return errors.WithStack(err)
	}
	tables := []any{SQLValue{}, SQLDocument{}}
	for _, table := range tables {
		err := db.gormDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&table).Error
		if err != nil {
			return errors.WithStack(err)
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
	return errors.WithStack(err)
}

func (db *SQLDatabase) Get(ctx context.Context, name string) *ReturnValue {
	rs, err := db.gormDB.WithContext(ctx).Table(db.ValuesTable()).Where("name = ?", name).Select("value").Rows()
	if err != nil {
		return &ReturnValue{err: errors.WithStack(err), exists: false}
	}
	defer rs.Close()
	if rs.Next() {
		var value string
		err := rs.Scan(&value)
		if err != nil {
			return &ReturnValue{err: errors.WithStack(err), exists: false}
		}
		return &ReturnValue{value: value, exists: true}
	}
	return &ReturnValue{value: "", exists: false}
}

func (db *SQLDatabase) Delete(ctx context.Context, name string) error {
	err := db.gormDB.WithContext(ctx).Delete(&SQLValue{Name: name}).Error
	return errors.WithStack(err)
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
	err := db.gormDB.WithContext(ctx).Table(db.DocumentTable()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "collection"}, {Name: "subset"}, {Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"score", "categories", "timestamp"}),
	}).Create(rows).Error
	return errors.WithStack(err)
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
				return nil, errors.WithStack(err)
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
		return nil, errors.WithStack(err)
	}
	documents := make([]Score, 0, 10)
	for rows.Next() {
		switch db.driver {
		case Postgres:
			var document PostgresDocument
			if err = rows.Scan(&document.Id, &document.Score, &document.Categories, &document.Timestamp); err != nil {
				return nil, errors.WithStack(err)
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
				return nil, errors.WithStack(err)
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
				return errors.WithStack(err)
			}
			tx.Update("categories", string(q))
		}
	}
	return tx.Error
}

func (db *SQLDatabase) DeleteScores(ctx context.Context, collections []string, condition ScoreCondition) error {
	if err := condition.Check(); err != nil {
		return errors.WithStack(err)
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
		if db.driver == MySQL {
			// In MySQL, we need to truncate the time to milliseconds because MySQL will round the time to milliseconds.
			args = append(args, condition.Before.Truncate(time.Millisecond))
		} else {
			args = append(args, *condition.Before)
		}
	}
	return db.gormDB.WithContext(ctx).Delete(&SQLDocument{}, append([]any{builder.String()}, args...)...).Error
}

func (db *SQLDatabase) ScanScores(ctx context.Context, callback func(collection, id, subset string, timestamp time.Time) error) error {
	rows, err := db.gormDB.WithContext(ctx).Table(db.DocumentTable()).Select("collection, id, subset, timestamp").Rows()
	if err != nil {
		return errors.WithStack(err)
	}
	defer rows.Close()
	for rows.Next() {
		var collection, id, subset string
		var timestamp time.Time
		if err = rows.Scan(&collection, &id, &subset, &timestamp); err != nil {
			return errors.WithStack(err)
		}
		if err = callback(collection, id, subset, timestamp); err != nil {
			return errors.WithStack(err)
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
	points := []TimeSeriesPoint{}
	switch db.driver {
	case Postgres:
		if err := db.gormDB.WithContext(ctx).
			Raw(fmt.Sprintf("SELECT name, bucket_timestamp AS timestamp, value FROM ("+
				"SELECT *, TO_TIMESTAMP((EXTRACT(epoch FROM timestamp)::int / ?) * ?) AS bucket_timestamp,"+
				"ROW_NUMBER() OVER (PARTITION BY (EXTRACT(epoch FROM timestamp)::int / ?) ORDER BY timestamp DESC) AS rn "+
				"FROM %s WHERE name = ? and timestamp >= ? and timestamp <= ?) AS t WHERE rn = 1",
				db.PointsTable()), int(duration.Seconds()), int(duration.Seconds()), int(duration.Seconds()), name, begin, end).
			Scan(&points).Error; err != nil {
			return nil, errors.WithStack(err)
		}
	case MySQL:
		if err := db.gormDB.WithContext(ctx).
			Raw(fmt.Sprintf("SELECT name, bucket_timestamp AS timestamp, value FROM("+
				"SELECT *, FROM_UNIXTIME(FLOOR(UNIX_TIMESTAMP(timestamp) / ?) * ?) AS bucket_timestamp,"+
				"ROW_NUMBER() OVER (PARTITION BY FLOOR(UNIX_TIMESTAMP(timestamp) / ?) ORDER BY timestamp DESC) AS rn "+
				"FROM %s WHERE name = ? and timestamp >= ? and timestamp <= ?) AS t WHERE rn = 1;",
				db.PointsTable()), int(duration.Seconds()), int(duration.Seconds()), int(duration.Seconds()), name, begin, end).
			Scan(&points).Error; err != nil {
			return nil, errors.WithStack(err)
		}
	case SQLite:
		rows, err := db.gormDB.WithContext(ctx).
			Raw(fmt.Sprintf("select name, bucket_timestamp as timestamp, value from ("+
				"select *, datetime(strftime('%%s', substr(timestamp, 0, 20)) / ? * ?, 'unixepoch') as bucket_timestamp,"+
				"row_number() over (partition by strftime('%%s', substr(timestamp, 0, 20)) / ? order by timestamp desc) as rn "+
				"from %s where name = ? and timestamp >= ? and timestamp <= ?) where rn = 1",
				db.PointsTable()), int(duration.Seconds()), int(duration.Seconds()), int(duration.Seconds()), name, begin, end).
			Rows()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		defer rows.Close()
		for rows.Next() {
			var point TimeSeriesPoint
			var timestamp string
			if err := rows.Scan(&point.Name, &timestamp, &point.Value); err != nil {
				return nil, errors.WithStack(err)
			}
			point.Timestamp, err = dateparse.ParseAny(timestamp)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			points = append(points, point)
		}
	}
	return points, nil
}

const sqlGeoVersion = 1

// Serialize local schema changes against point operations, including operations
// through separate SQLDatabase handles. DDL is deliberately not wrapped in a
// transaction: MySQL commits it implicitly. The catalog is the recovery journal.
var sqlGeoMu sync.RWMutex

type sqlGeoIndex struct {
	Name         string
	PhysicalName string
	Version      int
	State        string
}

func (db *SQLDatabase) quoteGeoIdentifier(name string) string {
	var quoted strings.Builder
	db.gormDB.Dialector.QuoteTo(&quoted, name)
	return quoted.String()
}

func (db *SQLDatabase) geoCatalogTable() string {
	return db.quoteGeoIdentifier(geoCatalogName(string(db.TablePrefix)))
}

func (db *SQLDatabase) geoStringType() string {
	switch db.driver {
	case MySQL:
		return "VARBINARY(256)"
	case Postgres:
		return "BYTEA"
	default:
		return "TEXT COLLATE BINARY"
	}
}

// PostgreSQL TEXT cannot represent embedded NULs; bind logical keys as BYTEA.
func (db *SQLDatabase) geoStringArg(value string) any {
	if db.driver == Postgres {
		return []byte(value)
	}
	return value
}

func (db *SQLDatabase) checkGeoMetadata(index sqlGeoIndex) error {
	if index.Version != sqlGeoVersion || index.PhysicalName != geoPhysicalName(string(db.TablePrefix), index.Name) {
		return fmt.Errorf("invalid geo index metadata for %q", index.Name)
	}
	switch index.State {
	case "creating", "ready", "dropping":
		return nil
	default:
		return fmt.Errorf("invalid geo index state for %q", index.Name)
	}
}

func (db *SQLDatabase) lookupGeoIndex(ctx context.Context, name string) (sqlGeoIndex, error) {
	var indexes []sqlGeoIndex
	err := db.gormDB.WithContext(ctx).Raw("SELECT name, physical_name, version, state FROM "+db.geoCatalogTable()+" WHERE name = ?", db.geoStringArg(name)).Scan(&indexes).Error
	if err != nil {
		return sqlGeoIndex{}, err
	}
	if len(indexes) == 0 {
		return sqlGeoIndex{}, fmt.Errorf("geo index %q %w", name, storage.ErrNotFound)
	}
	if err = db.checkGeoMetadata(indexes[0]); err != nil {
		return sqlGeoIndex{}, err
	}
	return indexes[0], nil
}

func (db *SQLDatabase) readyGeoIndex(ctx context.Context, name string) (sqlGeoIndex, error) {
	index, err := db.lookupGeoIndex(ctx, name)
	if err != nil {
		return sqlGeoIndex{}, err
	}
	if index.State != "ready" {
		return sqlGeoIndex{}, fmt.Errorf("geo index %q %w", name, storage.ErrNotFound)
	}
	return index, nil
}

func (db *SQLDatabase) initGeoIndexes() error {
	sqlGeoMu.Lock()
	defer sqlGeoMu.Unlock()
	ctx := context.Background()
	catalog := db.geoCatalogTable()
	physicalType := db.geoStringType()
	if db.driver == Postgres {
		physicalType = `TEXT COLLATE "C"`
	}
	ddl := "CREATE TABLE IF NOT EXISTS " + catalog + " (name " + db.geoStringType() + " NOT NULL PRIMARY KEY, physical_name " + physicalType + " NOT NULL UNIQUE, version INTEGER NOT NULL, state VARCHAR(16) NOT NULL)"
	if err := db.gormDB.WithContext(ctx).Exec(ddl).Error; err != nil {
		return err
	}
	indexes, err := db.listGeoIndexes(ctx)
	if err != nil {
		return err
	}
	for _, index := range indexes {
		switch index.State {
		case "creating":
			if err = db.finishGeoIndexCreation(ctx, index); err != nil {
				return err
			}
		case "dropping":
			if err = db.dropGeoIndex(ctx, index); err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *SQLDatabase) listGeoIndexes(ctx context.Context) ([]sqlGeoIndex, error) {
	var indexes []sqlGeoIndex
	if err := db.gormDB.WithContext(ctx).Raw("SELECT name, physical_name, version, state FROM " + db.geoCatalogTable()).Scan(&indexes).Error; err != nil {
		return nil, err
	}
	for _, index := range indexes {
		if err := db.checkGeoMetadata(index); err != nil {
			return nil, err
		}
	}
	return indexes, nil
}

// dropGeoIndex records intent before DDL and removes metadata only after the
// table is gone, so an interrupted drop can safely be retried during Init.
func (db *SQLDatabase) dropGeoIndex(ctx context.Context, index sqlGeoIndex) error {
	catalog := db.geoCatalogTable()
	if err := db.gormDB.WithContext(ctx).Exec("UPDATE "+catalog+" SET state = ? WHERE name = ? AND physical_name = ?", "dropping", db.geoStringArg(index.Name), index.PhysicalName).Error; err != nil {
		return err
	}
	if err := db.gormDB.WithContext(ctx).Exec("DROP TABLE IF EXISTS " + db.quoteGeoIdentifier(index.PhysicalName)).Error; err != nil {
		return err
	}
	return db.gormDB.WithContext(ctx).Exec("DELETE FROM "+catalog+" WHERE name = ? AND physical_name = ?", db.geoStringArg(index.Name), index.PhysicalName).Error
}

func (db *SQLDatabase) purgeGeoIndexes() error {
	sqlGeoMu.Lock()
	defer sqlGeoMu.Unlock()
	ctx := context.Background()
	indexes, err := db.listGeoIndexes(ctx)
	if err != nil {
		return err
	}
	for _, index := range indexes {
		if err = db.dropGeoIndex(ctx, index); err != nil {
			return err
		}
	}
	return nil
}

func (db *SQLDatabase) AddGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	sqlGeoMu.Lock()
	defer sqlGeoMu.Unlock()
	index, err := db.lookupGeoIndex(ctx, name)
	if err == nil {
		if index.State == "ready" {
			return fmt.Errorf("geo index %q %w", name, storage.ErrAlreadyExists)
		}
		if index.State == "creating" {
			return db.finishGeoIndexCreation(ctx, index)
		}
		// Finish an interrupted drop before publishing a new creation intent.
		if err = db.dropGeoIndex(ctx, index); err != nil {
			return err
		}
	} else if !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	physical := geoPhysicalName(string(db.TablePrefix), name)
	catalog := db.geoCatalogTable()
	if err = db.gormDB.WithContext(ctx).Exec("INSERT INTO "+catalog+" (name, physical_name, version, state) VALUES (?, ?, ?, ?)", db.geoStringArg(name), physical, sqlGeoVersion, "creating").Error; err != nil {
		// A different connection/process may have published the same name.
		if existing, lookupErr := db.lookupGeoIndex(ctx, name); lookupErr == nil && existing.State == "ready" {
			return fmt.Errorf("geo index %q %w", name, storage.ErrAlreadyExists)
		}
		return err
	}
	return db.finishGeoIndexCreation(ctx, sqlGeoIndex{Name: name, PhysicalName: physical, Version: sqlGeoVersion, State: "creating"})
}

// Resume only a validated catalog owner's creation intent. Never adopt a ready
// index or remove its table to recover an interrupted creation.
func (db *SQLDatabase) finishGeoIndexCreation(ctx context.Context, index sqlGeoIndex) error {
	owner, err := db.lookupGeoIndex(ctx, index.Name)
	if err != nil {
		return err
	}
	if owner != index || owner.State != "creating" {
		return fmt.Errorf("geo index %q creation no longer authorized %w", index.Name, storage.ErrNotFound)
	}
	table := db.quoteGeoIdentifier(index.PhysicalName)
	ddl := "CREATE TABLE IF NOT EXISTS " + table + " (id " + db.geoStringType() + " NOT NULL PRIMARY KEY, longitude DOUBLE PRECISION NOT NULL, latitude DOUBLE PRECISION NOT NULL, cell BIGINT NOT NULL)"
	if err = db.gormDB.WithContext(ctx).Exec(ddl).Error; err != nil {
		return err
	}
	// The bounded name is unique across PostgreSQL/SQLite's schema-wide index
	// namespace as well as MySQL's table-local namespace.
	indexName := "gidx_" + strings.TrimPrefix(index.PhysicalName, "ggeo_")
	rangeIndex := db.quoteGeoIdentifier(indexName)
	if db.driver == MySQL {
		var count int64
		if err = db.gormDB.WithContext(ctx).Raw("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?", index.PhysicalName, indexName).Scan(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err = db.gormDB.WithContext(ctx).Exec("CREATE INDEX " + rangeIndex + " ON " + table + " (cell, id)").Error; err != nil {
				// Another creator may win between the existence probe and DDL.
				var mysqlErr *mysql.MySQLError
				if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1061 {
					return err
				}
			}
		}
	} else if err = db.gormDB.WithContext(ctx).Exec("CREATE INDEX IF NOT EXISTS " + rangeIndex + " ON " + table + " (cell, id)").Error; err != nil {
		return err
	}
	result := db.gormDB.WithContext(ctx).Exec("UPDATE "+db.geoCatalogTable()+" SET state = ? WHERE name = ? AND physical_name = ? AND version = ? AND state = ?", "ready", db.geoStringArg(index.Name), index.PhysicalName, index.Version, "creating")
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("geo index %q creation no longer authorized %w", index.Name, storage.ErrNotFound)
	}
	return nil
}

func (db *SQLDatabase) AddGeoPoints(ctx context.Context, name string, points []GeoPoint) error {
	records, err := prepareGeoPoints(name, points)
	if err != nil {
		return err
	}
	sqlGeoMu.RLock()
	defer sqlGeoMu.RUnlock()
	index, err := db.readyGeoIndex(ctx, name)
	if err != nil {
		return err
	}
	table := db.quoteGeoIdentifier(index.PhysicalName)
	statement := "INSERT INTO " + table + " (id, longitude, latitude, cell) VALUES (?, ?, ?, ?)"
	if db.driver == MySQL {
		statement += " ON DUPLICATE KEY UPDATE longitude = VALUES(longitude), latitude = VALUES(latitude), cell = VALUES(cell)"
	} else {
		statement += " ON CONFLICT (id) DO UPDATE SET longitude = excluded.longitude, latitude = excluded.latitude, cell = excluded.cell"
	}
	for _, record := range records {
		if err = db.gormDB.WithContext(ctx).Exec(statement, db.geoStringArg(record.Id), record.Longitude, record.Latitude, record.Cell).Error; err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (db *SQLDatabase) SearchGeoPoints(ctx context.Context, name string, query GeoQuery) ([]GeoResult, error) {
	query, err := normalizeGeoQuery(name, query)
	if err != nil {
		return nil, err
	}
	sqlGeoMu.RLock()
	defer sqlGeoMu.RUnlock()
	index, err := db.readyGeoIndex(ctx, name)
	if err != nil {
		return nil, err
	}
	return searchGeoPoints(ctx, query, func(ctx context.Context, ranges []geoRange, emit func(geoRecord) error) error {
		return db.scanGeoPoints(ctx, index.PhysicalName, ranges, emit)
	})
}

// scanGeoPoints uses bounded keyset pages, not a LIMIT on the total candidate
// set. The column collation makes the cursor order match byte-exact IDs.
func (db *SQLDatabase) scanGeoPoints(ctx context.Context, physical string, ranges []geoRange, emit func(geoRecord) error) error {
	table := db.quoteGeoIdentifier(physical)
	for _, cellRange := range ranges {
		var last geoRecord
		first := true
		for {
			statement := "SELECT id, longitude, latitude, cell FROM " + table + " WHERE cell >= ? AND cell <= ?"
			args := []any{cellRange.Min, cellRange.Max}
			if !first {
				statement += " AND (cell > ? OR (cell = ? AND id > ?))"
				args = append(args, last.Cell, last.Cell, db.geoStringArg(last.Id))
			}
			statement += " ORDER BY cell, id LIMIT 512"
			rows, err := db.gormDB.WithContext(ctx).Raw(statement, args...).Rows()
			if err != nil {
				return err
			}
			page := make([]geoRecord, 0, 512)
			for rows.Next() {
				var record geoRecord
				if err = rows.Scan(&record.Id, &record.Longitude, &record.Latitude, &record.Cell); err != nil {
					rows.Close()
					return err
				}
				page = append(page, record)
			}
			rowsErr := rows.Err()
			closeErr := rows.Close()
			if rowsErr != nil {
				return rowsErr
			}
			if closeErr != nil {
				return closeErr
			}
			for _, record := range page {
				if err = ctx.Err(); err != nil {
					return err
				}
				if err = emit(record); err != nil {
					return err
				}
			}
			if len(page) < 512 {
				break
			}
			last = page[len(page)-1]
			first = false
		}
	}
	return ctx.Err()
}

func (db *SQLDatabase) DeleteGeoPoints(ctx context.Context, name string, ids ...string) error {
	ids, err := prepareGeoIDs(name, ids)
	if err != nil {
		return err
	}
	sqlGeoMu.RLock()
	defer sqlGeoMu.RUnlock()
	index, err := db.readyGeoIndex(ctx, name)
	if err != nil {
		return err
	}
	// Bound parameter counts even for large deletes.
	for begin := 0; begin < len(ids); begin += 512 {
		end := begin + 512
		if end > len(ids) {
			end = len(ids)
		}
		args := make([]any, end-begin)
		for i, id := range ids[begin:end] {
			args[i] = db.geoStringArg(id)
		}
		if err = db.gormDB.WithContext(ctx).Exec("DELETE FROM "+db.quoteGeoIdentifier(index.PhysicalName)+" WHERE id IN ?", args).Error; err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (db *SQLDatabase) DeleteGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	sqlGeoMu.Lock()
	defer sqlGeoMu.Unlock()
	index, err := db.lookupGeoIndex(ctx, name)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return db.dropGeoIndex(ctx, index)
}
