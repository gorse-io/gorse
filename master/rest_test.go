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

package master

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/go-viper/mapstructure/v2"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/zhenghaoz/gorse/common/expression"
	"github.com/zhenghaoz/gorse/common/mock"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/model/ctr"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"github.com/zhenghaoz/gorse/storage/meta"
	"google.golang.org/protobuf/proto"
)

const (
	mockMasterUsername = "admin"
	mockMasterPassword = "pass"
)

func marshal(t *testing.T, v interface{}) string {
	s, err := json.Marshal(v)
	assert.NoError(t, err)
	return string(s)
}

func marshalJSONLines[T any](t *testing.T, v []T) string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	for _, item := range v {
		err := encoder.Encode(item)
		assert.NoError(t, err)
	}
	return buf.String()
}

func convertToMapStructure(t *testing.T, v interface{}) map[string]interface{} {
	var m map[string]interface{}
	err := mapstructure.Decode(v, &m)
	assert.NoError(t, err)
	return m
}

type MasterAPITestSuite struct {
	suite.Suite
	Master
	handler      *restful.Container
	openAIServer *mock.OpenAIServer
	cookie       string
}

func (suite *MasterAPITestSuite) SetupTest() {
	// open database
	var err error
	suite.Settings = config.NewSettings()
	suite.metaStore, err = meta.Open(fmt.Sprintf("sqlite://%s/meta.db", suite.T().TempDir()), suite.Config.Master.MetaTimeout)
	suite.NoError(err)
	suite.DataClient, err = data.Open(fmt.Sprintf("sqlite://%s/data.db", suite.T().TempDir()), "")
	suite.NoError(err)
	suite.CacheClient, err = cache.Open(fmt.Sprintf("sqlite://%s/cache.db", suite.T().TempDir()), "")
	suite.NoError(err)
	// init database
	err = suite.metaStore.Init()
	suite.NoError(err)
	err = suite.DataClient.Init()
	suite.NoError(err)
	err = suite.CacheClient.Init()
	suite.NoError(err)
	// create server
	suite.Config = config.GetDefaultConfig()
	suite.Config.Master.DashboardUserName = mockMasterUsername
	suite.Config.Master.DashboardPassword = mockMasterPassword
	suite.WebService = new(restful.WebService)
	suite.CreateWebService()
	suite.RestServer.CreateWebService()
	// create handler
	suite.handler = restful.NewContainer()
	suite.handler.Add(suite.WebService)
	// creat mock AI server
	suite.openAIServer = mock.NewOpenAIServer()
	go func() {
		_ = suite.openAIServer.Start()
	}()
	suite.openAIServer.Ready()
	clientConfig := openai.DefaultConfig(suite.openAIServer.AuthToken())
	clientConfig.BaseURL = suite.openAIServer.BaseURL()
	suite.openAIClient = openai.NewClientWithConfig(clientConfig)
	// login
	req, err := http.NewRequest("POST", "/login",
		strings.NewReader(fmt.Sprintf("user_name=%s&password=%s", mockMasterUsername, mockMasterPassword)))
	suite.NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	suite.login(resp, req)
	suite.Equal(http.StatusFound, resp.Code)
	suite.cookie = resp.Header().Get("Set-Cookie")
}

func (suite *MasterAPITestSuite) TearDownTest() {
	err := suite.metaStore.Close()
	suite.NoError(err)
	err = suite.DataClient.Close()
	suite.NoError(err)
	err = suite.CacheClient.Close()
	suite.NoError(err)
	err = suite.openAIServer.Close()
	suite.NoError(err)
}

func (suite *MasterAPITestSuite) TestExportUsers() {
	ctx := context.Background()
	// insert users
	users := []data.User{
		{UserId: "1", Labels: map[string]any{"gender": "male", "job": "engineer"}},
		{UserId: "2", Labels: map[string]any{"gender": "male", "job": "lawyer"}},
		{UserId: "3", Labels: map[string]any{"gender": "female", "job": "teacher"}},
	}
	err := suite.DataClient.BatchInsertUsers(ctx, users)
	suite.NoError(err)
	// send request
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Header.Set("Cookie", suite.cookie)
	w := httptest.NewRecorder()
	suite.importExportUsers(w, req)
	suite.Equal(http.StatusOK, w.Result().StatusCode)
	suite.Equal("application/jsonl", w.Header().Get("Content-Type"))
	suite.Equal("attachment;filename=users.jsonl", w.Header().Get("Content-Disposition"))
	suite.Equal(marshalJSONLines(suite.T(), users), w.Body.String())
}

