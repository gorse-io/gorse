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
	"bufio"
	"fmt"
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	_ "github.com/gorse-io/dashboard"
	"github.com/rakyll/statik/fs"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
)

func (m *Master) CreateWebService() {
	ws := m.WebService
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	ws.Path("/api/")

	ws.Route(ws.GET("/dashboard/cluster").To(m.getCluster).
		Doc("Get nodes in the cluster.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Writes([]Node{}))
	ws.Route(ws.GET("/dashboard/config").To(m.getConfig).
		Doc("Get config.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Writes(config.Config{}))
	ws.Route(ws.GET("/dashboard/stats").To(m.getStats).
		Doc("Get global status.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Writes(Status{}))
	ws.Route(ws.GET("/dashboard/tasks").To(m.getTasks).
		Doc("Get tasks.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Writes([]Task{}))
	// Get a user
	ws.Route(ws.GET("/dashboard/user/{user-id}").To(m.getUser).
		Doc("Get a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes(User{}))
	// Get a user feedback
	ws.Route(ws.GET("/dashboard/user/{user-id}/feedback/{feedback-type}").To(m.getTypedFeedbackByUser).
		Doc("Get feedback by user id with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Writes([]Feedback{}))
	// Get users
	ws.Route(ws.GET("/dashboard/users").To(m.getUsers).
		Doc("Get users.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned users").DataType("int")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Writes(UserIterator{}))
	// Get popular items
	ws.Route(ws.GET("/dashboard/popular").To(m.getPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	// Get latest items
	ws.Route(ws.GET("/dashboard/latest").To(m.getLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}/{recommender}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("recommender", "one of `final`, `collaborative`, `user_based` and `item_based`").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/item/{item-id}/neighbors").To(m.getItemNeighbors).
		Doc("get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.QueryParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/user/{user-id}/neighbors").To(m.getUserNeighbors).
		Doc("get neighbors of a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.QueryParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned users").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.User{}))
}

// SinglePageAppFileSystem is the file system for single page app.
type SinglePageAppFileSystem struct {
	root http.FileSystem
}

// Open index.html if required file not exists.
func (fs *SinglePageAppFileSystem) Open(name string) (http.File, error) {
	f, err := fs.root.Open(name)
	if os.IsNotExist(err) {
		return fs.root.Open("/index.html")
	}
	return f, err
}

func (m *Master) StartHttpServer() {
	m.CreateWebService()
	statikFS, err := fs.New()
	if err != nil {
		base.Logger().Fatal("failed to load statik files", zap.Error(err))
	}
	http.Handle("/", http.FileServer(&SinglePageAppFileSystem{statikFS}))
	http.HandleFunc("/api/bulk/users", m.importExportUsers)
	http.HandleFunc("/api/bulk/items", m.importExportItems)
	http.HandleFunc("/api/bulk/feedback", m.importExportFeedback)
	//http.HandleFunc("/api/bulk/libfm", m.exportToLibFM)
	m.RestServer.StartHttpServer()
}

func (m *Master) getCluster(_ *restful.Request, response *restful.Response) {
	// collect nodes
	workers := make([]*Node, 0)
	servers := make([]*Node, 0)
	m.nodesInfoMutex.RLock()
	for _, info := range m.nodesInfo {
		switch info.Type {
		case WorkerNode:
			workers = append(workers, info)
		case ServerNode:
			servers = append(servers, info)
		}
	}
	m.nodesInfoMutex.RUnlock()
	// return nodes
	nodes := make([]*Node, 0)
	nodes = append(nodes, workers...)
	nodes = append(nodes, servers...)
	server.Ok(response, nodes)
}

func (m *Master) getConfig(_ *restful.Request, response *restful.Response) {
	server.Ok(response, m.GorseConfig)
}

type Status struct {
	NumServers          int
	NumWorkers          int
	NumUsers            int
	NumItems            int
	NumUserLabels       int
	NumItemLabels       int
	NumTotalPosFeedback int
	NumValidPosFeedback int
	NumValidNegFeedback int
	RankingScore        ranking.Score
	ClickScore          click.Score
}

func (m *Master) getStats(_ *restful.Request, response *restful.Response) {
	status := Status{}
	// read number of users
	if measurements, err := m.DataClient.GetMeasurements(NumUsers, 1); err != nil {
		server.InternalServerError(response, err)
		return
	} else if len(measurements) > 0 {
		status.NumUsers = int(measurements[0].Value)
	}
	// read number of items
	if measurements, err := m.DataClient.GetMeasurements(NumItems, 1); err != nil {
		server.InternalServerError(response, err)
		return
	} else if len(measurements) > 0 {
		status.NumItems = int(measurements[0].Value)
	}
	// read number of user labels
	if measurements, err := m.DataClient.GetMeasurements(NumUserLabels, 1); err != nil {
		server.InternalServerError(response, err)
		return
	} else if len(measurements) > 0 {
		status.NumUserLabels = int(measurements[0].Value)
	}
	// read number of item labels
	if measurements, err := m.DataClient.GetMeasurements(NumItemLabels, 1); err != nil {
		server.InternalServerError(response, err)
		return
	} else if len(measurements) > 0 {
		status.NumItemLabels = int(measurements[0].Value)
	}
	// read number of total positive feedback
	if measurements, err := m.DataClient.GetMeasurements(NumTotalPosFeedbacks, 1); err != nil {
		server.InternalServerError(response, err)
		return
	} else if len(measurements) > 0 {
		status.NumTotalPosFeedback = int(measurements[0].Value)
	}
	// read number of valid positive feedback
	if measurements, err := m.DataClient.GetMeasurements(NumValidPosFeedbacks, 1); err != nil {
		server.InternalServerError(response, err)
		return
	} else if len(measurements) > 0 {
		status.NumValidPosFeedback = int(measurements[0].Value)
	}
	// read number of valid negative feedback
	if measurements, err := m.DataClient.GetMeasurements(NumValidNegFeedbacks, 1); err != nil {
		server.InternalServerError(response, err)
		return
	} else if len(measurements) > 0 {
		status.NumValidNegFeedback = int(measurements[0].Value)
	}
	// count the number of workers and servers
	m.nodesInfoMutex.Lock()
	for _, node := range m.nodesInfo {
		switch node.Type {
		case ServerNode:
			status.NumServers++
		case WorkerNode:
			status.NumWorkers++
		}
	}
	m.nodesInfoMutex.Unlock()
	status.RankingScore = m.rankingScore
	status.ClickScore = m.clickScore
	server.Ok(response, status)
}

func (m *Master) getTasks(_ *restful.Request, response *restful.Response) {
	tasks := m.taskMonitor.List()
	server.Ok(response, tasks)
}

type UserIterator struct {
	Cursor string
	Users  []User
}

type User struct {
	data.User
	LastActiveTime string
	LastUpdateTime string
}

func (m *Master) getUser(request *restful.Request, response *restful.Response) {
	// get user id
	userId := request.PathParameter("user-id")
	// get user
	user, err := m.DataClient.GetUser(userId)
	if err != nil {
		if err == data.ErrUserNotExist {
			server.PageNotFound(response, err)
		} else {
			server.InternalServerError(response, err)
		}
		return
	}
	detail := User{User: user}
	if detail.LastActiveTime, err = m.CacheClient.GetString(cache.LastModifyUserTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	if detail.LastUpdateTime, err = m.CacheClient.GetString(cache.LastUpdateUserRecommendTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	server.Ok(response, detail)
}

func (m *Master) getUsers(request *restful.Request, response *restful.Response) {
	// Authorize
	cursor := request.QueryParameter("cursor")
	n, err := server.ParseInt(request, "n", m.GorseConfig.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	// get all users
	cursor, users, err := m.DataClient.GetUsers(cursor, n)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	details := make([]User, len(users))
	for i, user := range users {
		details[i].User = user
		if details[i].LastActiveTime, err = m.CacheClient.GetString(cache.LastModifyUserTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
			server.InternalServerError(response, err)
			return
		}
		if details[i].LastUpdateTime, err = m.CacheClient.GetString(cache.LastUpdateUserRecommendTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, UserIterator{Cursor: cursor, Users: details})
}

func (m *Master) getRecommend(request *restful.Request, response *restful.Response) {
	// parse arguments
	recommender := request.PathParameter("recommender")
	userId := request.PathParameter("user-id")
	n, err := server.ParseInt(request, "n", m.GorseConfig.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	var results []string
	switch recommender {
	case "ctr":
		results, err = m.Recommend(userId, n, server.CTRRecommender)
	case "collaborative":
		results, err = m.Recommend(userId, n, server.CollaborativeRecommender)
	case "user_based":
		results, err = m.Recommend(userId, n, server.UserBasedRecommender)
	case "item_based":
		results, err = m.Recommend(userId, n, server.ItemBasedRecommender)
	}
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	// Send result
	details := make([]data.Item, len(results))
	for i := range results {
		details[i], err = m.DataClient.GetItem(results[i])
		if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, details)
}

type Feedback struct {
	FeedbackType string
	UserId       string
	Item         data.Item
	Timestamp    time.Time
	Comment      string
}

// get feedback by user-id with feedback type
func (m *Master) getTypedFeedbackByUser(request *restful.Request, response *restful.Response) {
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	feedback, err := m.DataClient.GetUserFeedback(userId, feedbackType)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	details := make([]Feedback, len(feedback))
	for i := range feedback {
		details[i].FeedbackType = feedback[i].FeedbackType
		details[i].UserId = feedback[i].UserId
		details[i].Timestamp = feedback[i].Timestamp
		details[i].Comment = feedback[i].Comment
		details[i].Item, err = m.DataClient.GetItem(feedback[i].ItemId)
		if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, details)
}

func (m *Master) getList(prefix, name string, request *restful.Request, response *restful.Response, retType interface{}) {
	var n, begin, end int
	var err error
	// read arguments
	if begin, err = server.ParseInt(request, "offset", 0); err != nil {
		server.BadRequest(response, err)
		return
	}
	if n, err = server.ParseInt(request, "n", m.GorseConfig.Server.DefaultN); err != nil {
		server.BadRequest(response, err)
		return
	}
	end = begin + n - 1
	// Get the popular list
	scores, err := m.CacheClient.GetScores(prefix, name, begin, end)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	// Send result
	switch retType.(type) {
	case data.Item:
		details := make([]data.Item, len(scores))
		for i := range scores {
			details[i], err = m.DataClient.GetItem(scores[i].Id)
			if err != nil {
				server.InternalServerError(response, err)
				return
			}
		}
		server.Ok(response, details)
	case data.User:
		details := make([]data.User, len(scores))
		for i := range scores {
			details[i], err = m.DataClient.GetUser(scores[i].Id)
			if err != nil {
				server.InternalServerError(response, err)
				return
			}
		}
		server.Ok(response, details)
	default:
		base.Logger().Fatal("unknown return type", zap.Any("ret_type", reflect.TypeOf(retType)))
	}
}

// getPopular gets popular items from database.
func (m *Master) getPopular(request *restful.Request, response *restful.Response) {
	m.getList(cache.PopularItems, "", request, response, data.Item{})
}

func (m *Master) getLatest(request *restful.Request, response *restful.Response) {
	m.getList(cache.LatestItems, "", request, response, data.Item{})
}

func (m *Master) getItemNeighbors(request *restful.Request, response *restful.Response) {
	itemId := request.PathParameter("item-id")
	m.getList(cache.ItemNeighbors, itemId, request, response, data.Item{})
}

func (m *Master) getUserNeighbors(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")
	m.getList(cache.UserNeighbors, userId, request, response, data.User{})
}

func (m *Master) importExportUsers(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		var err error
		response.Header().Set("Content-Type", "text/csv")
		response.Header().Set("Content-Disposition", "attachment;filename=users.csv")
		// write header
		if _, err = response.Write([]byte("user_id,labels\r\n")); err != nil {
			server.InternalServerError(restful.NewResponse(response), err)
			return
		}
		// write rows
		userChan, errChan := m.DataClient.GetUserStream(batchSize)
		for users := range userChan {
			for _, user := range users {
				if _, err = response.Write([]byte(fmt.Sprintf("%s,%s\r\n",
					base.Escape(user.UserId), base.Escape(strings.Join(user.Labels, "|"))))); err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
			}
		}
		if err = <-errChan; err != nil {
			server.InternalServerError(restful.NewResponse(response), errors.Trace(err))
			return
		}
	case http.MethodPost:
		hasHeader := formValue(request, "has-header", "true") == "true"
		sep := formValue(request, "sep", ",")
		// field separator must be a single character
		if len(sep) != 1 {
			server.BadRequest(restful.NewResponse(response), fmt.Errorf("field separator must be a single character"))
			return
		}
		labelSep := formValue(request, "label-sep", "|")
		fmtString := formValue(request, "format", "ul")
		file, _, err := request.FormFile("file")
		if err != nil {
			server.BadRequest(restful.NewResponse(response), err)
			return
		}
		defer file.Close()
		m.importUsers(response, file, hasHeader, sep, labelSep, fmtString)
	}
}

func (m *Master) importUsers(response http.ResponseWriter, file io.Reader, hasHeader bool, sep, labelSep, fmtString string) {
	lineCount := 0
	timeStart := time.Now()
	users := make([]data.User, 0)
	err := base.ReadLines(bufio.NewScanner(file), sep, func(lineNumber int, splits []string) bool {
		var err error
		// skip header
		if hasHeader {
			hasHeader = false
			return true
		}
		splits, err = format(fmtString, "ul", splits, lineNumber)
		if err != nil {
			server.BadRequest(restful.NewResponse(response), err)
			return false
		}
		// 1. user id
		if err = base.ValidateId(splits[0]); err != nil {
			server.BadRequest(restful.NewResponse(response),
				fmt.Errorf("invalid user id `%v` at line %d (%s)", splits[0], lineNumber, err.Error()))
			return false
		}
		user := data.User{UserId: splits[0]}
		// 2. labels
		if splits[1] != "" {
			user.Labels = strings.Split(splits[1], labelSep)
			for _, label := range user.Labels {
				if err = base.ValidateLabel(label); err != nil {
					server.BadRequest(restful.NewResponse(response),
						fmt.Errorf("invalid label `%v` at line %d (%s)", splits[1], lineNumber, err.Error()))
					return false
				}
			}
		}
		users = append(users, user)
		// batch insert
		if len(users) == batchSize {
			err = m.DataClient.BatchInsertUsers(users)
			if err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return false
			}
			users = nil
		}
		lineCount++
		return true
	})
	if err != nil {
		server.BadRequest(restful.NewResponse(response), err)
		return
	}
	if len(users) > 0 {
		err = m.DataClient.BatchInsertUsers(users)
		if err != nil {
			server.InternalServerError(restful.NewResponse(response), err)
			return
		}
	}
	m.notifyDataImported()
	timeUsed := time.Since(timeStart)
	base.Logger().Info("complete import users",
		zap.Duration("time_used", timeUsed),
		zap.Int("num_users", lineCount))
	server.Ok(restful.NewResponse(response), server.Success{RowAffected: lineCount})
}

func (m *Master) importExportItems(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		var err error
		response.Header().Set("Content-Type", "text/csv")
		response.Header().Set("Content-Disposition", "attachment;filename=items.csv")
		// write header
		if _, err = response.Write([]byte("item_id,time_stamp,labels,description\r\n")); err != nil {
			server.InternalServerError(restful.NewResponse(response), err)
			return
		}
		// write rows
		itemChan, errChan := m.DataClient.GetItemStream(batchSize, nil)
		for items := range itemChan {
			for _, item := range items {
				if _, err = response.Write([]byte(fmt.Sprintf("%s,%v,%s,%s\r\n",
					base.Escape(item.ItemId), item.Timestamp, base.Escape(strings.Join(item.Labels, "|")), base.Escape(item.Comment)))); err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
			}
		}
		if err = <-errChan; err != nil {
			server.InternalServerError(restful.NewResponse(response), errors.Trace(err))
			return
		}
	case http.MethodPost:
		hasHeader := formValue(request, "has-header", "true") == "true"
		sep := formValue(request, "sep", ",")
		// field separator must be a single character
		if len(sep) != 1 {
			server.BadRequest(restful.NewResponse(response), fmt.Errorf("field separator must be a single character"))
			return
		}
		labelSep := formValue(request, "label-sep", "|")
		fmtString := formValue(request, "format", "itlc")
		file, _, err := request.FormFile("file")
		if err != nil {
			server.BadRequest(restful.NewResponse(response), err)
			return
		}
		defer file.Close()
		m.importItems(response, file, hasHeader, sep, labelSep, fmtString)
	}
}

func (m *Master) importItems(response http.ResponseWriter, file io.Reader, hasHeader bool, sep, labelSep, fmtString string) {
	lineCount := 0
	timeStart := time.Now()
	items := make([]data.Item, 0)
	err := base.ReadLines(bufio.NewScanner(file), sep, func(lineNumber int, splits []string) bool {
		var err error
		// skip header
		if hasHeader {
			hasHeader = false
			return true
		}
		splits, err = format(fmtString, "itlc", splits, lineNumber)
		if err != nil {
			server.BadRequest(restful.NewResponse(response), err)
			return false
		}
		// 1. item id
		if err = base.ValidateId(splits[0]); err != nil {
			server.BadRequest(restful.NewResponse(response),
				fmt.Errorf("invalid item id `%v` at line %d (%s)", splits[0], lineNumber, err.Error()))
			return false
		}
		item := data.Item{ItemId: splits[0]}
		// 2. timestamp
		if splits[1] != "" {
			item.Timestamp, err = dateparse.ParseAny(splits[1])
			if err != nil {
				server.BadRequest(restful.NewResponse(response),
					fmt.Errorf("failed to parse datetime `%v` at line %v", splits[1], lineNumber))
				return false
			}
		}
		// 3. labels
		if splits[2] != "" {
			item.Labels = strings.Split(splits[2], labelSep)
			for _, label := range item.Labels {
				if err = base.ValidateLabel(label); err != nil {
					server.BadRequest(restful.NewResponse(response),
						fmt.Errorf("invalid label `%v` at line %d (%s)", splits[0], lineNumber, err.Error()))
					return false
				}
			}
		}
		// 4. comment
		item.Comment = splits[3]
		items = append(items, item)
		// batch insert
		if len(items) == batchSize {
			err = m.DataClient.BatchInsertItems(items)
			if err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return false
			}
			items = nil
		}
		lineCount++
		return true
	})
	if err != nil {
		server.BadRequest(restful.NewResponse(response), err)
		return
	}
	if len(items) > 0 {
		err = m.DataClient.BatchInsertItems(items)
		if err != nil {
			server.InternalServerError(restful.NewResponse(response), err)
			return
		}
	}
	m.notifyDataImported()
	timeUsed := time.Since(timeStart)
	base.Logger().Info("complete import items",
		zap.Duration("time_used", timeUsed),
		zap.Int("num_items", lineCount))
	server.Ok(restful.NewResponse(response), server.Success{RowAffected: lineCount})
}

func format(inFmt, outFmt string, s []string, lineCount int) ([]string, error) {
	if len(s) < len(inFmt) {
		base.Logger().Error("number of fields mismatch",
			zap.Int("expect", len(inFmt)),
			zap.Int("actual", len(s)))
		return nil, fmt.Errorf("number of fields mismatch at line %v", lineCount)
	}
	if inFmt == outFmt {
		return s, nil
	}
	pool := make(map[uint8]string)
	for i := range inFmt {
		pool[inFmt[i]] = s[i]
	}
	out := make([]string, len(outFmt))
	for i, c := range outFmt {
		out[i] = pool[uint8(c)]
	}
	return out, nil
}

func formValue(request *http.Request, fieldName, defaultValue string) string {
	value := request.FormValue(fieldName)
	if value == "" {
		return defaultValue
	}
	return value
}

func (m *Master) importExportFeedback(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		var err error
		response.Header().Set("Content-Type", "text/csv")
		response.Header().Set("Content-Disposition", "attachment;filename=feedback.csv")
		// write header
		if _, err = response.Write([]byte("feedback_type,user_id,item_id,time_stamp\r\n")); err != nil {
			server.InternalServerError(restful.NewResponse(response), err)
			return
		}
		// write rows
		feedbackChan, errChan := m.DataClient.GetFeedbackStream(batchSize, nil)
		for feedback := range feedbackChan {
			for _, v := range feedback {
				if _, err = response.Write([]byte(fmt.Sprintf("%s,%s,%s,%v\r\n",
					base.Escape(v.FeedbackType), base.Escape(v.UserId), base.Escape(v.ItemId), v.Timestamp))); err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
			}
		}
		if err = <-errChan; err != nil {
			server.InternalServerError(restful.NewResponse(response), errors.Trace(err))
			return
		}
	case http.MethodPost:
		hasHeader := formValue(request, "has-header", "true") == "true"
		sep := formValue(request, "sep", ",")
		// field separator must be a single character
		if len(sep) != 1 {
			server.BadRequest(restful.NewResponse(response), fmt.Errorf("field separator must be a single character"))
			return
		}
		fmtString := formValue(request, "format", "fuit")
		// import items
		file, _, err := request.FormFile("file")
		if err != nil {
			server.BadRequest(restful.NewResponse(response), err)
			return
		}
		defer file.Close()
		m.importFeedback(response, file, hasHeader, sep, fmtString)
	}
}

func (m *Master) importFeedback(response http.ResponseWriter, file io.Reader, hasHeader bool, sep, fmtString string) {
	var err error
	scanner := bufio.NewScanner(file)
	lineCount := 0
	timeStart := time.Now()
	feedbacks := make([]data.Feedback, 0)
	for scanner.Scan() {
		line := scanner.Text()
		if hasHeader {
			hasHeader = false
			continue
		}
		splits := strings.Split(line, sep)
		// reorder fields
		splits, err = format(fmtString, "fuit", splits, lineCount)
		if err != nil {
			server.BadRequest(restful.NewResponse(response), err)
			return
		}
		feedback := data.Feedback{}
		// 1. feedback type
		feedback.FeedbackType = splits[0]
		if err = base.ValidateId(splits[0]); err != nil {
			server.BadRequest(restful.NewResponse(response),
				fmt.Errorf("invalid feedback type `%v` at line %d (%s)", splits[0], lineCount, err.Error()))
			return
		}
		// 2. user id
		if err = base.ValidateId(splits[1]); err != nil {
			server.BadRequest(restful.NewResponse(response),
				fmt.Errorf("invalid user id `%v` at line %d (%s)", splits[1], lineCount, err.Error()))
			return
		}
		feedback.UserId = splits[1]
		// 3. item id
		if err = base.ValidateId(splits[2]); err != nil {
			server.BadRequest(restful.NewResponse(response),
				fmt.Errorf("invalid item id `%v` at line %d (%s)", splits[2], lineCount, err.Error()))
			return
		}
		feedback.ItemId = splits[2]
		feedback.Timestamp, err = dateparse.ParseAny(splits[3])
		if err != nil {
			server.BadRequest(restful.NewResponse(response),
				fmt.Errorf("failed to parse datetime `%v` at line %d", splits[3], lineCount))
		}
		feedbacks = append(feedbacks, feedback)
		// batch insert
		if len(feedbacks) == batchSize {
			err = m.InsertFeedbackToCache(feedbacks)
			if err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return
			}
			feedbacks = nil
		}
		lineCount++
	}
	if err = scanner.Err(); err != nil {
		server.BadRequest(restful.NewResponse(response), err)
		return
	}
	// insert to data store
	err = m.DataClient.BatchInsertFeedback(feedbacks,
		m.GorseConfig.Database.AutoInsertUser,
		m.GorseConfig.Database.AutoInsertItem)
	if err != nil {
		server.InternalServerError(restful.NewResponse(response), err)
		return
	}
	// insert to cache store
	if len(feedbacks) > 0 {
		err = m.InsertFeedbackToCache(feedbacks)
		if err != nil {
			server.InternalServerError(restful.NewResponse(response), err)
			return
		}
	}
	m.notifyDataImported()
	timeUsed := time.Since(timeStart)
	base.Logger().Info("complete import feedback",
		zap.Duration("time_used", timeUsed),
		zap.Int("num_items", lineCount))
	server.Ok(restful.NewResponse(response), server.Success{RowAffected: lineCount})
}

//func (m *Master) exportToLibFM(response http.ResponseWriter, _ *http.Request) {
//// load dataset
//dataSet, err := click.LoadDataFromDatabase(m.DataClient,
//	m.GorseConfig.Database.PositiveFeedbackType,
//	m.GorseConfig.Database.ReadFeedbackType)
//if err != nil {
//	server.InternalServerError(restful.NewResponse(response), err)
//}
//// write dataset
//response.Header().Set("Content-Type", "text/plain")
//response.Header().Set("Content-Disposition", "attachment;filename=libfm.txt")
//for i := range dataSet.Features {
//	builder := strings.Builder{}
//	builder.WriteString(fmt.Sprintf("%f", dataSet.Target[i]))
//	for _, j := range dataSet.Features[i] {
//		builder.WriteString(fmt.Sprintf(" %d:1", j))
//	}
//	builder.WriteString("\r\n")
//	_, err = response.Write([]byte(builder.String()))
//	if err != nil {
//		server.InternalServerError(restful.NewResponse(response), err)
//	}
//}
//}
