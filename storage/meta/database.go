// Copyright 2024 gorse Project Authors
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

package meta

import (
	"encoding/json"
	"github.com/XSAM/otelsql"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/storage"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"strings"
	"time"
)

const (
	COLLABORATIVE_FILTERING_MODEL = "COLLABORATIVE_FILTERING_MODEL"
	CLICK_THROUGH_RATE_MODEL      = "CLICK_THROUGH_RATE_MODEL"
)

type Model[T any] struct {
	ID    int64
	Score T
}

func (m *Model[T]) ToJSON() string {
	return string(lo.Must1(json.Marshal(m)))
}

func (m *Model[T]) FromJSON(data string) error {
	return json.Unmarshal([]byte(data), m)
}

type Node struct {
	UUID       string
	Hostname   string
	Type       string
	Version    string
	UpdateTime time.Time
}

type Database interface {
	Close() error
	Init() error
	UpdateNode(node *Node) error
	ListNodes() ([]*Node, error)
	Put(key, value string) error
	Get(key string) (*string, error)
}

// Open a connection to a database.
func Open(path string, ttl time.Duration) (Database, error) {
	var err error
	if strings.HasPrefix(path, storage.SQLitePrefix) {
		dataSourceName := path[len(storage.SQLitePrefix):]
		// append parameters
		if dataSourceName, err = storage.AppendURLParams(dataSourceName, []lo.Tuple2[string, string]{
			{"_pragma", "busy_timeout(10000)"},
			{"_pragma", "journal_mode(wal)"},
		}); err != nil {
			return nil, errors.Trace(err)
		}
		// connect to database
		database := new(SQLite)
		database.ttl = ttl
		if database.db, err = otelsql.Open("sqlite", dataSourceName,
			otelsql.WithAttributes(semconv.DBSystemSqlite),
			otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
		); err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
