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

var NoDatabaseError = fmt.Errorf("no database specified for datastore")

// NoDatabase means that no database used.
type NoDatabase struct{}

// Init method of NoDatabase returns NoDatabaseError.
func (NoDatabase) Init() error {
	return NoDatabaseError
}

// Close method of NoDatabase returns NoDatabaseError.
func (NoDatabase) Close() error {
	return NoDatabaseError
}

// InsertItem method of NoDatabase returns NoDatabaseError.
func (NoDatabase) InsertItem(item Item) error {
	return NoDatabaseError
}

// BatchInsertItem method of NoDatabase returns NoDatabaseError.
func (NoDatabase) BatchInsertItem(items []Item) error {
	return NoDatabaseError
}

// DeleteItem method of NoDatabase returns NoDatabaseError.
func (NoDatabase) DeleteItem(itemId string) error {
	return NoDatabaseError
}

// GetItem method of NoDatabase returns NoDatabaseError.
func (NoDatabase) GetItem(itemId string) (Item, error) {
	return Item{}, NoDatabaseError
}

// GetItems method of NoDatabase returns NoDatabaseError.
func (NoDatabase) GetItems(cursor string, n int, time *time.Time) (string, []Item, error) {
	return "", nil, NoDatabaseError
}

// GetItemFeedback method of NoDatabase returns NoDatabaseError.
func (NoDatabase) GetItemFeedback(itemId string, feedbackType *string) ([]Feedback, error) {
	return nil, NoDatabaseError
}

// InsertUser method of NoDatabase returns NoDatabaseError.
func (NoDatabase) InsertUser(user User) error {
	return NoDatabaseError
}

// DeleteUser method of NoDatabase returns NoDatabaseError.
func (NoDatabase) DeleteUser(userId string) error {
	return NoDatabaseError
}

// GetUser method of NoDatabase returns NoDatabaseError.
func (NoDatabase) GetUser(userId string) (User, error) {
	return User{}, NoDatabaseError
}

// GetUsers method of NoDatabase returns NoDatabaseError.
func (NoDatabase) GetUsers(cursor string, n int) (string, []User, error) {
	return "", nil, NoDatabaseError
}

// GetUserFeedback method of NoDatabase returns NoDatabaseError.
func (NoDatabase) GetUserFeedback(userId string, feedbackType *string) ([]Feedback, error) {
	return nil, NoDatabaseError
}

// GetUserItemFeedback method of NoDatabase returns NoDatabaseError.
func (NoDatabase) GetUserItemFeedback(userId, itemId string, feedbackType *string) ([]Feedback, error) {
	return nil, NoDatabaseError
}

// DeleteUserItemFeedback method of NoDatabase returns NoDatabaseError.
func (NoDatabase) DeleteUserItemFeedback(userId, itemId string, feedbackType *string) (int, error) {
	return 0, NoDatabaseError
}

// InsertFeedback method of NoDatabase returns NoDatabaseError.
func (NoDatabase) InsertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	return NoDatabaseError
}

// BatchInsertFeedback method of NoDatabase returns NoDatabaseError.
func (NoDatabase) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error {
	return NoDatabaseError
}

// GetFeedback method of NoDatabase returns NoDatabaseError.
func (NoDatabase) GetFeedback(cursor string, n int, feedbackType *string, time *time.Time) (string, []Feedback, error) {
	return "", nil, NoDatabaseError
}

// InsertMeasurement method of NoDatabase returns NoDatabaseError.
func (NoDatabase) InsertMeasurement(measurement Measurement) error {
	return NoDatabaseError
}

// GetMeasurements method of NoDatabase returns NoDatabaseError.
func (NoDatabase) GetMeasurements(name string, n int) ([]Measurement, error) {
	return nil, NoDatabaseError
}
