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
	"os"
	"testing"
	"time"

	client "github.com/gorse-io/gorse-go"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

var (
	serverEndpoint    string
	dashboardEndpoint string
)

func init() {
	serverEndpoint = os.Getenv("GORSE_SERVER_ENDPOINT")
	dashboardEndpoint = os.Getenv("GORSE_DASHBOARD_ENDPOINT")
}

type GorseClientTestSuite struct {
	suite.Suite
	client *client.GorseClient
}

func (suite *GorseClientTestSuite) SetupSuite() {
	if serverEndpoint == "" || dashboardEndpoint == "" {
		suite.T().Skip("GORSE_SERVER_ENDPOINT or GORSE_DASHBOARD_ENDPOINT is not set")
	}
	suite.client = client.NewGorseClient(serverEndpoint, "")
}

func (suite *GorseClientTestSuite) TestUsers() {
	ctx := context.Background()

	cursor, err := suite.client.GetUsers(ctx, 3, "")
	suite.NoError(err)
	suite.NotEmpty(cursor.Cursor)
	if suite.Len(cursor.Users, 3) {
		suite.Equal(client.User{
			UserId: "1",
			Labels: map[string]any{
				"age":        float64(24),
				"gender":     "M",
				"occupation": "technician",
				"zip_code":   "85711",
			},
		}, cursor.Users[0])
		suite.Equal(client.User{
			UserId: "10",
			Labels: map[string]any{
				"age":        float64(53),
				"gender":     "M",
				"occupation": "lawyer",
				"zip_code":   "90703",
			},
		}, cursor.Users[1])
		suite.Equal(client.User{
			UserId: "100",
			Labels: map[string]any{
				"age":        float64(36),
				"gender":     "M",
				"occupation": "executive",
				"zip_code":   "90254",
			},
		}, cursor.Users[2])
	}

	user := client.User{
		UserId:  "1000",
		Labels:  map[string]any{"gender": "M", "occupation": "engineer"},
		Comment: "zhenghaoz",
	}
	rowAffected, err := suite.client.InsertUser(ctx, user)
	suite.NoError(err)
	suite.Equal(1, rowAffected.RowAffected)
	resp, err := suite.client.GetUser(ctx, "1000")
	suite.NoError(err)
	suite.Equal(user, resp)

	patch := client.UserPatch{
		Comment: proto.String("hongmi"),
	}
	rowAffected, err = suite.client.UpdateUser(ctx, user.UserId, patch)
	suite.NoError(err)
	suite.Equal(1, rowAffected.RowAffected)
	resp, err = suite.client.GetUser(ctx, "1000")
	suite.NoError(err)
	suite.Equal("hongmi", resp.Comment)

	deleteAffect, err := suite.client.DeleteUser(ctx, "1000")
	suite.NoError(err)
	suite.Equal(1, deleteAffect.RowAffected)
	_, err = suite.client.GetUser(ctx, "1000")
	suite.Equal("1000: user not found", err.Error())
}

func (suite *GorseClientTestSuite) TestItems() {
	ctx := context.TODO()
	items, err := suite.client.GetItems(ctx, 3, "")
	suite.NoError(err)
	suite.NotEmpty(items.Cursor)
	if suite.Len(items.Items, 3) {
		suite.Equal("1", items.Items[0].ItemId)
		suite.Equal([]string{"Animation", "Children's", "Comedy"}, items.Items[0].Categories)
		suite.Equal(time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC), items.Items[0].Timestamp)
		suite.Equal("Toy Story (1995)", items.Items[0].Comment)
		suite.Equal("10", items.Items[1].ItemId)
		suite.Equal([]string{"Drama", "War"}, items.Items[1].Categories)
		suite.Equal(time.Date(1996, 1, 22, 0, 0, 0, 0, time.UTC), items.Items[1].Timestamp)
		suite.Equal("Richard III (1995)", items.Items[1].Comment)
		suite.Equal("100", items.Items[2].ItemId)
		suite.Equal([]string{"Crime", "Drama", "Thriller"}, items.Items[2].Categories)
		suite.Equal(time.Date(1997, 2, 14, 0, 0, 0, 0, time.UTC), items.Items[2].Timestamp)
		suite.Equal("Fargo (1996)", items.Items[2].Comment)
	}

	item := client.Item{
		ItemId:   "2000",
		IsHidden: true,
		Labels: map[string]any{
			"embedding": []any{0.1, 0.2, 0.3},
		},
		Categories: []string{"Comedy", "Animation"},
		Timestamp:  time.Now().UTC(),
		Comment:    "Minions (2015)",
	}
	rowAffected, err := suite.client.InsertItem(ctx, item)
	suite.NoError(err)
	suite.Equal(1, rowAffected.RowAffected)
	resp, err := suite.client.GetItem(ctx, "2000")
	suite.NoError(err)
	suite.Equal(item, resp)

	patch := client.ItemPatch{
		Comment: proto.String("小黄人 (2015)"),
	}
	rowAffected, err = suite.client.UpdateItem(ctx, item.ItemId, patch)
	suite.NoError(err)
	suite.Equal(1, rowAffected.RowAffected)
	resp, err = suite.client.GetItem(ctx, "2000")
	suite.NoError(err)
	suite.Equal("小黄人 (2015)", resp.Comment)

	deleteAffect, err := suite.client.DeleteItem(ctx, "2000")
	suite.NoError(err)
	suite.Equal(1, deleteAffect.RowAffected)
	_, err = suite.client.GetItem(ctx, "2000")
	suite.Equal("2000: item not found", err.Error())
}

func (suite *GorseClientTestSuite) TestFeedback() {
	ctx := context.TODO()
	timestamp := time.Unix(1660459054, 0).UTC()
	userId := "800"
	insertFeedbackResp, err := suite.client.InsertFeedback(ctx, []client.Feedback{{
		FeedbackType: "like",
		UserId:       userId,
		Timestamp:    timestamp,
		ItemId:       "200",
	}})
	suite.NoError(err)
	suite.Equal(1, insertFeedbackResp.RowAffected)

	insertFeedbacksResp, err := suite.client.InsertFeedback(ctx, []client.Feedback{{
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
	suite.NoError(err)
	suite.Equal(2, insertFeedbacksResp.RowAffected)

	feedbacks, err := suite.client.ListFeedbacks(ctx, "read", userId)
		suite.NoError(err)
	suite.ElementsMatch([]client.Feedback{
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

func TestGorseClientTestSuite(t *testing.T) {
	suite.Run(t, new(GorseClientTestSuite))
}
