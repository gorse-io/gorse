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

// Init method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Init() error {
	return ErrNoDatabase
}

// Close method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Close() error {
	return ErrNoDatabase
}

// InsertItem method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) InsertItem(item Item) error {
	return ErrNoDatabase
}

// BatchInsertItem method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) BatchInsertItem(items []Item) error {
	return ErrNoDatabase
}

// DeleteItem method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) DeleteItem(itemId string) error {
	return ErrNoDatabase
}

// GetItem method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetItem(itemId string) (Item, error) {
	return Item{}, ErrNoDatabase
}

// GetItems method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetItems(cursor string, n int, time *time.Time) (string, []Item, error) {
	return "", nil, ErrNoDatabase
}

// GetItemFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetItemFeedback(itemId string, feedbackTypes ...string) ([]Feedback, error) {
	return nil, ErrNoDatabase
}

// InsertUser method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) InsertUser(user User) error {
	return ErrNoDatabase
}

// DeleteUser method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) DeleteUser(userId string) error {
	return ErrNoDatabase
}

// GetUser method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUser(userId string) (User, error) {
	return User{}, ErrNoDatabase
}

// GetUsers method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUsers(cursor string, n int) (string, []User, error) {
	return "", nil, ErrNoDatabase
}

// GetUserFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUserFeedback(userId string, feedbackTypes ...string) ([]Feedback, error) {
	return nil, ErrNoDatabase
}

// GetUserItemFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUserItemFeedback(userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	return nil, ErrNoDatabase
}

// DeleteUserItemFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) DeleteUserItemFeedback(userId, itemId string, feedbackTypes ...string) (int, error) {
	return 0, ErrNoDatabase
}

// InsertFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) InsertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	return ErrNoDatabase
}

// BatchInsertFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error {
	return ErrNoDatabase
}

// GetFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetFeedback(cursor string, n int, time *time.Time, feedbackTypes ...string) (string, []Feedback, error) {
	return "", nil, ErrNoDatabase
}

// InsertMeasurement method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) InsertMeasurement(measurement Measurement) error {
	return ErrNoDatabase
}

// GetMeasurements method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetMeasurements(name string, n int) ([]Measurement, error) {
	return nil, ErrNoDatabase
}

// GetClickThroughRate method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetClickThroughRate(date time.Time, positiveTypes []string, readType string) (float64, error) {
	return 0, ErrNoDatabase
}

// CountActiveUsers method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) CountActiveUsers(date time.Time) (int, error) {
	return 0, ErrNoDatabase
}
