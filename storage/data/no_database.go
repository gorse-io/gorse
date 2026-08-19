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
	"time"

	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage"
)

// NoDatabase means that no database used.
type NoDatabase struct{}

// Optimize is used by ClickHouse only.
func (NoDatabase) Optimize() error {
	return storage.ErrNoDatabase
}

// Init method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) Init() error {
	return storage.ErrNoDatabase
}

// Reconcile method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) Reconcile(_ config.SearchConfig) error {
	return storage.ErrNoDatabase
}

func (NoDatabase) Ping() error {
	return storage.ErrNoDatabase
}

// Close method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) Close() error {
	return storage.ErrNoDatabase
}

func (NoDatabase) Purge() error {
	return storage.ErrNoDatabase
}

// BatchInsertItems method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) BatchInsertItems(_ context.Context, _ []Item) error {
	return storage.ErrNoDatabase
}

// BatchGetItems method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) BatchGetItems(_ context.Context, _ []string, _ GetOptions) ([]Item, error) {
	return nil, storage.ErrNoDatabase
}

// DeleteItem method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) DeleteItem(_ context.Context, _ string) error {
	return storage.ErrNoDatabase
}

// GetItem method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetItem(_ context.Context, _ string) (Item, error) {
	return Item{}, storage.ErrNoDatabase
}

// SearchItems method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) SearchItems(_ context.Context, _ string, _ int) ([]ScoredItem, error) {
	return nil, storage.ErrNoDatabase
}

// GetItems method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetItems(_ context.Context, _ string, _ int, _ *time.Time) (string, []Item, error) {
	return "", nil, storage.ErrNoDatabase
}

// GetLatestItems method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetLatestItems(_ context.Context, _ int, _ []string, _ *time.Time) ([]Item, error) {
	return nil, storage.ErrNoDatabase
}

// GetItemStream method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetItemStream(_ context.Context, _ int, _ *time.Time) (chan []Item, chan error) {
	itemChan := make(chan []Item, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(itemChan)
		defer close(errChan)
		errChan <- storage.ErrNoDatabase
	}()
	return itemChan, errChan
}

// GetItemFeedback method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetItemFeedback(_ context.Context, _ string, _ ...string) ([]Feedback, error) {
	return nil, storage.ErrNoDatabase
}

// BatchInsertUsers method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) BatchInsertUsers(_ context.Context, _ []User) error {
	return storage.ErrNoDatabase
}

// DeleteUser method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) DeleteUser(_ context.Context, _ string) error {
	return storage.ErrNoDatabase
}

// GetUser method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetUser(_ context.Context, _ string) (User, error) {
	return User{}, storage.ErrNoDatabase
}

// GetUsers method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetUsers(_ context.Context, _ string, _ int) (string, []User, error) {
	return "", nil, storage.ErrNoDatabase
}

// GetUserStream method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetUserStream(_ context.Context, _ int) (chan []User, chan error) {
	userChan := make(chan []User, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(userChan)
		defer close(errChan)
		errChan <- storage.ErrNoDatabase
	}()
	return userChan, errChan
}

// GetUserFeedback method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetUserFeedback(context.Context, string, *time.Time, ...expression.FeedbackTypeExpression) ([]Feedback, error) {
	return nil, storage.ErrNoDatabase
}

// GetUserItemFeedback method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetUserItemFeedback(_ context.Context, _, _ string, _ ...string) ([]Feedback, error) {
	return nil, storage.ErrNoDatabase
}

// DeleteUserItemFeedback method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) DeleteUserItemFeedback(_ context.Context, _, _ string, _ ...string) (int, error) {
	return 0, storage.ErrNoDatabase
}

// BatchInsertFeedback method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) BatchInsertFeedback(_ context.Context, _ []Feedback, _, _, _ bool) error {
	return storage.ErrNoDatabase
}

// GetFeedback method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetFeedback(_ context.Context, _ string, _ int, _, _ *time.Time, _ ...string) (string, []Feedback, error) {
	return "", nil, storage.ErrNoDatabase
}

// GetFeedbackStream method of NoDatabase returns storage.ErrNoDatabase.
func (NoDatabase) GetFeedbackStream(_ context.Context, _ int, _ ...ScanOption) (chan []Feedback, chan error) {
	feedbackChan := make(chan []Feedback, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(feedbackChan)
		defer close(errChan)
		errChan <- storage.ErrNoDatabase
	}()
	return feedbackChan, errChan
}

func (d NoDatabase) ModifyItem(_ context.Context, _ string, _ ItemPatch) error {
	return storage.ErrNoDatabase
}

func (d NoDatabase) ModifyUser(_ context.Context, _ string, _ UserPatch) error {
	return storage.ErrNoDatabase
}

func (d NoDatabase) CountUsers(_ context.Context) (int, error) {
	return 0, storage.ErrNoDatabase
}

func (d NoDatabase) CountItems(_ context.Context) (int, error) {
	return 0, storage.ErrNoDatabase
}

func (d NoDatabase) CountFeedback(_ context.Context) (int, error) {
	return 0, storage.ErrNoDatabase
}