func (suite *MasterAPITestSuite) TestExportItems() {
	ctx := context.Background()
	// insert items
	items := []data.Item{
		{
			ItemId:     "1",
			IsHidden:   false,
			Categories: []string{"x"},
			Timestamp:  time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC),
			Labels:     map[string]any{"genre": []string{"comedy", "sci-fi"}},
			Comment:    "o,n,e",
		},
		{
			ItemId:     "2",
			IsHidden:   false,
			Categories: []string{"x", "y"},
			Timestamp:  time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC),
			Labels:     map[string]any{"genre": []string{"documentary", "sci-fi"}},
			Comment:    "t\r\nw\r\no",
		},
		{
			ItemId:     "3",
			IsHidden:   true,
			Categories: nil,
			Timestamp:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
			Labels:     nil,
			Comment:    "\"three\"",
		},
	}
	err := suite.DataClient.BatchInsertItems(ctx, items)
	suite.NoError(err)
	// send request
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Header.Set("Cookie", suite.cookie)
	w := httptest.NewRecorder()
	suite.importExportItems(w, req)
	suite.Equal(http.StatusOK, w.Result().StatusCode)
	suite.Equal("application/jsonl", w.Header().Get("Content-Type"))
	suite.Equal("attachment;filename=items.jsonl", w.Header().Get("Content-Disposition"))
	suite.Equal(marshalJSONLines(suite.T(), items), w.Body.String())
}

func (suite *MasterAPITestSuite) TestExportFeedback() {
	ctx := context.Background()
	// insert feedback
	feedbacks := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "read", UserId: "2", ItemId: "6"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "share", UserId: "1", ItemId: "4"}},
	}
	err := suite.DataClient.BatchInsertFeedback(ctx, feedbacks, true, true, true)
	suite.NoError(err)
	// send request
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Header.Set("Cookie", suite.cookie)
	w := httptest.NewRecorder()
	suite.importExportFeedback(w, req)
	suite.Equal(http.StatusOK, w.Result().StatusCode)
	suite.Equal("application/jsonl", w.Header().Get("Content-Type"))
	suite.Equal("attachment;filename=feedback.jsonl", w.Header().Get("Content-Disposition"))
	suite.Equal(marshalJSONLines(suite.T(), feedbacks), w.Body.String())
}

func (suite *MasterAPITestSuite) TestImportUsers() {
	ctx := context.Background()
	// send request
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	file, err := writer.CreateFormFile("file", "users.jsonl")
	suite.NoError(err)
	_, err = file.Write([]byte(`{"UserId":"1","Labels":{"性别":"男","职业":"工程师"}}
{"UserId":"2","Labels":{"性别":"男","职业":"律师"}}
{"UserId":"3","Labels":{"性别":"女","职业":"教师"}}`))
	suite.NoError(err)
	err = writer.Close()
	suite.NoError(err)
	req := httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", suite.cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	suite.importExportUsers(w, req)
	// check
	suite.Equal(http.StatusOK, w.Result().StatusCode)
	suite.JSONEq(marshal(suite.T(), server.Success{RowAffected: 3}), w.Body.String())
	_, items, err := suite.DataClient.GetUsers(ctx, "", 100)
	suite.NoError(err)
	suite.Equal([]data.User{
		{UserId: "1", Labels: map[string]any{"性别": "男", "职业": "工程师"}},
		{UserId: "2", Labels: map[string]any{"性别": "男", "职业": "律师"}},
		{UserId: "3", Labels: map[string]any{"性别": "女", "职业": "教师"}},
	}, items)
}

func (suite *MasterAPITestSuite) TestImportItems() {
	ctx := context.Background()
	// send request
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	file, err := writer.CreateFormFile("file", "items.jsonl")
	suite.NoError(err)
	_, err = file.Write([]byte(`{"ItemId":"1","IsHidden":false,"Categories":["x"],"Timestamp":"2020-01-01 01:01:01.000000001 +0000 UTC","Labels":{"类型":["喜剧","科幻"]},"Comment":"one"}
{"ItemId":"2","IsHidden":false,"Categories":["x","y"],"Timestamp":"2021-01-01 01:01:01.000000001 +0000 UTC","Labels":{"类型":["卡通","科幻"]},"Comment":"two"}
{"ItemId":"3","IsHidden":true,"Timestamp":"2022-01-01 01:01:01.000000001 +0000 UTC","Comment":"three"}`))
	suite.NoError(err)
	err = writer.Close()
	suite.NoError(err)
	req := httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", suite.cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	suite.importExportItems(w, req)
	// check
	suite.Equal(http.StatusOK, w.Result().StatusCode)
	suite.JSONEq(marshal(suite.T(), server.Success{RowAffected: 3}), w.Body.String())
	_, items, err := suite.DataClient.GetItems(ctx, "", 100, nil)
	suite.NoError(err)
	suite.Equal([]data.Item{
		{
			ItemId:     "1",
			IsHidden:   false,
			Categories: []string{"x"},
			Timestamp:  time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC),
			Labels:     map[string]any{"类型": []any{"喜剧", "科幻"}},
			Comment:    "one"},
		{
			ItemId:     "2",
			IsHidden:   false,
			Categories: []string{"x", "y"},
			Timestamp:  time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC),
			Labels:     map[string]any{"类型": []any{"卡通", "科幻"}},
			Comment:    "two",
		},
		{
			ItemId:     "3",
			IsHidden:   true,
			Categories: nil,
			Timestamp:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
			Labels:     nil,
			Comment:    "three",
		},
	}, items)
}

