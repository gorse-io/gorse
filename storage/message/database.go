// Copyright 2021 gorse Project Authors
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

package message

import (
	"github.com/XSAM/otelsql"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/storage"
	semconv "go.opentelemetry.io/otel/semconv/v1.8.0"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"moul.io/zapgorm2"
	"strings"
	"time"
)

type Message struct {
	Data      string
	Timestamp time.Time
}

type Database interface {
	Init() error
	Push(name string, message Message) error
	Pop(name string) (Message, error)
}

func Open(path, prefix string) (Database, error) {
	var err error
	if strings.HasPrefix(path, storage.SQLitePrefix) {
		// append parameters
		if path, err = storage.AppendURLParams(path, []lo.Tuple2[string, string]{
			{"_pragma", "busy_timeout(10000)"},
			{"_pragma", "journal_mode(wal)"},
		}); err != nil {
			return nil, errors.Trace(err)
		}
		// connect to database
		name := path[len(storage.SQLitePrefix):]
		database := new(SQLite)
		if database.client, err = otelsql.Open("sqlite", name,
			otelsql.WithAttributes(semconv.DBSystemSqlite),
			otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
		); err != nil {
			return nil, errors.Trace(err)
		}
		gormConfig := storage.NewGORMConfig(prefix)
		gormConfig.Logger = &zapgorm2.Logger{
			ZapLogger:                 log.Logger(),
			LogLevel:                  logger.Warn,
			SlowThreshold:             10 * time.Second,
			SkipCallerLookup:          false,
			IgnoreRecordNotFoundError: false,
		}
		database.gormDB, err = gorm.Open(sqlite.Dialector{Conn: database.client}, gormConfig)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
