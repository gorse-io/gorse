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
package data

import "fmt"

var NoDatabaseError = fmt.Errorf("no database specified for datastore")

type NoDatabase struct{}

func (NoDatabase) Init() error {
	return NoDatabaseError
}

func (NoDatabase) Close() error {
	return NoDatabaseError
}

func (NoDatabase) InsertItem(item Item) error {
	return NoDatabaseError
}

func (NoDatabase) BatchInsertItem(items []Item) error {
	return NoDatabaseError
}

func (NoDatabase) DeleteItem(itemId string) error {
	return NoDatabaseError
}

func (NoDatabase) GetItem(itemId string) (Item, error) {
	return Item{}, NoDatabaseError
}

func (NoDatabase) GetItems(cursor string, n int) (string, []Item, error) {
	return "", nil, NoDatabaseError
}

func (NoDatabase) GetItemFeedback(itemId string, feedbackType *string) ([]Feedback, error) {
	return nil, NoDatabaseError
}

func (NoDatabase) InsertUser(user User) error {
	return NoDatabaseError
}

func (NoDatabase) DeleteUser(userId string) error {
	return NoDatabaseError
}

func (NoDatabase) GetUser(userId string) (User, error) {
	return User{}, NoDatabaseError
}

func (NoDatabase) GetUsers(cursor string, n int) (string, []User, error) {
	return "", nil, NoDatabaseError
}

func (NoDatabase) GetUserFeedback(userId string, feedbackType *string) ([]Feedback, error) {
	return nil, NoDatabaseError
}

func (NoDatabase) GetUserItemFeedback(userId, itemId string, feedbackType *string) ([]Feedback, error) {
	return nil, NoDatabaseError
}

func (NoDatabase) DeleteUserItemFeedback(userId, itemId string, feedbackType *string) (int, error) {
	return 0, NoDatabaseError
}

func (NoDatabase) InsertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	return NoDatabaseError
}

func (NoDatabase) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error {
	return NoDatabaseError
}

func (NoDatabase) GetFeedback(cursor string, n int, feedbackType *string) (string, []Feedback, error) {
	return "", nil, NoDatabaseError
}
