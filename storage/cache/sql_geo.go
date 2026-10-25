// Copyright 2026 gorse Project Authors
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
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/gorse-io/gorse/storage"
)

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
