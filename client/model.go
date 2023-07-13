// Copyright 2022 gorse Project Authors
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

package client

import "time"

type Feedback struct {
	FeedbackType string `json:"FeedbackType"`
	UserId       string `json:"UserId"`
	ItemId       string `json:"ItemId"`
	Timestamp    string `json:"Timestamp"`
}

type Feedbacks struct {
	Cursor   string     `json:"Cursor"`
	Feedback []Feedback `json:"Feedback"`
}

type ErrorMessage string

func (e ErrorMessage) Error() string {
	return string(e)
}

type RowAffected struct {
	RowAffected int `json:"RowAffected"`
}

type Score struct {
	Id    string  `json:"Id"`
	Score float32 `json:"Score"`
}

type User struct {
	UserId    string   `json:"UserId"`
	Labels    []string `json:"Labels"`
	Subscribe []string `json:"Subscribe"`
	Comment   string   `json:"Comment"`
}

type Users struct {
	Cursor string `json:"Cursor"`
	Users  []User `json:"Users"`
}

type UserPatch struct {
	Labels    []string
	Subscribe []string
	Comment   *string
}

type Item struct {
	ItemId     string   `json:"ItemId"`
	IsHidden   bool     `json:"IsHidden"`
	Labels     []string `json:"Labels"`
	Categories []string `json:"Categories"`
	Timestamp  string   `json:"Timestamp"`
	Comment    string   `json:"Comment"`
}

type Items struct {
	Cursor string `json:"Cursor"`
	Items  []Item `json:"Items"`
}

type ItemPatch struct {
	IsHidden   *bool
	Categories []string
	Timestamp  *time.Time
	Labels     []string
	Comment    *string
}

type Health struct {
	CacheStoreConnected bool   `json:"CacheStoreConnected"`
	CacheStoreError     string `json:"CacheStoreError"`
	DataStoreConnected  bool   `json:"DataStoreConnected"`
	DataStoreError      string `json:"DataStoreError"`
	Ready               bool   `json:"Ready"`
}
