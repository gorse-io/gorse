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
	"encoding/json"
	"github.com/alicebob/miniredis/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

type mockServer struct {
	dataStoreServer  *miniredis.Miniredis
	cacheStoreServer *miniredis.Miniredis
	handler          *restful.Container
	Master
}

func newMockServer(t *testing.T) (*mockServer, string) {
	s := new(mockServer)
	// create mock redis server
	var err error
	s.dataStoreServer, err = miniredis.Run()
	assert.NoError(t, err)
	s.cacheStoreServer, err = miniredis.Run()
	assert.NoError(t, err)
	// open database
	s.Settings = config.NewSettings()
	s.DataClient, err = data.Open("redis://" + s.dataStoreServer.Addr())
	assert.NoError(t, err)
	s.CacheClient, err = cache.Open("redis://" + s.cacheStoreServer.Addr())
	assert.NoError(t, err)
	// create server
	s.Config = config.GetDefaultConfig()
	s.RestServer.HiddenItemsManager = server.NewHiddenItemsManager(&s.RestServer)
	s.RestServer.PopularItemsCache = server.NewPopularItemsCache(&s.RestServer)
	s.WebService = new(restful.WebService)
	s.CreateWebService()
	// create handler
	s.handler = restful.NewContainer()
	s.handler.Add(s.WebService)
	// login
	req, err := http.NewRequest("POST", "/login", bytes.NewBuffer([]byte(`{"username":"admin","password":"admin"}`)))
	assert.NoError(t, err)
	resp := httptest.NewRecorder()
	s.login(resp, req)
	assert.Equal(t, http.StatusFound, resp.Code)
	return s, resp.Header().Get("Set-Cookie")
}

func (s *mockServer) Close(t *testing.T) {
	err := s.DataClient.Close()
	assert.NoError(t, err)
	err = s.CacheClient.Close()
	assert.NoError(t, err)
	s.dataStoreServer.Close()
	s.cacheStoreServer.Close()
}

func marshal(t *testing.T, v interface{}) string {
	s, err := json.Marshal(v)
	assert.NoError(t, err)
	return string(s)
}

func TestMaster_ExportUsers(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// insert users
	users := []data.User{
		{UserId: "1", Labels: []string{"a", "b"}},
		{UserId: "2", Labels: []string{"b", "c"}},
		{UserId: "3", Labels: []string{"c", "d"}},
	}
	err := s.DataClient.BatchInsertUsers(users)
	assert.NoError(t, err)
	// send request
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Header.Set("Cookie", cookie)
	w := httptest.NewRecorder()
	s.importExportUsers(w, req)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Equal(t, "attachment;filename=users.csv", w.Header().Get("Content-Disposition"))
	assert.Equal(t, "user_id,labels\r\n"+
		"1,a|b\r\n"+
		"2,b|c\r\n"+
		"3,c|d\r\n", w.Body.String())
}

func TestMaster_ExportItems(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// insert items
	items := []data.Item{
		{"1", false, []string{"x"}, time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC), []string{"a", "b"}, "o,n,e"},
		{"2", false, []string{"x", "y"}, time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC), []string{"b", "c"}, "t\r\nw\r\no"},
		{"3", true, nil, time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC), nil, "\"three\""},
	}
	err := s.DataClient.BatchInsertItems(items)
	assert.NoError(t, err)
	// send request
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Header.Set("Cookie", cookie)
	w := httptest.NewRecorder()
	s.importExportItems(w, req)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Equal(t, "attachment;filename=items.csv", w.Header().Get("Content-Disposition"))
	assert.Equal(t, "item_id,is_hidden,categories,time_stamp,labels,description\r\n"+
		"1,false,x,2020-01-01 01:01:01.000000001 +0000 UTC,a|b,\"o,n,e\"\r\n"+
		"2,false,x|y,2021-01-01 01:01:01.000000001 +0000 UTC,b|c,\"t\r\nw\r\no\"\r\n"+
		"3,true,,2022-01-01 01:01:01.000000001 +0000 UTC,,\"\"\"three\"\"\"\r\n", w.Body.String())
}

