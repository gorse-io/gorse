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
	"io"
	"net/http"
	"os"
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
		Doc("Get global statistics.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes(Status{}))
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
	ws.Route(ws.GET("/dashboard/recommend/{user-id}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/neighbors/{item-id}").To(m.getNeighbors).
		Doc("get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.QueryParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
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
	http.HandleFunc("/api/bulk/items", m.importExportItems)
	http.HandleFunc("/api/bulk/feedback", m.importExportFeedback)
	m.RestServer.StartHttpServer()
}

func (m *Master) getCluster(request *restful.Request, response *restful.Response) {
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

func (m *Master) getConfig(request *restful.Request, response *restful.Response) {
	server.Ok(response, m.GorseConfig)
}

type Status struct {
	NumUsers       string
	NumItems       string
	NumPosFeedback string
	RankingScore   float32
	ClickScore     float32
}

func (m *Master) getStats(request *restful.Request, response *restful.Response) {
	status := Status{}
	var err error
	// read number of users
	status.NumUsers, err = m.CacheClient.GetString(cache.GlobalMeta, cache.NumUsers)
	if err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	// read number of items
	status.NumItems, err = m.CacheClient.GetString(cache.GlobalMeta, cache.NumItems)
	if err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	// read number of positive feedback
	status.NumPosFeedback, err = m.CacheClient.GetString(cache.GlobalMeta, cache.NumPositiveFeedback)
	if err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	status.RankingScore = m.rankingScore.Precision
	status.ClickScore = m.clickScore.Precision
	server.Ok(response, status)
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
	if detail.LastActiveTime, err = m.CacheClient.GetString(cache.LastActiveTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	if detail.LastUpdateTime, err = m.CacheClient.GetString(cache.LastUpdateRecommendTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
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
		if details[i].LastActiveTime, err = m.CacheClient.GetString(cache.LastActiveTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
			server.InternalServerError(response, err)
			return
		}
		if details[i].LastUpdateTime, err = m.CacheClient.GetString(cache.LastUpdateRecommendTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, UserIterator{Cursor: cursor, Users: details})
}

func (m *Master) getRecommend(request *restful.Request, response *restful.Response) {
	// parse arguments
	userId := request.PathParameter("user-id")
	n, err := server.ParseInt(request, "n", m.GorseConfig.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	results, err := m.Recommend(userId, n)
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

func (m *Master) getList(prefix, name string, request *restful.Request, response *restful.Response) {
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
	items, err := m.CacheClient.GetScores(prefix, name, begin, end)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	// Send result
	details := make([]data.Item, len(items))
	for i := range items {
		details[i], err = m.DataClient.GetItem(items[i].ItemId)
		if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, details)
}

// getPopular gets popular items from database.
func (m *Master) getPopular(request *restful.Request, response *restful.Response) {
	m.getList(cache.PopularItems, "", request, response)
}

func (m *Master) getLatest(request *restful.Request, response *restful.Response) {
	m.getList(cache.LatestItems, "", request, response)
}

func (m *Master) getNeighbors(request *restful.Request, response *restful.Response) {
	itemId := request.PathParameter("item-id")
	m.getList(cache.SimilarItems, itemId, request, response)
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
		var cursor string
		const batchSize = 1024
		for {
			var items []data.Item
			cursor, items, err = m.DataClient.GetItems(cursor, batchSize, nil)
			if err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return
			}
			for _, item := range items {
				if _, err = response.Write([]byte(fmt.Sprintf("%s,%v,%s,%s\r\n",
					base.Escape(item.ItemId), item.Timestamp, base.Escape(strings.Join(item.Labels, "|")), base.Escape(item.Comment)))); err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
			}
			if cursor == "" {
				break
			}
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
	err := base.ReadLines(bufio.NewScanner(file), sep[0], func(lineNumber int, splits []string) bool {
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
						fmt.Errorf("invalid item id `%v` at line %d (%s)", splits[0], lineNumber, err.Error()))
					return false
				}
			}
		}
		// 4. comment
		item.Comment = splits[3]
		items = append(items, item)
		//err = m.DataClient.InsertItem(item)
		//if err != nil {
		//	server.InternalServerError(restful.NewResponse(response), err)
		//	return false
		//}
		lineCount++
		return true
	})
	if err != nil {
		server.BadRequest(restful.NewResponse(response), err)
		return
	}
	err = m.DataClient.BatchInsertItem(items)
	if err != nil {
		server.InternalServerError(restful.NewResponse(response), err)
		return
	}
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
		var cursor string
		const batchSize = 1024
		for {
			var feedback []data.Feedback
			cursor, feedback, err = m.DataClient.GetFeedback(cursor, batchSize, nil)
			if err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return
			}
			for _, v := range feedback {
				if _, err = response.Write([]byte(fmt.Sprintf("%s,%s,%s,%v\r\n",
					base.Escape(v.FeedbackType), base.Escape(v.UserId), base.Escape(v.ItemId), v.Timestamp))); err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
			}
			if cursor == "" {
				break
			}
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
	err = m.InsertFeedbackToCache(feedbacks)
	if err != nil {
		server.InternalServerError(restful.NewResponse(response), err)
		return
	}
	timeUsed := time.Since(timeStart)
	base.Logger().Info("complete import feedback",
		zap.Duration("time_used", timeUsed),
		zap.Int("num_items", lineCount))
	server.Ok(restful.NewResponse(response), server.Success{RowAffected: lineCount})
}
