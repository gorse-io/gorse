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

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const (
	GORSE_ENDPOINT = "http://127.0.0.1:8087"
)

func TestFeedback(t *testing.T) {
	client := NewGorseClient(GORSE_ENDPOINT, "zhenghaoz")

	timestamp := time.Unix(1660459054, 0).UTC().Format(time.RFC3339)
	userId := "800"
	insertFeedbackResp, err := client.InsertFeedback([]Feedback{{
		FeedbackType: "like",
		UserId:       userId,
		Timestamp:    timestamp,
		ItemId:       "200",
	}})
	assert.NoError(t, err)
	assert.Equal(t, 1, insertFeedbackResp.RowAffected)

	insertFeedbacksResp, err := client.InsertFeedback([]Feedback{{
		FeedbackType: "read",
		UserId:       userId,
		Timestamp:    timestamp,
		ItemId:       "300",
	}, {
		FeedbackType: "read",
		UserId:       userId,
		Timestamp:    timestamp,
		ItemId:       "400",
	}})
	assert.NoError(t, err)
	assert.Equal(t, 2, insertFeedbacksResp.RowAffected)

	feedbacks, err := client.ListFeedbacks("read", userId)
	assert.NoError(t, err)
	assert.Equal(t, []Feedback{
		{
			FeedbackType: "read",
			UserId:       userId,
			Timestamp:    timestamp,
			ItemId:       "300",
		}, {
			FeedbackType: "read",
			UserId:       userId,
			Timestamp:    timestamp,
			ItemId:       "400",
		},
	}, feedbacks)
}

func TestRecommend(t *testing.T) {
	client := NewGorseClient(GORSE_ENDPOINT, "zhenghaoz")
	r := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   0,
	})
	r.ZAddArgs(context.Background(), "offline_recommend/100", redis.ZAddArgs{
		Members: []redis.Z{
			{
				Score:  1,
				Member: "1",
			},
			{
				Score:  2,
				Member: "2",
			},
			{
				Score:  3,
				Member: "3",
			},
		},
	})
	resp, err := client.GetRecommend("100", "", 10)
	assert.NoError(t, err)
	assert.Equal(t, []string{"3", "2", "1"}, resp)
}

func TestSessionRecommend(t *testing.T) {
	client := NewGorseClient(GORSE_ENDPOINT, "zhenghaoz")
	r := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   0,
	})
	ctx := context.Background()
	r.ZAddArgs(ctx, "item_neighbors/1", redis.ZAddArgs{
		Members: []redis.Z{
			{
				Score:  100000,
				Member: "2",
			},
			{
				Score:  1,
				Member: "9",
			},
		},
	})
	r.ZAddArgs(ctx, "item_neighbors/2", redis.ZAddArgs{
		Members: []redis.Z{
			{
				Score:  100000,
				Member: "3",
			},
			{
				Score:  1,
				Member: "8",
			},
			{
				Score:  1,
				Member: "9",
			},
		},
	})
	r.ZAddArgs(ctx, "item_neighbors/3", redis.ZAddArgs{
		Members: []redis.Z{
			{
				Score:  100000,
				Member: "4",
			},
			{
				Score:  1,
				Member: "7",
			},
			{
				Score:  1,
				Member: "8",
			},
			{
				Score:  1,
				Member: "9",
			},
		},
	})
	r.ZAddArgs(ctx, "item_neighbors/4", redis.ZAddArgs{
		Members: []redis.Z{
			{
				Score:  100000,
				Member: "1",
			},
			{
				Score:  1,
				Member: "6",
			},
			{
				Score:  1,
				Member: "7",
			},
			{
				Score:  1,
				Member: "8",
			},
			{
				Score:  1,
				Member: "9",
			},
		},
	})

	feedbackType := "like"
	userId := "0"
	timestamp := time.Unix(1660459054, 0).UTC().Format(time.RFC3339)
	resp, err := client.SessionRecommend([]Feedback{
		{
			FeedbackType: feedbackType,
			UserId:       userId,
			ItemId:       "1",
			Timestamp:    timestamp,
		},
		{
			FeedbackType: feedbackType,
			UserId:       userId,
			ItemId:       "2",
			Timestamp:    timestamp,
		},
		{
			FeedbackType: feedbackType,
			UserId:       userId,
			ItemId:       "3",
			Timestamp:    timestamp,
		},
		{
			FeedbackType: feedbackType,
			UserId:       userId,
			ItemId:       "4",
			Timestamp:    timestamp,
		},
	}, 3)
	assert.NoError(t, err)
	assert.Equal(t, []Score{
		{
			Id:    "9",
			Score: 4,
		},
		{
			Id:    "8",
			Score: 3,
		},
		{
			Id:    "7",
			Score: 2,
		},
	}, resp)
}

func TestNeighbors(t *testing.T) {
	client := NewGorseClient(GORSE_ENDPOINT, "zhenghaoz")
	r := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   0,
	})
	ctx := context.Background()
	r.ZAddArgs(ctx, "item_neighbors/100", redis.ZAddArgs{
		Members: []redis.Z{
			{
				Score:  1,
				Member: "1",
			},
			{
				Score:  2,
				Member: "2",
			},
			{
				Score:  3,
				Member: "3",
			},
		},
	})

	itemId := "100"
	resp, err := client.GetNeighbors(itemId, 3)
	assert.NoError(t, err)
	assert.Equal(t, []Score{
		{
			Id:    "3",
			Score: 3,
		}, {
			Id:    "2",
			Score: 2,
		}, {
			Id:    "1",
			Score: 1,
		},
	}, resp)
}

func TestUsers(t *testing.T) {

	client := NewGorseClient(GORSE_ENDPOINT, "zhenghaoz")
	user := User{
		UserId:    "100",
		Labels:    []string{"a", "b", "c"},
		Subscribe: []string{"d", "e"},
		Comment:   "comment",
	}
	rowAffected, err := client.InsertUser(user)
	assert.NoError(t, err)
	assert.Equal(t, 1, rowAffected.RowAffected)

	userResp, err := client.GetUser("100")
	assert.NoError(t, err)
	assert.Equal(t, user, *userResp)

	deleteAffect, err := client.DeleteUser("100")
	assert.NoError(t, err)
	assert.Equal(t, 1, deleteAffect.RowAffected)

	_, err = client.GetUser("100")
	assert.Equal(t, "100: user not found", err.Error())
}

func TestItems(t *testing.T) {
	client := NewGorseClient(GORSE_ENDPOINT, "zhenghaoz")

	timestamp := time.Unix(1660459054, 0).UTC().Format(time.RFC3339)
	item := Item{
		ItemId:     "100",
		IsHidden:   true,
		Labels:     []string{"a", "b", "c"},
		Categories: []string{"d", "e"},
		Timestamp:  timestamp,
		Comment:    "comment",
	}
	rowAffected, err := client.InsertItem(item)
	assert.NoError(t, err)
	assert.Equal(t, 1, rowAffected.RowAffected)

	itemResp, err := client.GetItem("100")
	assert.NoError(t, err)
	assert.Equal(t, item, *itemResp)

	deleteAffect, err := client.DeleteItem("100")
	assert.NoError(t, err)
	assert.Equal(t, 1, deleteAffect.RowAffected)

	_, err = client.GetItem("100")
	assert.Equal(t, "100: item not found", err.Error())
}