func TestMaster_ExportFeedback(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// insert feedback
	feedbacks := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "share", UserId: "1", ItemId: "4"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "read", UserId: "2", ItemId: "6"}},
	}
	err := s.DataClient.BatchInsertFeedback(feedbacks, true, true, true)
	assert.NoError(t, err)
	// send request
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.Header.Set("Cookie", cookie)
	w := httptest.NewRecorder()
	s.importExportFeedback(w, req)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Equal(t, "attachment;filename=feedback.csv", w.Header().Get("Content-Disposition"))
	assert.Equal(t, "feedback_type,user_id,item_id,time_stamp\r\n"+
		"click,0,2,0001-01-01 00:00:00 +0000 UTC\r\n"+
		"read,2,6,0001-01-01 00:00:00 +0000 UTC\r\n"+
		"share,1,4,0001-01-01 00:00:00 +0000 UTC\r\n", w.Body.String())
}

func TestMaster_ImportUsers(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// send request
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	err := writer.WriteField("has-header", "false")
	assert.NoError(t, err)
	err = writer.WriteField("sep", "\t")
	assert.NoError(t, err)
	err = writer.WriteField("label-sep", "::")
	assert.NoError(t, err)
	err = writer.WriteField("format", "lu")
	assert.NoError(t, err)
	file, err := writer.CreateFormFile("file", "users.csv")
	assert.NoError(t, err)
	_, err = file.Write([]byte("a::b\t1\n" +
		"b::c\t2\n" +
		"\"c::d\"\t\"3\"\n"))
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)
	req := httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	s.importExportUsers(w, req)
	// check
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.JSONEq(t, marshal(t, server.Success{RowAffected: 3}), w.Body.String())
	_, items, err := s.DataClient.GetUsers("", 100)
	assert.NoError(t, err)
	assert.Equal(t, []data.User{
		{UserId: "1", Labels: []string{"a", "b"}},
		{UserId: "2", Labels: []string{"b", "c"}},
		{UserId: "3", Labels: []string{"c", "d"}},
	}, items)
}

func TestMaster_ImportUsers_DefaultFormat(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// send request
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	file, err := writer.CreateFormFile("file", "users.csv")
	assert.NoError(t, err)
	_, err = file.Write([]byte("user_id,labels\r\n" +
		"1,a|用例\r\n" +
		"2,b|乱码\r\n" +
		"\"3\",\"c|测试\"\r\n"))
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)
	req := httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	s.importExportUsers(w, req)
	// check
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.JSONEq(t, marshal(t, server.Success{RowAffected: 3}), w.Body.String())
	_, items, err := s.DataClient.GetUsers("", 100)
	assert.NoError(t, err)
	assert.Equal(t, []data.User{
		{UserId: "1", Labels: []string{"a", "用例"}},
		{UserId: "2", Labels: []string{"b", "乱码"}},
		{UserId: "3", Labels: []string{"c", "测试"}},
	}, items)
}

