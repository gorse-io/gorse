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
	"context"
	"github.com/zhenghaoz/gorse/common/expression"
	"time"
)

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

func (NoDatabase) Ping() error {
	return ErrNoDatabase
}

// Close method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Close() error {
	return ErrNoDatabase
}

func (NoDatabase) Purge() error {
	return ErrNoDatabase
}

// BatchInsertItems method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) BatchInsertItems(_ context.Context, _ []Item) error {
	return ErrNoDatabase
}

// BatchGetItems method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) BatchGetItems(_ context.Context, _ []string) ([]Item, error) {
	return nil, ErrNoDatabase
}

// DeleteItem method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) DeleteItem(_ context.Context, _ string) error {
	return ErrNoDatabase
}

// GetItem method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetItem(_ context.Context, _ string) (Item, error) {
	return Item{}, ErrNoDatabase
}

// GetItems method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetItems(_ context.Context, _ string, _ int, _ *time.Time) (string, []Item, error) {
	return "", nil, ErrNoDatabase
}

// GetItemStream method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetItemStream(_ context.Context, _ int, _ *time.Time) (chan []Item, chan error) {
	itemChan := make(chan []Item, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(itemChan)
		defer close(errChan)
		errChan <- ErrNoDatabase
	}()
	return itemChan, errChan
}

// GetItemFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetItemFeedback(_ context.Context, _ string, _ ...string) ([]Feedback, error) {
	return nil, ErrNoDatabase
}

// BatchInsertUsers method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) BatchInsertUsers(_ context.Context, _ []User) error {
	return ErrNoDatabase
}

// DeleteUser method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) DeleteUser(_ context.Context, _ string) error {
	return ErrNoDatabase
}

// GetUser method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUser(_ context.Context, _ string) (User, error) {
	return User{}, ErrNoDatabase
}

// GetUsers method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUsers(_ context.Context, _ string, _ int) (string, []User, error) {
	return "", nil, ErrNoDatabase
}

// GetUserStream method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUserStream(_ context.Context, _ int) (chan []User, chan error) {
	userChan := make(chan []User, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(userChan)
		defer close(errChan)
		errChan <- ErrNoDatabase
	}()
	return userChan, errChan
}

// GetUserFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUserFeedback(context.Context, string, *time.Time, ...expression.FeedbackTypeExpression) ([]Feedback, error) {
	return nil, ErrNoDatabase
}

// GetUserItemFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetUserItemFeedback(_ context.Context, _, _ string, _ ...string) ([]Feedback, error) {
	return nil, ErrNoDatabase
}

// DeleteUserItemFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) DeleteUserItemFeedback(_ context.Context, _, _ string, _ ...string) (int, error) {
	return 0, ErrNoDatabase
}

// BatchInsertFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) BatchInsertFeedback(_ context.Context, _ []Feedback, _, _, _ bool) error {
	return ErrNoDatabase
}

// GetFeedback method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetFeedback(_ context.Context, _ string, _ int, _, _ *time.Time, _ ...string) (string, []Feedback, error) {
	return "", nil, ErrNoDatabase
}

// GetFeedbackStream method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetFeedbackStream(_ context.Context, _ int, _ ...ScanOption) (chan []Feedback, chan error) {
	feedbackChan := make(chan []Feedback, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(feedbackChan)
		defer close(errChan)
		errChan <- ErrNoDatabase
	}()
	return feedbackChan, errChan
}

func (d NoDatabase) ModifyItem(_ context.Context, _ string, _ ItemPatch) error {
	return ErrNoDatabase
}

func (d NoDatabase) ModifyUser(_ context.Context, _ string, _ UserPatch) error {
	return ErrNoDatabase
}

func (d NoDatabase) CountUsers(_ context.Context) (int, error) {
	return 0, ErrNoDatabase
}

func (d NoDatabase) CountItems(_ context.Context) (int, error) {
	return 0, ErrNoDatabase
}

func (d NoDatabase) CountFeedback(_ context.Context) (int, error) {
	return 0, ErrNoDatabase
}