func (suite *MasterAPITestSuite) TestImportFeedback() {
	// send request
	ctx := context.Background()
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	file, err := writer.CreateFormFile("file", "feedback.jsonl")
	suite.NoError(err)
	_, err = file.Write([]byte(`{"FeedbackType":"click","UserId":"0","ItemId":"2","Timestamp":"0001-01-01 00:00:00 +0000 UTC"}
{"FeedbackType":"read","UserId":"2","ItemId":"6","Timestamp":"0001-01-01 00:00:00 +0000 UTC"}
{"FeedbackType":"share","UserId":"1","ItemId":"4","Timestamp":"0001-01-01 00:00:00 +0000 UTC"}`))
	suite.NoError(err)
	err = writer.Close()
	suite.NoError(err)
	req := httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", suite.cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	suite.importExportFeedback(w, req)
	// check
	suite.Equal(http.StatusOK, w.Result().StatusCode)
	suite.JSONEq(marshal(suite.T(), server.Success{RowAffected: 3}), w.Body.String())
	_, feedback, err := suite.DataClient.GetFeedback(ctx, "", 100, nil, lo.ToPtr(time.Now()))
	suite.NoError(err)
	suite.Equal([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "read", UserId: "2", ItemId: "6"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "share", UserId: "1", ItemId: "4"}},
	}, feedback)
}

func (suite *MasterAPITestSuite) TestGetCluster() {
	// add nodes
	serverNode := &meta.Node{
		UUID:       "alan turnin",
		Hostname:   "192.168.1.100",
		Type:       protocol.NodeType_Server.String(),
		Version:    "server_version",
		UpdateTime: time.Now().UTC(),
	}
	workerNode := &meta.Node{
		UUID:       "dennis ritchie",
		Hostname:   "192.168.1.101",
		Type:       protocol.NodeType_Worker.String(),
		Version:    "worker_version",
		UpdateTime: time.Now().UTC(),
	}
	err := suite.metaStore.UpdateNode(serverNode)
	suite.NoError(err)
	err = suite.metaStore.UpdateNode(workerNode)
	suite.NoError(err)
	// get nodes
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/cluster").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), []*meta.Node{serverNode, workerNode})).
		End()
}

func (suite *MasterAPITestSuite) TestGetStats() {
	ctx := context.Background()
	// set stats
	suite.collaborativeFilteringModelScore = cf.Score{Precision: 0.1}
	suite.clickScore = ctr.Score{Precision: 0.2}
	err := suite.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumUsers), 123))
	suite.NoError(err)
	err = suite.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumItems), 234))
	suite.NoError(err)
	err = suite.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumValidPosFeedbacks), 345))
	suite.NoError(err)
	err = suite.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumValidNegFeedbacks), 456))
	suite.NoError(err)
	// get stats
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/stats").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), Status{
			NumUsers:            123,
			NumItems:            234,
			NumValidPosFeedback: 345,
			NumValidNegFeedback: 456,
			MatchingModelScore:  cf.Score{Precision: 0.1},
			RankingModelScore:   ctr.Score{Precision: 0.2},
			BinaryVersion:       "unknown-version",
		})).
		End()
}