func TestMaster_ImportItems(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// send request
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	err := writer.WriteField("has-header", "false")
	assert.NoError(t, err)
	err = writer.WriteField("sep", "\t")
	assert.NoError(t, err)
	err = writer.WriteField("label-sep", "::")
	assert.NoError(t, err)
	err = writer.WriteField("format", "ildtch")
	assert.NoError(t, err)
	file, err := writer.CreateFormFile("file", "items.csv")
	assert.NoError(t, err)
	_, err = file.Write([]byte("1\ta::b\t\"o,n,e\"\t2020-01-01 01:01:01.000000001 +0000 UTC\tx\t0\n" +
		"2\tb::c\t\"t\r\nw\r\no\"\t2021-01-01 01:01:01.000000001 +0000 UTC\tx::y\t0\n" +
		"\"3\"\t\"c::d\"\t\"\"\"three\"\"\"\t\"2022-01-01 01:01:01.000000001 +0000 UTC\"\t\t\"1\"\n"))
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)
	req := httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	s.importExportItems(w, req)
	// check
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.JSONEq(t, marshal(t, server.Success{RowAffected: 3}), w.Body.String())
	_, items, err := s.DataClient.GetItems("", 100, nil)
	assert.NoError(t, err)
	assert.Equal(t, []data.Item{
		{"1", false, []string{"x"}, time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC), []string{"a", "b"}, "o,n,e"},
		{"2", false, []string{"x", "y"}, time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC), []string{"b", "c"}, "t\r\nw\r\no"},
		{"3", true, nil, time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC), []string{"c", "d"}, "\"three\""},
	}, items)
}

func TestMaster_ImportItems_DefaultFormat(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// send request
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	file, err := writer.CreateFormFile("file", "items.csv")
	assert.NoError(t, err)
	_, err = file.Write([]byte("item_id,is_hidden,categories,time_stamp,labels,description\r\n" +
		"1,false,x,2020-01-01 01:01:01.000000001 +0000 UTC,a|b,one\r\n" +
		"2,false,x|y,2021-01-01 01:01:01.000000001 +0000 UTC,b|c,two\r\n" +
		"\"3\",\"true\",,\"2022-01-01 01:01:01.000000001 +0000 UTC\",,\"three\"\r\n"))
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)
	req := httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	s.importExportItems(w, req)
	// check
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.JSONEq(t, marshal(t, server.Success{RowAffected: 3}), w.Body.String())
	_, items, err := s.DataClient.GetItems("", 100, nil)
	assert.NoError(t, err)
	assert.Equal(t, []data.Item{
		{"1", false, []string{"x"}, time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC), []string{"a", "b"}, "one"},
		{"2", false, []string{"x", "y"}, time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC), []string{"b", "c"}, "two"},
		{"3", true, nil, time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC), nil, "three"},
	}, items)
}

func TestMaster_ImportFeedback(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// send request
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	err := writer.WriteField("format", "uift")
	assert.NoError(t, err)
	err = writer.WriteField("sep", "\t")
	assert.NoError(t, err)
	err = writer.WriteField("has-header", "false")
	assert.NoError(t, err)
	file, err := writer.CreateFormFile("file", "feedback.csv")
	assert.NoError(t, err)
	_, err = file.Write([]byte("0\t2\tclick\t0001-01-01 00:00:00 +0000 UTC\n" +
		"2\t6\tread\t0001-01-01 00:00:00 +0000 UTC\n" +
		"\"1\"\t\"4\"\t\"share\"\t\"0001-01-01 00:00:00 +0000 UTC\"\n"))
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)
	req := httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	s.importExportFeedback(w, req)
	// check
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.JSONEq(t, marshal(t, server.Success{RowAffected: 3}), w.Body.String())
	_, feedback, err := s.DataClient.GetFeedback("", 100, nil)
	assert.NoError(t, err)
	assert.Equal(t, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "read", UserId: "2", ItemId: "6"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "share", UserId: "1", ItemId: "4"}},
	}, feedback)
}

func TestMaster_ImportFeedback_Default(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// send request
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	file, err := writer.CreateFormFile("file", "feedback.csv")
	assert.NoError(t, err)
	_, err = file.Write([]byte("feedback_type,user_id,item_id,time_stamp\r\n" +
		"click,0,2,0001-01-01 00:00:00 +0000 UTC\r\n" +
		"read,2,6,0001-01-01 00:00:00 +0000 UTC\r\n" +
		"\"share\",\"1\",\"4\",\"0001-01-01 00:00:00 +0000 UTC\"\r\n"))
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)
	req := httptest.NewRequest("POST", "https://example.com/", buf)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	s.importExportFeedback(w, req)
	// check
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.JSONEq(t, marshal(t, server.Success{RowAffected: 3}), w.Body.String())
	_, feedback, err := s.DataClient.GetFeedback("", 100, nil)
	assert.NoError(t, err)
	assert.Equal(t, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "read", UserId: "2", ItemId: "6"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "share", UserId: "1", ItemId: "4"}},
	}, feedback)
}

