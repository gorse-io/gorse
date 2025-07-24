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
	"database/sql"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"time"
)

type SQLite struct {
	db  *sql.DB
	ttl time.Duration
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

func (s *SQLite) Init() error {
	// Create tables
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS nodes (
	uuid TEXT PRIMARY KEY,
	hostname TEXT,
	type TEXT,
	version TEXT,
	update_time DATETIME
);`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS cron_jobs (
	name TEXT PRIMARY KEY,
	description TEXT,
	current INTEGER,
	total INTEGER,
	start_time TIMESTAMP,
	end_time TIMESTAMP,
	update_time TIMESTAMP
);`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS key_values (
	key TEXT PRIMARY KEY,
	value TEXT
);`); err != nil {
		return err
	}
	return nil
}

func (s *SQLite) UpdateNode(node *Node) error {
	_, err := s.db.Exec(`
INSERT INTO nodes (uuid, hostname, type, version, update_time)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(uuid) DO UPDATE SET
	hostname = excluded.hostname,
	type = excluded.type,
	version = excluded.version,
	update_time = excluded.update_time
`, node.UUID, node.Hostname, node.Type, node.Version, node.UpdateTime.UTC())
	return err
}

func (s *SQLite) ListNodes() ([]*Node, error) {
	// List nodes within TTL
	rs, err := s.db.Query(`
SELECT uuid, hostname, type, version, update_time FROM nodes
WHERE update_time > datetime('now', ?)
`, fmt.Sprintf("-%.0f seconds", s.ttl.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var nodes []*Node
	for rs.Next() {
		var node Node
		if err = rs.Scan(&node.UUID, &node.Hostname, &node.Type, &node.Version, &node.UpdateTime); err != nil {
			return nil, err
		}
		nodes = append(nodes, &node)
	}
	// Delete outdated nodes
	if _, err = s.db.Exec(`
DELETE FROM nodes WHERE update_time < datetime('now', ?)
`, fmt.Sprintf("-%.0f seconds", s.ttl.Seconds())); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (s *SQLite) Put(key, value string) error {
	_, err := s.db.Exec(`
INSERT INTO key_values (key, value) VALUES (?, ?)
`, key, value)
	return err
}

func (s *SQLite) Get(key string) (*string, error) {
	var value string
	err := s.db.QueryRow(`
SELECT value FROM key_values WHERE key = ?
`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // key not found
		}
		return nil, err // other error
	}
	return &value, nil // key found
}