func (suite *MasterAPITestSuite) TestGetRates() {
	ctx := context.Background()
	// write rates
	suite.Config.Recommend.DataSource.PositiveFeedbackTypes = []expression.FeedbackTypeExpression{
		expression.MustParseFeedbackTypeExpression("a"),
		expression.MustParseFeedbackTypeExpression("b"),
	}
	// This first measurement should be overwritten.
	baseTimestamp := time.Now().UTC().Truncate(24 * time.Hour)
	err := suite.CacheClient.AddTimeSeriesPoints(ctx, []cache.TimeSeriesPoint{
		{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 100.0, Timestamp: baseTimestamp.Add(-2 * 24 * time.Hour)},
		{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 2.0, Timestamp: baseTimestamp.Add(-2 * 24 * time.Hour)},
		{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 2.0, Timestamp: baseTimestamp.Add(-1 * 24 * time.Hour)},
		{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 3.0, Timestamp: baseTimestamp.Add(-0 * 24 * time.Hour)},
		{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 20.0, Timestamp: baseTimestamp.Add(-2 * 24 * time.Hour)},
		{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 20.0, Timestamp: baseTimestamp.Add(-1 * 24 * time.Hour)},
		{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 30.0, Timestamp: baseTimestamp.Add(-0 * 24 * time.Hour)},
	})
	suite.NoError(err)

	// get rates
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/rates").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), map[string][]cache.TimeSeriesPoint{
			"a": {
				{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 2.0, Timestamp: baseTimestamp.Add(-2 * 24 * time.Hour)},
				{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 2.0, Timestamp: baseTimestamp.Add(-1 * 24 * time.Hour)},
				{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 3.0, Timestamp: baseTimestamp.Add(-0 * 24 * time.Hour)},
			},
			"b": {
				{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 20.0, Timestamp: baseTimestamp.Add(-2 * 24 * time.Hour)},
				{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 20.0, Timestamp: baseTimestamp.Add(-1 * 24 * time.Hour)},
				{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 30.0, Timestamp: baseTimestamp.Add(-0 * 24 * time.Hour)},
			},
		})).
		End()
}

func (suite *MasterAPITestSuite) TestGetCategories() {
	ctx := context.Background()
	// insert categories
	err := suite.CacheClient.SetSet(ctx, cache.ItemCategories, "a", "b", "c")
	suite.NoError(err)
	// get categories
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/categories").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), []string{"a", "b", "c"})).
		End()
}

func (suite *MasterAPITestSuite) TestGetUsers() {
	ctx := context.Background()
	// add users
	users := []User{
		{data.User{UserId: "0"}, time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC), time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC)},
		{data.User{UserId: "1"}, time.Date(2001, 1, 1, 1, 1, 1, 1, time.UTC), time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC)},
		{data.User{UserId: "2"}, time.Date(2002, 1, 1, 1, 1, 1, 1, time.UTC), time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC)},
	}
	for _, user := range users {
		err := suite.DataClient.BatchInsertUsers(ctx, []data.User{user.User})
		suite.NoError(err)
		err = suite.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, user.UserId), user.LastActiveTime))
		suite.NoError(err)
		err = suite.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastUpdateUserRecommendTime, user.UserId), user.LastUpdateTime))
		suite.NoError(err)
	}
	// get users
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/users").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), UserIterator{
			Cursor: "",
			Users:  users,
		})).
		End()
	// get a user
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/user/1").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), users[1])).
		End()
}

func (suite *MasterAPITestSuite) TestSearchDocumentsOfItems() {
	type ListOperator struct {
		Name       string
		Collection string
		Subset     string
		Category   string
		Get        string
	}
	ctx := context.Background()
	operators := []ListOperator{
		{"ItemToItem", cache.ItemToItem, cache.Key("neighbors", "0"), "", "/api/dashboard/item-to-item/neighbors/0"},
		{"ItemToItemCategory", cache.ItemToItem, cache.Key("neighbors", "0"), "*", "/api/dashboard/item-to-item/neighbors/0"},
		{"LatestItems", cache.NonPersonalized, cache.Latest, "", "/api/dashboard/non-personalized/latest/"},
		{"PopularItems", cache.NonPersonalized, cache.Popular, "", "/api/dashboard/non-personalized/popular/"},
		{"LatestItemsCategory", cache.NonPersonalized, cache.Latest, "*", "/api/dashboard/non-personalized/latest/"},
		{"PopularItemsCategory", cache.NonPersonalized, cache.Popular, "*", "/api/dashboard/non-personalized/popular/"},
	}
	lastModified := time.Now()
	for i, operator := range operators {
		suite.T().Run(operator.Name, func(t *testing.T) {
			// Put scores
			scores := []cache.Score{
				{Id: strconv.Itoa(i) + "0", Score: 100, Categories: []string{operator.Category}},
				{Id: strconv.Itoa(i) + "1", Score: 99, Categories: []string{operator.Category}},
				{Id: strconv.Itoa(i) + "2", Score: 98, Categories: []string{operator.Category}},
				{Id: strconv.Itoa(i) + "3", Score: 97, Categories: []string{operator.Category}},
				{Id: strconv.Itoa(i) + "4", Score: 96, Categories: []string{operator.Category}},
			}
			err := suite.CacheClient.AddScores(ctx, operator.Collection, operator.Subset, scores)
			suite.NoError(err)
			err = suite.CacheClient.Set(ctx, cache.Time(cache.Key(operator.Collection+"_update_time", operator.Subset), lastModified))
			assert.NoError(t, err)
			items := make([]ScoredItem, 0)
			for _, score := range scores {
				items = append(items, ScoredItem{Item: data.Item{ItemId: score.Id}, Score: score.Score})
				err = suite.DataClient.BatchInsertItems(ctx, []data.Item{{ItemId: score.Id}})
				suite.NoError(err)
			}
			// hide item
			apitest.New().
				Handler(suite.handler).
				Patch("/api/item/"+strconv.Itoa(i)+"3").
				Header("Cookie", suite.cookie).
				JSON(data.ItemPatch{IsHidden: proto.Bool(true)}).
				Expect(t).
				Status(http.StatusOK).
				End()
			apitest.New().
				Handler(suite.handler).
				Get(operator.Get).
				Header("Cookie", suite.cookie).
				Query("category", operator.Category).
				Expect(t).
				Status(http.StatusOK).
				HeaderPresent("Last-Modified").
				Body(marshal(t, []ScoredItem{items[0], items[1], items[2], items[4]})).
				End()
		})
	}
}