func TestMaster_GetCluster(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// add nodes
	serverNode := &Node{"alan turnin", ServerNode, "192.168.1.100", 1080, "server_version"}
	workerNode := &Node{"dennis ritchie", WorkerNode, "192.168.1.101", 1081, "worker_version"}
	s.nodesInfo = make(map[string]*Node)
	s.nodesInfo["alan turning"] = serverNode
	s.nodesInfo["dennis ritchie"] = workerNode
	// get nodes
	apitest.New().
		Handler(s.handler).
		Get("/api/dashboard/cluster").
		Header("Cookie", cookie).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []*Node{workerNode, serverNode})).
		End()
}

func TestMaster_GetStats(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// set stats
	s.rankingScore = ranking.Score{Precision: 0.1}
	s.clickScore = click.Score{Precision: 0.2}
	err := s.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumUsers), 123))
	assert.NoError(t, err)
	err = s.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumItems), 234))
	assert.NoError(t, err)
	err = s.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumValidPosFeedbacks), 345))
	assert.NoError(t, err)
	err = s.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumValidNegFeedbacks), 456))
	assert.NoError(t, err)
	// get stats
	apitest.New().
		Handler(s.handler).
		Get("/api/dashboard/stats").
		Header("Cookie", cookie).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, Status{
			NumUsers:            123,
			NumItems:            234,
			NumValidPosFeedback: 345,
			NumValidNegFeedback: 456,
			MatchingModelScore:  ranking.Score{Precision: 0.1},
			RankingModelScore:   click.Score{Precision: 0.2},
			BinaryVersion:       "unknown-version",
		})).
		End()
}

func TestMaster_GetRates(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// write rates
	s.Config.Recommend.DataSource.PositiveFeedbackTypes = []string{"a", "b"}
	err := s.RestServer.InsertMeasurement(server.Measurement{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 2.0, Timestamp: time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)})
	assert.NoError(t, err)
	err = s.RestServer.InsertMeasurement(server.Measurement{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 2.0, Timestamp: time.Date(2000, 1, 2, 1, 1, 1, 0, time.UTC)})
	assert.NoError(t, err)
	err = s.RestServer.InsertMeasurement(server.Measurement{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 3.0, Timestamp: time.Date(2000, 1, 3, 1, 1, 1, 0, time.UTC)})
	assert.NoError(t, err)
	err = s.RestServer.InsertMeasurement(server.Measurement{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 20.0, Timestamp: time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)})
	assert.NoError(t, err)
	err = s.RestServer.InsertMeasurement(server.Measurement{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 20.0, Timestamp: time.Date(2000, 1, 2, 1, 1, 1, 0, time.UTC)})
	assert.NoError(t, err)
	err = s.RestServer.InsertMeasurement(server.Measurement{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 30.0, Timestamp: time.Date(2000, 1, 3, 1, 1, 1, 0, time.UTC)})
	assert.NoError(t, err)
	// get rates
	apitest.New().
		Handler(s.handler).
		Get("/api/dashboard/rates").
		Header("Cookie", cookie).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, map[string][]server.Measurement{
			"a": {
				{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 3.0, Timestamp: time.Date(2000, 1, 3, 1, 1, 1, 0, time.UTC)},
				{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 2.0, Timestamp: time.Date(2000, 1, 2, 1, 1, 1, 0, time.UTC)},
				{Name: cache.Key(PositiveFeedbackRate, "a"), Value: 2.0, Timestamp: time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)},
			},
			"b": {
				{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 30.0, Timestamp: time.Date(2000, 1, 3, 1, 1, 1, 0, time.UTC)},
				{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 20.0, Timestamp: time.Date(2000, 1, 2, 1, 1, 1, 0, time.UTC)},
				{Name: cache.Key(PositiveFeedbackRate, "b"), Value: 20.0, Timestamp: time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)},
			},
		})).
		End()
}

