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

package data

import (
	"fmt"
	"time"
)

var ErrNoDatabase = fmt.Errorf("no database specified for datastore")

// NoDatabase means that no database used.
type NoDatabase struct{}

// Optimize is used by ClickHouse only.
func (NoDatabase) Optimize() error {
	return ErrNoDatabase
}

// Init method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Init() error {
	return ErrNoDatabase
}

// Close method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Close() error {
	return ErrNoDatabase
}

// InsertItem method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) InsertItem(_ Item) error {
	return ErrNoDatabase
}

// BatchInsertItem method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) BatchInsertItem(_ []Item) error {
	return ErrNoDatabase
}

// DeleteItem method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) DeleteItem(_ string) error {
	return ErrNoDatabase
}

// GetItem method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetItem(_ string) (Item, error) {
	return Item{}, ErrNoDatabase
}

// GetItems method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetItems(_ string, _ int, _ *time.Time) (string, []Item, error) {
	return "", nil, ErrNoDatabase
}

// GetItemFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetItemFeedback(_ string, _ ...string) ([]Feedback, error) {
	return nil, ErrNoDatabase
}

// InsertUser method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) InsertUser(_ User) error {
	return ErrNoDatabase
}

// DeleteUser method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) DeleteUser(_ string) error {
	return ErrNoDatabase
}

// GetUser method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUser(_ string) (User, error) {
	return User{}, ErrNoDatabase
}

// GetUsers method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUsers(_ string, _ int) (string, []User, error) {
	return "", nil, ErrNoDatabase
}

// GetUserFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUserFeedback(_ string, _ ...string) ([]Feedback, error) {
	return nil, ErrNoDatabase
}

// GetUserItemFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUserItemFeedback(_, _ string, _ ...string) ([]Feedback, error) {
	return nil, ErrNoDatabase
}

// DeleteUserItemFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) DeleteUserItemFeedback(_, _ string, _ ...string) (int, error) {
	return 0, ErrNoDatabase
}

// InsertFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) InsertFeedback(_ Feedback, _, _ bool) error {
	return ErrNoDatabase
}

// BatchInsertFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) BatchInsertFeedback(_ []Feedback, _, _ bool) error {
	return ErrNoDatabase
}

// GetFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetFeedback(_ string, _ int, _ *time.Time, _ ...string) (string, []Feedback, error) {
	return "", nil, ErrNoDatabase
}

// InsertMeasurement method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) InsertMeasurement(_ Measurement) error {
	return ErrNoDatabase
}

// GetMeasurements method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetMeasurements(_ string, _ int) ([]Measurement, error) {
	return nil, ErrNoDatabase
}

// GetClickThroughRate method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetClickThroughRate(_ time.Time, _ []string, _ string) (float64, error) {
	return 0, ErrNoDatabase
}