func (suite *MasterAPITestSuite) TestSearchDocumentsOfUsers() {
	type ListOperator struct {
		Prefix string
		Label  string
		Get    string
	}
	ctx := context.Background()
	operators := []ListOperator{
		{cache.UserToUser, cache.Key("neighbors", "0"), "/api/dashboard/user-to-user/neighbors/0/"},
	}
	lastModified := time.Now()
	for _, operator := range operators {
		suite.T().Logf("test RESTful API: %v", operator.Get)
		// Put scores
		scores := []cache.Score{
			{Id: "0", Score: 100, Categories: []string{""}},
			{Id: "1", Score: 99, Categories: []string{""}},
			{Id: "2", Score: 98, Categories: []string{""}},
			{Id: "3", Score: 97, Categories: []string{""}},
			{Id: "4", Score: 96, Categories: []string{""}},
		}
		err := suite.CacheClient.AddScores(ctx, operator.Prefix, operator.Label, scores)
		suite.NoError(err)
		err = suite.CacheClient.Set(ctx, cache.Time(cache.Key(operator.Prefix+"_update_time", operator.Label), lastModified))
		suite.NoError(err)
		users := make([]ScoreUser, 0)
		for _, score := range scores {
			users = append(users, ScoreUser{User: data.User{UserId: score.Id}, Score: score.Score})
			err = suite.DataClient.BatchInsertUsers(ctx, []data.User{{UserId: score.Id}})
			suite.NoError(err)
		}
		apitest.New().
			Handler(suite.handler).
			Get(operator.Get).
			Header("Cookie", suite.cookie).
			Expect(suite.T()).
			Status(http.StatusOK).
			HeaderPresent("Last-Modified").
			Body(marshal(suite.T(), users)).
			End()
	}
}

func (suite *MasterAPITestSuite) TestFeedback() {
	ctx := context.Background()
	// insert feedback
	feedback := []Feedback{
		{FeedbackType: "click", UserId: "0", Item: data.Item{ItemId: "0"}},
		{FeedbackType: "click", UserId: "0", Item: data.Item{ItemId: "2"}},
		{FeedbackType: "click", UserId: "0", Item: data.Item{ItemId: "4"}},
		{FeedbackType: "click", UserId: "0", Item: data.Item{ItemId: "6"}},
		{FeedbackType: "click", UserId: "0", Item: data.Item{ItemId: "8"}},
	}
	for _, v := range feedback {
		err := suite.DataClient.BatchInsertFeedback(ctx, []data.Feedback{{
			FeedbackKey: data.FeedbackKey{FeedbackType: v.FeedbackType, UserId: v.UserId, ItemId: v.Item.ItemId},
		}}, true, true, true)
		suite.NoError(err)
	}
	// get feedback
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/user/0/feedback/click").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), feedback)).
		End()
}

