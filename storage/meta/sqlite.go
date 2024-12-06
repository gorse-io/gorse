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
	update_time TIMESTAMP
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
	return nil
}

func (s *SQLite) SetTTL(ttl time.Duration) {
	s.ttl = ttl
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
`, node.UUID, node.Hostname, node.Type, node.Version, node.UpdateTime)
	return err
}

func (s *SQLite) ListNodes() ([]*Node, error) {
	rs, err := s.db.Query(`
SELECT uuid, hostname, type, version, update_time FROM nodes
`)
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
	return nodes, nil
}

func (s *SQLite) UpdateCronJob(cronJob *CronJob) error {
	//TODO implement me
	panic("implement me")
}

func (s *SQLite) ListCronJobs() ([]*CronJob, error) {
	rs, err := s.db.Query(`
SELECT name, description, current, total, start_time, end_time, update_time FROM cron_jobs
`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var cronJobs []*CronJob
	for rs.Next() {
		var cronJob CronJob
		if err = rs.Scan(&cronJob.Name, &cronJob.Description, &cronJob.Current, &cronJob.Total, &cronJob.StartTime, &cronJob.EndTime, &cronJob.UpdateTime); err != nil {
			return nil, err
		}
		cronJobs = append(cronJobs, &cronJob)
	}
	return cronJobs, nil
}