func TestMaster_GetCategories(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// insert categories
	err := s.CacheClient.SetSet(cache.ItemCategories, "a", "b", "c")
	assert.NoError(t, err)
	// get categories
	apitest.New().
		Handler(s.handler).
		Get("/api/dashboard/categories").
		Header("Cookie", cookie).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"a", "b", "c"})).
		End()
}

func TestMaster_GetUsers(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// add users
	users := []User{
		{data.User{UserId: "0"}, time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC), time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC)},
		{data.User{UserId: "1"}, time.Date(2001, 1, 1, 1, 1, 1, 1, time.UTC), time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC)},
		{data.User{UserId: "2"}, time.Date(2002, 1, 1, 1, 1, 1, 1, time.UTC), time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC)},
	}
	for _, user := range users {
		err := s.DataClient.BatchInsertUsers([]data.User{user.User})
		assert.NoError(t, err)
		err = s.CacheClient.Set(cache.Time(cache.Key(cache.LastModifyUserTime, user.UserId), user.LastActiveTime))
		assert.NoError(t, err)
		err = s.CacheClient.Set(cache.Time(cache.Key(cache.LastUpdateUserRecommendTime, user.UserId), user.LastUpdateTime))
		assert.NoError(t, err)
	}
	// get users
	apitest.New().
		Handler(s.handler).
		Get("/api/dashboard/users").
		Header("Cookie", cookie).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, UserIterator{
			Cursor: "",
			Users:  users,
		})).
		End()
	// get a user
	apitest.New().
		Handler(s.handler).
		Get("/api/dashboard/user/1").
		Header("Cookie", cookie).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, users[1])).
		End()
}

func TestServer_SortedItems(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	type ListOperator struct {
		Name   string
		Prefix string
		Label  string
		Get    string
	}
	operators := []ListOperator{
		{"Item Neighbors", cache.ItemNeighbors, "0", "/api/dashboard/item/0/neighbors"},
		{"Item Neighbors in Category", cache.ItemNeighbors, "0/*", "/api/dashboard/item/0/neighbors/*"},
		{"Latest Items", cache.LatestItems, "", "/api/dashboard/latest/"},
		{"Popular Items", cache.PopularItems, "", "/api/dashboard/popular/"},
		{"Latest Items in Category", cache.LatestItems, "*", "/api/dashboard/latest/*"},
		{"Popular Items in Category", cache.PopularItems, "*", "/api/dashboard/popular/*"},
	}
	for i, operator := range operators {
		t.Run(operator.Name, func(t *testing.T) {
			// Put scores
			scores := []cache.Scored{
				{strconv.Itoa(i) + "0", 100},
				{strconv.Itoa(i) + "1", 99},
				{strconv.Itoa(i) + "2", 98},
				{strconv.Itoa(i) + "3", 97},
				{strconv.Itoa(i) + "4", 96},
			}
			err := s.CacheClient.SetSorted(cache.Key(operator.Prefix, operator.Label), scores)
			assert.NoError(t, err)
			err = server.NewCacheModification(s.CacheClient, s.HiddenItemsManager).HideItem(strconv.Itoa(i) + "3").Exec()
			assert.NoError(t, err)
			items := make([]data.Item, 0)
			for _, score := range scores {
				items = append(items, data.Item{ItemId: score.Id})
				err = s.DataClient.BatchInsertItems([]data.Item{{ItemId: score.Id}})
				assert.NoError(t, err)
			}
			apitest.New().
				Handler(s.handler).
				Get(operator.Get).
				Header("Cookie", cookie).
				Expect(t).
				Status(http.StatusOK).
				Body(marshal(t, []data.Item{items[0], items[1], items[2], items[4]})).
				End()
		})
	}
}