func (suite *MasterAPITestSuite) TestGetRecommends() {
	// inset recommendation
	itemIds := []cache.Score{
		{Id: "1", Score: 99, Categories: []string{""}},
		{Id: "2", Score: 98, Categories: []string{""}},
		{Id: "3", Score: 97, Categories: []string{""}},
		{Id: "4", Score: 96, Categories: []string{""}},
		{Id: "5", Score: 95, Categories: []string{""}},
		{Id: "6", Score: 94, Categories: []string{""}},
		{Id: "7", Score: 93, Categories: []string{""}},
		{Id: "8", Score: 92, Categories: []string{""}},
	}
	ctx := context.Background()
	err := suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0", itemIds)
	suite.NoError(err)
	// insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}},
	}
	err = suite.DataClient.BatchInsertFeedback(ctx, feedback, true, true, true)
	suite.NoError(err)
	// insert items
	for _, item := range itemIds {
		err = suite.DataClient.BatchInsertItems(ctx, []data.Item{{ItemId: item.Id}})
		suite.NoError(err)
	}
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/recommend/0/offline").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), []data.Item{
			{ItemId: "1"}, {ItemId: "3"}, {ItemId: "5"}, {ItemId: "6"}, {ItemId: "7"}, {ItemId: "8"},
		})).
		End()

	suite.Config.Recommend.Online.FallbackRecommend = []string{"collaborative", "item_based", "user_based", "latest", "popular"}
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/recommend/0/_").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), []data.Item{
			{ItemId: "1"}, {ItemId: "3"}, {ItemId: "5"}, {ItemId: "6"}, {ItemId: "7"}, {ItemId: "8"},
		})).
		End()
}

func (suite *MasterAPITestSuite) TestPurge() {
	ctx := context.Background()
	// insert data
	err := suite.CacheClient.Set(ctx, cache.String("key", "value"))
	suite.NoError(err)
	ret, err := suite.CacheClient.Get(ctx, "key").String()
	suite.NoError(err)
	suite.Equal("value", ret)

	err = suite.CacheClient.AddSet(ctx, "set", "a", "b", "c")
	suite.NoError(err)
	set, err := suite.CacheClient.GetSet(ctx, "set")
	suite.NoError(err)
	suite.ElementsMatch([]string{"a", "b", "c"}, set)

	err = suite.CacheClient.AddScores(ctx, "sorted", "", []cache.Score{
		{Id: "a", Score: 1, Categories: []string{""}},
		{Id: "b", Score: 2, Categories: []string{""}},
		{Id: "c", Score: 3, Categories: []string{""}}})
	suite.NoError(err)
	z, err := suite.CacheClient.SearchScores(ctx, "sorted", "", []string{""}, 0, -1)
	suite.NoError(err)
	suite.ElementsMatch([]cache.Score{
		{Id: "a", Score: 1, Categories: []string{""}},
		{Id: "b", Score: 2, Categories: []string{""}},
		{Id: "c", Score: 3, Categories: []string{""}}}, z)

	err = suite.DataClient.BatchInsertFeedback(ctx, lo.Map(lo.Range(100), func(t int, i int) data.Feedback {
		return data.Feedback{FeedbackKey: data.FeedbackKey{
			FeedbackType: "click",
			UserId:       strconv.Itoa(t),
			ItemId:       strconv.Itoa(t),
		}}
	}), true, true, true)
	suite.NoError(err)
	_, users, err := suite.DataClient.GetUsers(ctx, "", 100)
	suite.NoError(err)
	suite.Equal(100, len(users))
	_, items, err := suite.DataClient.GetItems(ctx, "", 100, nil)
	suite.NoError(err)
	suite.Equal(100, len(items))
	_, feedbacks, err := suite.DataClient.GetFeedback(ctx, "", 100, nil, lo.ToPtr(time.Now()))
	suite.NoError(err)
	suite.Equal(100, len(feedbacks))

	// purge data
	req := httptest.NewRequest("POST", "https://example.com/",
		strings.NewReader("check_list=delete_users,delete_items,delete_feedback,delete_cache"))
	req.Header.Set("Cookie", suite.cookie)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	suite.purge(w, req)
	suite.Equal(http.StatusOK, w.Code)

	_, err = suite.CacheClient.Get(ctx, "key").String()
	suite.ErrorIs(err, errors.NotFound)
	set, err = suite.CacheClient.GetSet(ctx, "set")
	suite.NoError(err)
	suite.Empty(set)
	z, err = suite.CacheClient.SearchScores(ctx, "sorted", "", []string{""}, 0, -1)
	suite.NoError(err)
	suite.Empty(z)

	_, users, err = suite.DataClient.GetUsers(ctx, "", 100)
	suite.NoError(err)
	suite.Empty(users)
	_, items, err = suite.DataClient.GetItems(ctx, "", 100, nil)
	suite.NoError(err)
	suite.Empty(items)
	_, feedbacks, err = suite.DataClient.GetFeedback(ctx, "", 100, nil, lo.ToPtr(time.Now()))
	suite.NoError(err)
	suite.Empty(feedbacks)
}

