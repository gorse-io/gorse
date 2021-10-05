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
func (NoDatabase) SetScores(_, _ string, _ []Scored) error {
	return ErrNoDatabase
}

// GetScores method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetScores(_, _ string, _, _ int) ([]Scored, error) {
	return nil, ErrNoDatabase
}

// ClearScores method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) ClearScores(_, _ string) error {
	return ErrNoDatabase
}

// AppendScores method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) AppendScores(_, _ string, _ ...Scored) error {
	return ErrNoDatabase
}

// GetString method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetString(_, _ string) (string, error) {
	return "", ErrNoDatabase
}

// SetString method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) SetString(_, _, _ string) error {
	return ErrNoDatabase
}

// GetInt method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetInt(_, _ string) (int, error) {
	return 0, ErrNoDatabase
}

// SetInt method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) SetInt(_, _ string, _ int) error {
	return ErrNoDatabase
}

// GetTime method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetTime(_, _ string) (time.Time, error) {
	return time.Time{}, ErrNoDatabase
}

// SetTime method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) SetTime(_, _ string, _ time.Time) error {
	return ErrNoDatabase
}

// IncrInt method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) IncrInt(_, _ string) error {
	return ErrNoDatabase
}
