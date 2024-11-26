// Copyright 2024 gorse Project Authors
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

package logics

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/data"
	"strconv"
	"testing"
	"time"
)

func TestLatest(t *testing.T) {
	timestamp := time.Now()
	latest := NewLatest(10, timestamp)
	for i := 0; i < 100; i++ {
		item := data.Item{ItemId: strconv.Itoa(i), Timestamp: timestamp.Add(time.Duration(-i) * time.Second)}
		latest.Push(item, nil)
	}
	scores := latest.PopAll()
	assert.Len(t, scores, 10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(i), scores[i].Id)
		assert.Equal(t, float64(timestamp.Add(time.Duration(-i)*time.Second).Unix()), scores[i].Score)
		assert.Equal(t, timestamp, scores[i].Timestamp)
	}
}

func TestPopular(t *testing.T) {
	timestamp := time.Now()
	popular := NewPopular(10, timestamp)
	for i := 0; i < 100; i++ {
		item := data.Item{ItemId: strconv.Itoa(i)}
		var feedback []data.Feedback
		for j := 0; j < i; j++ {
			feedback = append(feedback, data.Feedback{FeedbackKey: data.FeedbackKey{UserId: strconv.Itoa(j)}, Timestamp: timestamp})
		}
		popular.Push(item, feedback)
	}
	scores := popular.PopAll()
	assert.Len(t, scores, 10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(99-i), scores[i].Id)
		assert.Equal(t, float64(99-i), scores[i].Score)
	}
}

func TestFilter(t *testing.T) {
	timestamp := time.Now()
	latest, err := NewLeaderBoard(config.NonPersonalizedConfig{
		Name:   "latest",
		Score:  "item.Timestamp.Unix()",
		Filter: "!item.IsHidden",
	}, 10, timestamp)
	assert.NoError(t, err)
	for i := 0; i < 100; i++ {
		item := data.Item{ItemId: strconv.Itoa(i), Timestamp: timestamp.Add(time.Duration(-i) * time.Second)}
		item.IsHidden = i < 10
		latest.Push(item, nil)
	}
	scores := latest.PopAll()
	assert.Len(t, scores, 10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(i+10), scores[i].Id)
		assert.Equal(t, float64(timestamp.Add(time.Duration(-i-10)*time.Second).Unix()), scores[i].Score)
		assert.Equal(t, timestamp, scores[i].Timestamp)
	}
}

func TestHidden(t *testing.T) {
	timestamp := time.Now()
	latest := NewLatest(10, timestamp)
	for i := 0; i < 100; i++ {
		item := data.Item{ItemId: strconv.Itoa(i), Timestamp: timestamp.Add(time.Duration(-i) * time.Second)}
		item.IsHidden = i < 10
		latest.Push(item, nil)
	}
	scores := latest.PopAll()
	assert.Len(t, scores, 10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(i+10), scores[i].Id)
		assert.Equal(t, float64(timestamp.Add(time.Duration(-i-10)*time.Second).Unix()), scores[i].Score)
		assert.Equal(t, timestamp, scores[i].Timestamp)
	}
}