func TestServer_SortedUsers(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	type ListOperator struct {
		Prefix string
		Label  string
		Get    string
	}
	operators := []ListOperator{
		{cache.UserNeighbors, "0", "/api/dashboard/user/0/neighbors"},
	}
	for _, operator := range operators {
		t.Logf("test RESTful API: %v", operator.Get)
		// Put scores
		scores := []cache.Scored{
			{"0", 100},
			{"1", 99},
			{"2", 98},
			{"3", 97},
			{"4", 96},
		}
		err := s.CacheClient.SetSorted(cache.Key(operator.Prefix, operator.Label), scores)
		assert.NoError(t, err)
		users := make([]data.User, 0)
		for _, score := range scores {
			users = append(users, data.User{UserId: score.Id})
			err = s.DataClient.BatchInsertUsers([]data.User{{UserId: score.Id}})
			assert.NoError(t, err)
		}
		apitest.New().
			Handler(s.handler).
			Get(operator.Get).
			Header("Cookie", cookie).
			Expect(t).
			Status(http.StatusOK).
			Body(marshal(t, users)).
			End()
	}
}

func TestServer_Feedback(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// insert feedback
	feedback := []Feedback{
		{FeedbackType: "click", UserId: "0", Item: data.Item{ItemId: "0"}},
		{FeedbackType: "click", UserId: "0", Item: data.Item{ItemId: "2"}},
		{FeedbackType: "click", UserId: "0", Item: data.Item{ItemId: "4"}},
		{FeedbackType: "click", UserId: "0", Item: data.Item{ItemId: "6"}},
		{FeedbackType: "click", UserId: "0", Item: data.Item{ItemId: "8"}},
	}
	for _, v := range feedback {
		err := s.DataClient.BatchInsertFeedback([]data.Feedback{{
			FeedbackKey: data.FeedbackKey{FeedbackType: v.FeedbackType, UserId: v.UserId, ItemId: v.Item.ItemId},
		}}, true, true, true)
		assert.NoError(t, err)
	}
	// get feedback
	apitest.New().
		Handler(s.handler).
		Get("/api/dashboard/user/0/feedback/click").
		Header("Cookie", cookie).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, feedback)).
		End()
}

func TestServer_GetRecommends(t *testing.T) {
	s, cookie := newMockServer(t)
	defer s.Close(t)
	// inset recommendation
	itemIds := []cache.Scored{
		{"1", 99},
		{"2", 98},
		{"3", 97},
		{"4", 96},
		{"5", 95},
		{"6", 94},
		{"7", 93},
		{"8", 92},
	}
	err := s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0"), itemIds)
	assert.NoError(t, err)
	// insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}},
	}
	err = s.RestServer.InsertFeedbackToCache(feedback)
	assert.NoError(t, err)
	// insert items
	for _, item := range itemIds {
		err = s.DataClient.BatchInsertItems([]data.Item{{ItemId: item.Id}})
		assert.NoError(t, err)
	}
	apitest.New().
		Handler(s.handler).
		Get("/api/dashboard/recommend/0/offline").
		Header("Cookie", cookie).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []data.Item{
			{ItemId: "1"}, {ItemId: "3"}, {ItemId: "5"}, {ItemId: "6"}, {ItemId: "7"}, {ItemId: "8"},
		})).
		End()

	s.Config.Recommend.Online.FallbackRecommend = []string{"collaborative", "item_based", "user_based", "latest", "popular"}
	apitest.New().
		Handler(s.handler).
		Get("/api/dashboard/recommend/0/_").
		Header("Cookie", cookie).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []data.Item{
			{ItemId: "1"}, {ItemId: "3"}, {ItemId: "5"}, {ItemId: "6"}, {ItemId: "7"}, {ItemId: "8"},
		})).
		End()
}
