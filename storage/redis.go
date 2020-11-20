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
package storage

import (
	"github.com/go-redis/redis/v8"
)

type Redis struct {
	client *redis.Client
}

func (redis *Redis) Close() error {
	return redis.client.Close()
}

func (redis *Redis) InsertItem(item Item) error {
	panic("not implemented")
}

func (redis *Redis) BatchInsertItem(items []Item) error {
	panic("not implemented")
}

func (redis *Redis) DeleteItem(itemId string) error {
	panic("not implemented")
}

func (redis *Redis) GetItem(itemId string) (Item, error) {
	panic("not implemented")
}

func (redis *Redis) GetItems(n int, offset int) ([]Item, error) {
	panic("not implemented")
}

func (redis *Redis) GetItemFeedback(itemId string) ([]Feedback, error) {
	panic("not implemented")
}

func (redis *Redis) GetLabelItems(label string) ([]Item, error) {
	panic("not implemented")
}

func (redis *Redis) GetLabels() ([]string, error) {
	panic("not implemented")
}

func (redis *Redis) InsertUser(user User) error {
	panic("not implemented")
}

func (redis *Redis) DeleteUser(userId string) error {
	panic("not implemented")
}

func (redis *Redis) GetUser(userId string) (User, error) {
	panic("not implemented")
}

func (redis *Redis) GetUsers() ([]User, error) {
	panic("not implemented")
}

func (redis *Redis) GetUserFeedback(userId string) ([]Feedback, error) {
	panic("not implemented")
}

func (redis *Redis) InsertUserIgnore(userId string, items []string) error {
	panic("not implemented")
}

func (redis *Redis) GetUserIgnore(userId string) ([]string, error) {
	panic("not implemented")
}

func (redis *Redis) CountUserIgnore(userId string) (int, error) {
	panic("not implemented")
}

func (redis *Redis) InsertFeedback(feedback Feedback) error {
	panic("not implemented")
}

func (redis *Redis) BatchInsertFeedback(feedback []Feedback) error {
	panic("not implemented")
}

func (redis *Redis) GetFeedback() ([]Feedback, error) {
	panic("not implemented")
}

func (redis *Redis) GetString(name string) (string, error) {
	panic("not implemented")
}

func (redis *Redis) SetString(name string, val string) error {
	panic("not implemented")
}

func (redis *Redis) GetInt(name string) (int, error) {
	panic("not implemented")
}

func (redis *Redis) SetInt(name string, val int) error {
	panic("not implemented")
}

func (redis *Redis) SetNeighbors(itemId string, items []RecommendedItem) error {
	panic("not implemented")
}

func (redis *Redis) SetPop(label string, items []RecommendedItem) error {
	panic("not implemented")
}

func (redis *Redis) SetLatest(label string, items []RecommendedItem) error {
	panic("not implemented")
}

func (redis *Redis) SetRecommend(userId string, items []RecommendedItem) error {
	panic("not implemented")
}

func (redis *Redis) GetNeighbors(itemId string, n int, offset int) ([]RecommendedItem, error) {
	panic("not implemented")
}

func (redis *Redis) GetPop(label string, n int, offset int) ([]RecommendedItem, error) {
	panic("not implemented")
}

func (redis *Redis) GetLatest(label string, n int, offset int) ([]RecommendedItem, error) {
	panic("not implemented")
}

func (redis *Redis) GetRecommend(userId string, n int, offset int) ([]RecommendedItem, error) {
	panic("not implemented")
}