func (suite *MasterAPITestSuite) TestGetConfig() {
	suite.Config.Recommend.DataSource.PositiveFeedbackTypes = []expression.FeedbackTypeExpression{
		expression.MustParseFeedbackTypeExpression("a")}
	suite.Config.Recommend.DataSource.ReadFeedbackTypes = []expression.FeedbackTypeExpression{
		expression.MustParseFeedbackTypeExpression("b")}
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/config").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), formatConfig(convertToMapStructure(suite.T(), suite.Config)))).
		End()

	suite.Config.Master.DashboardRedacted = true
	redactedConfig := formatConfig(convertToMapStructure(suite.T(), suite.Config))
	delete(redactedConfig, "database")
	apitest.New().
		Handler(suite.handler).
		Get("/api/dashboard/config").
		Header("Cookie", suite.cookie).
		Expect(suite.T()).
		Status(http.StatusOK).
		Body(marshal(suite.T(), redactedConfig)).
		End()
}

func (suite *MasterAPITestSuite) TestDumpAndRestore() {
	ctx := context.Background()
	// insert users
	users := make([]data.User, batchSize+1)
	for i := range users {
		users[i] = data.User{
			UserId: fmt.Sprintf("%05d", i),
			Labels: map[string]any{"a": fmt.Sprintf("%d", 2*i+1), "b": fmt.Sprintf("%d", 2*i+2)},
		}
	}
	err := suite.DataClient.BatchInsertUsers(ctx, users)
	suite.NoError(err)
	// insert items
	items := make([]data.Item, batchSize+1)
	for i := range items {
		items[i] = data.Item{
			ItemId: fmt.Sprintf("%05d", i),
			Labels: map[string]any{"a": fmt.Sprintf("%d", 2*i+1), "b": fmt.Sprintf("%d", 2*i+2)},
		}
	}
	err = suite.DataClient.BatchInsertItems(ctx, items)
	suite.NoError(err)
	// insert feedback
	feedback := make([]data.Feedback, batchSize+1)
	for i := range feedback {
		feedback[i] = data.Feedback{
			FeedbackKey: data.FeedbackKey{
				FeedbackType: "click",
				UserId:       fmt.Sprintf("%05d", i),
				ItemId:       fmt.Sprintf("%05d", i),
			},
		}
	}
	err = suite.DataClient.BatchInsertFeedback(ctx, feedback, true, true, true)
	suite.NoError(err)

	// dump data
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Header.Set("Cookie", suite.cookie)
	w := httptest.NewRecorder()
	suite.dump(w, req)
	suite.Equal(http.StatusOK, w.Code)

	// restore data
	err = suite.DataClient.Purge()
	suite.NoError(err)
	req = httptest.NewRequest("POST", "https://example.com/", bytes.NewReader(w.Body.Bytes()))
	req.Header.Set("Cookie", suite.cookie)
	req.Header.Set("Content-Type", "application/octet-stream")
	w = httptest.NewRecorder()
	suite.restore(w, req)
	suite.Equal(http.StatusOK, w.Code)

	// check data
	_, returnUsers, err := suite.DataClient.GetUsers(ctx, "", len(users))
	suite.NoError(err)
	if suite.Equal(len(users), len(returnUsers)) {
		suite.Equal(users, returnUsers)
	}
	_, returnItems, err := suite.DataClient.GetItems(ctx, "", len(items), nil)
	suite.NoError(err)
	if suite.Equal(len(items), len(returnItems)) {
		suite.Equal(items, returnItems)
	}
	_, returnFeedback, err := suite.DataClient.GetFeedback(ctx, "", len(feedback), nil, lo.ToPtr(time.Now()))
	suite.NoError(err)
	if suite.Equal(len(feedback), len(returnFeedback)) {
		suite.Equal(feedback, returnFeedback)
	}
}

