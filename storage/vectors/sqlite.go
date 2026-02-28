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

package vectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	_ "modernc.org/sqlite/vec"
)

func init() {
	Register([]string{storage.SQLitePrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		database := new(SQLite)
		// Strip sqlite:// prefix
		dbPath := strings.TrimPrefix(path, storage.SQLitePrefix)
		var err error
		database.db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	})
}

type SQLite struct {
	db *sql.DB
}

func (db *SQLite) Init() error {
	return nil
}

func (db *SQLite) Optimize() error {
	return nil
}

func (db *SQLite) Close() error {
	return db.db.Close()
}

func (db *SQLite) ListCollections(ctx context.Context) ([]string, error) {
	rows, err := db.db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND sql LIKE '%USING vec0%'")
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, errors.Trace(err)
		}
		names = append(names, name)
	}
	return names, nil
}

func (db *SQLite) AddCollection(ctx context.Context, name string, dimensions int, distance Distance) error {
	var metric string
	switch distance {
	case Cosine:
		metric = "cosine"
	case Euclidean:
		metric = "l2"
	case Dot:
		metric = "ip"
	default:
		return errors.NotSupportedf("distance method")
	}
	_, err := db.db.ExecContext(ctx, fmt.Sprintf("CREATE VIRTUAL TABLE %s USING vec0(id TEXT, categories TEXT, timestamp INTEGER, vector FLOAT[%d] distance_metric=%s)", name, dimensions, metric))
	return errors.Trace(err)
}

func (db *SQLite) DeleteCollection(ctx context.Context, name string) error {
	var count int
	err := db.db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count)
	if err != nil {
		return errors.Trace(err)
	}
	if count == 0 {
		return errors.NotFoundf("collection %s", name)
	}
	_, err = db.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", name))
	return errors.Trace(err)
}

func (db *SQLite) AddVectors(ctx context.Context, collection string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s(id, categories, timestamp, vector) VALUES(?, ?, ?, ?)", collection))
	if err != nil {
		_ = tx.Rollback()
		return errors.Trace(err)
	}
	defer stmt.Close()

	for _, v := range vectors {
		categories, err := json.Marshal(v.Categories)
		if err != nil {
			_ = tx.Rollback()
			return errors.Trace(err)
		}
		vectorJson, err := json.Marshal(v.Vector)
		if err != nil {
			_ = tx.Rollback()
			return errors.Trace(err)
		}
		timestamp := v.Timestamp.UnixMilli()
		_, err = stmt.ExecContext(ctx, v.Id, string(categories), timestamp, string(vectorJson))
		if err != nil {
			_ = tx.Rollback()
			return errors.Trace(err)
		}
	}
	return errors.Trace(tx.Commit())
}

func (db *SQLite) DeleteVectors(ctx context.Context, collection string, timestamp time.Time) error {
	_, err := db.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE timestamp < ?", collection), timestamp.UnixMilli())
	return errors.Trace(err)
}

func (db *SQLite) QueryVectors(ctx context.Context, collection string, q []float32, categories []string, topK int) ([]Vector, error) {
	if topK <= 0 {
		return []Vector{}, nil
	}

	qJson, err := json.Marshal(q)
	if err != nil {
		return nil, errors.Trace(err)
	}

	query := fmt.Sprintf("SELECT id, categories FROM %s WHERE vector MATCH ? AND k = ? ", collection)
	var args []any
	args = append(args, string(qJson), topK)

	if len(categories) > 0 {
		var categoryConditions []string
		for _, category := range categories {
			categoryConditions = append(categoryConditions, "json_contains(categories, ?)")
			args = append(args, fmt.Sprintf("[%q]", category))
		}
		query += " AND (" + strings.Join(categoryConditions, " OR ") + ")"
	}
	query += " ORDER BY distance"

	rows, err := db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rows.Close()

	var results []Vector
	for rows.Next() {
		var v Vector
		var categoriesStr string
		if err := rows.Scan(&v.Id, &categoriesStr); err != nil {
			return nil, errors.Trace(err)
		}
		if err := json.Unmarshal([]byte(categoriesStr), &v.Categories); err != nil {
			return nil, errors.Trace(err)
		}
		results = append(results, v)
	}
	return results, nil
}
