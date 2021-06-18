// Copyright 2020 gorse Project Authors
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

import "time"

// NoDatabase means no database used for cache.
type NoDatabase struct{}

// Close method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Close() error {
	return ErrNoDatabase
}

// SetScores method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) SetScores(prefix, name string, items []ScoredItem) error {
	return ErrNoDatabase
}

// GetScores method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetScores(prefix, name string, begin, end int) ([]ScoredItem, error) {
	return nil, ErrNoDatabase
}

// ClearList method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) ClearList(prefix, name string) error {
	return ErrNoDatabase
}

// AppendList method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) AppendList(prefix, name string, items ...string) error {
	return ErrNoDatabase
}

// GetList method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetList(prefix, name string) ([]string, error) {
	return nil, ErrNoDatabase
}

// GetString method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetString(prefix, name string) (string, error) {
	return "", ErrNoDatabase
}

// SetString method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) SetString(prefix, name, val string) error {
	return ErrNoDatabase
}

// GetInt method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetInt(prefix, name string) (int, error) {
	return 0, ErrNoDatabase
}

// SetInt method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) SetInt(prefix, name string, val int) error {
	return ErrNoDatabase
}

// GetTime method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetTime(prefix, name string) (time.Time, error) {
	return time.Time{}, ErrNoDatabase
}

// SetTime method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) SetTime(prefix, name string, val time.Time) error {
	return ErrNoDatabase
}

// IncrInt method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) IncrInt(prefix, name string) error {
	return ErrNoDatabase
}
