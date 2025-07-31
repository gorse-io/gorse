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
	"strconv"
	"testing"
	"time"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/assert"
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
	popular := NewPopular(0, 10, timestamp)
	for i := 0; i < 100; i++ {
		item := data.Item{ItemId: strconv.Itoa(i)}
		feedback := make([]data.Feedback, i)
		popular.Push(item, feedback)
	}
	scores := popular.PopAll()
	assert.Len(t, scores, 10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(99-i), scores[i].Id)
		assert.Equal(t, float64(99-i), scores[i].Score)
	}
}

func TestPopularWindow(t *testing.T) {
	// Create popular recommender
	timestamp := time.Now()
	popular := NewPopular(time.Hour, 10, timestamp)

	// Add items
	for i := 0; i < 100; i++ {
		item := data.Item{ItemId: strconv.Itoa(i), Timestamp: timestamp.Add(time.Second - time.Hour)}
		feedback := make([]data.Feedback, i)
		popular.Push(item, feedback)
	}

	// Add outdated items
	for i := 100; i < 110; i++ {
		item := data.Item{ItemId: strconv.Itoa(i), Timestamp: timestamp.Add(-time.Hour)}
		feedback := make([]data.Feedback, i)
		popular.Push(item, feedback)
	}

	// Check result
	scores := popular.PopAll()
	assert.Len(t, scores, 10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(99-i), scores[i].Id)
		assert.Equal(t, float64(99-i), scores[i].Score)
	}
}

func TestFilter(t *testing.T) {
	timestamp := time.Now()
	latest, err := NewNonPersonalized(config.NonPersonalizedConfig{
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

func TestMostStarredWeekly(t *testing.T) {
	// Create non-personalized recommender
	timestamp := time.Now()
	mostStarredWeekly, err := NewNonPersonalized(config.NonPersonalizedConfig{
		Name:   "most_starred_weekly",
		Score:  "count(feedback, .FeedbackType == 'star')",
		Filter: "(now() - item.Timestamp).Hours() < 168",
	}, 10, timestamp)
	assert.NoError(t, err)

	// Add items
	for i := 0; i < 100; i++ {
		item := data.Item{ItemId: strconv.Itoa(i), Timestamp: timestamp.Add(-167 * time.Hour)}
		var feedback []data.Feedback
		for j := 0; j < i; j++ {
			feedback = append(feedback, data.Feedback{
				FeedbackKey: data.FeedbackKey{
					FeedbackType: "star",
					UserId:       strconv.Itoa(j),
					ItemId:       strconv.Itoa(i),
				},
				Timestamp: timestamp,
			})
			feedback = append(feedback, data.Feedback{
				FeedbackKey: data.FeedbackKey{
					FeedbackType: "like",
					UserId:       strconv.Itoa(j),
					ItemId:       strconv.Itoa(i),
				},
				Timestamp: timestamp,
			})
		}
		mostStarredWeekly.Push(item, feedback)
	}

	// Add outdated items
	for i := 100; i < 110; i++ {
		item := data.Item{ItemId: strconv.Itoa(i), Timestamp: timestamp.Add(-168 * time.Hour)}
		var feedback []data.Feedback
		for j := 0; j < i; j++ {
			feedback = append(feedback, data.Feedback{
				FeedbackKey: data.FeedbackKey{
					FeedbackType: "star",
					UserId:       strconv.Itoa(j),
					ItemId:       strconv.Itoa(i),
				},
				Timestamp: timestamp.Add(-time.Hour * 169),
			})
		}
		mostStarredWeekly.Push(item, feedback)
	}

	// Check result
	scores := mostStarredWeekly.PopAll()
	assert.Len(t, scores, 10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(99-i), scores[i].Id)
		assert.Equal(t, float64(99-i), scores[i].Score)
	}
}