func (suite *MasterAPITestSuite) TestExportAndImport() {
	ctx := context.Background()
	// insert users
	users := make([]data.User, batchSize+1)
	for i := range users {
		users[i] = data.User{
			UserId: fmt.Sprintf("%05d", i),
			Labels: map[string]any{"a": fmt.Sprintf("%d", 2*i+1), "b": fmt.Sprintf("%d", 2*i+2)},
		}
	}
	err := suite.DataClient.BatchInsertUsers(ctx, users)
	suite.NoError(err)
	// insert items
	items := make([]data.Item, batchSize+1)
	for i := range items {
		items[i] = data.Item{
			ItemId: fmt.Sprintf("%05d", i),
			Labels: map[string]any{"a": fmt.Sprintf("%d", 2*i+1), "b": fmt.Sprintf("%d", 2*i+2)},
		}
	}
	err = suite.DataClient.BatchInsertItems(ctx, items)
	suite.NoError(err)
	// insert feedback
	feedback := make([]data.Feedback, batchSize+1)
	for i := range feedback {
		feedback[i] = data.Feedback{
			FeedbackKey: data.FeedbackKey{
				FeedbackType: "click",
				UserId:       fmt.Sprintf("%05d", i),
				ItemId:       fmt.Sprintf("%05d", i),
			},
		}
	}
	err = suite.DataClient.BatchInsertFeedback(ctx, feedback, true, true, true)
	suite.NoError(err)

	// export users
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Header.Set("Cookie", suite.cookie)
	w := httptest.NewRecorder()
	suite.importExportUsers(w, req)
	suite.Equal(http.StatusOK, w.Code)
	usersData := w.Body.Bytes()
	// export items
	req = httptest.NewRequest("GET", "https://example.com/", nil)
	req.Header.Set("Cookie", suite.cookie)
	w = httptest.NewRecorder()
	suite.importExportItems(w, req)
	suite.Equal(http.StatusOK, w.Code)
	itemsData := w.Body.Bytes()
	// export feedback
	req = httptest.NewRequest("GET", "https://example.com/", nil)
	req.Header.Set("Cookie", suite.cookie)
	w = httptest.NewRecorder()
	suite.importExportFeedback(w, req)
	suite.Equal(http.StatusOK, w.Code)
	feedbackData := w.Body.Bytes()

	err = suite.DataClient.Purge()
	suite.NoError(err)
	// import users
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	file, err := writer.CreateFormFile("file", "users.jsonl")
	suite.NoError(err)
	_, err = file.Write(usersData)
	suite.NoError(err)
	err = writer.Close()
	suite.NoError(err)
	req = httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", suite.cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	suite.importExportUsers(w, req)
	suite.Equal(http.StatusOK, w.Code)
	// import items
	buf = bytes.NewBuffer(nil)
	writer = multipart.NewWriter(buf)
	file, err = writer.CreateFormFile("file", "items.jsonl")
	suite.NoError(err)
	_, err = file.Write(itemsData)
	suite.NoError(err)
	err = writer.Close()
	suite.NoError(err)
	req = httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", suite.cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	suite.importExportItems(w, req)
	suite.Equal(http.StatusOK, w.Code)
	// import feedback
	buf = bytes.NewBuffer(nil)
	writer = multipart.NewWriter(buf)
	file, err = writer.CreateFormFile("file", "feedback.jsonl")
	suite.NoError(err)
	_, err = file.Write(feedbackData)
	suite.NoError(err)
	err = writer.Close()
	suite.NoError(err)
	req = httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", suite.cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	suite.importExportFeedback(w, req)
	suite.Equal(http.StatusOK, w.Code)

	// check data
	_, returnUsers, err := suite.DataClient.GetUsers(ctx, "", len(users))
	suite.NoError(err)
	if suite.Equal(len(users), len(returnUsers)) {
		suite.Equal(users, returnUsers)
	}
	_, returnItems, err := suite.DataClient.GetItems(ctx, "", len(items), nil)
	suite.NoError(err)
	if suite.Equal(len(items), len(returnItems)) {
		suite.Equal(items, returnItems)
	}
	_, returnFeedback, err := suite.DataClient.GetFeedback(ctx, "", len(feedback), nil, lo.ToPtr(time.Now()))
	suite.NoError(err)
	if suite.Equal(len(feedback), len(returnFeedback)) {
		suite.Equal(feedback, returnFeedback)
	}
}

func (suite *MasterAPITestSuite) TestChat() {
	content := "In my younger and more vulnerable years my father gave me some advice that I've been turning over in" +
		" my mind ever since. \"Whenever you feel like criticizing any one,\" he told me, \" just remember that all " +
		"the people in this world haven't had the advantages that you've had.\""
	buf := strings.NewReader(content)
	req := httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", suite.cookie)
	w := httptest.NewRecorder()
	suite.chat(w, req)
	suite.Equal(http.StatusOK, w.Code, w.Body.String())
	suite.Equal(content, w.Body.String())
}

func TestMasterAPI(t *testing.T) {
	suite.Run(t, new(MasterAPITestSuite))
}
