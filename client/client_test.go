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
		Comment: new("hongmi"),
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
		Timestamp:  time.Now().UTC().Truncate(time.Second),
		Comment:    "Minions (2015)",
	}
	rowAffected, err := suite.client.InsertItem(ctx, item)
	suite.NoError(err)
	suite.Equal(1, rowAffected.RowAffected)
	resp, err := suite.client.GetItem(ctx, "2000")
	suite.NoError(err)
	suite.Equal(item, resp)

	patch := client.ItemPatch{
		Comment: new("小黄人 (2015)"),
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
	ctx := context.Background()
	_, err := suite.client.InsertUser(ctx, client.User{UserId: "2000"})
	suite.NoError(err)

	feedback := []client.Feedback{
		{
			FeedbackType: "watch",
			UserId:       "2000",
			ItemId:       "1",
			Value:        1.0,
			Timestamp:    time.Now().UTC().Truncate(time.Second),
		},
		{
			FeedbackType: "watch",
			UserId:       "2000",
			ItemId:       "1060",
			Value:        2.0,
			Timestamp:    time.Now().UTC().Truncate(time.Second),
		},
		{
			FeedbackType: "watch",
			UserId:       "2000",
			ItemId:       "11",
			Value:        3.0,
			Timestamp:    time.Now().UTC().Truncate(time.Second),
		},
	}
	for _, fb := range feedback {
		_, err := suite.client.DeleteFeedbacks(ctx, fb.UserId, fb.ItemId)
		suite.NoError(err)
	}
	_, err = suite.client.InsertFeedback(ctx, feedback)
	suite.NoError(err)

	userFeedback, err := suite.client.ListFeedbacks(ctx, "watch", "2000")
	suite.NoError(err)
	suite.Equal(feedback, userFeedback)

	_, err = suite.client.DeleteFeedback(ctx, "watch", "2000", "1")
	suite.NoError(err)
	userFeedback, err = suite.client.ListFeedbacks(ctx, "watch", "2000")
	suite.NoError(err)
	suite.Equal([]client.Feedback{feedback[1], feedback[2]}, userFeedback)
}

func (suite *GorseClientTestSuite) TestLatest() {
	ctx := context.Background()
	items, err := suite.client.GetLatestItems(ctx, "", "", 3, 0)
	suite.NoError(err)
	if suite.Len(items, 3) {
		suite.Equal("315", items[0].Id)
		suite.Equal("1432", items[1].Id)
		suite.Equal("918", items[2].Id)
	}
}

func (suite *GorseClientTestSuite) TestItemToItem() {
	ctx := context.Background()
	neighbors, err := suite.client.GetNeighbors(ctx, "1", 3)
	suite.NoError(err)
	if suite.Len(neighbors, 3) {
		suite.Equal("1060", neighbors[0].Id)
		suite.Equal("404", neighbors[1].Id)
		suite.Equal("1219", neighbors[2].Id)
	}
}

func (suite *GorseClientTestSuite) TestRecommend() {
	ctx := context.Background()
	_, err := suite.client.InsertUser(ctx, client.User{UserId: "3000"})
	suite.NoError(err)
	recommendations, err := suite.client.GetRecommend(ctx, "3000", "", 3, 0)
	suite.NoError(err)
	suite.Len(recommendations, 3)
	if suite.Len(recommendations, 3) {
		suite.Equal("315", recommendations[0])
		suite.Equal("1432", recommendations[1])
		suite.Equal("918", recommendations[2])
	}
}

func TestGorseClientTestSuite(t *testing.T) {
	suite.Run(t, new(GorseClientTestSuite))
}
