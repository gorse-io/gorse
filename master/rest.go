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
	"fmt"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	_ "github.com/gorse-io/dashboard"
	"github.com/rakyll/statik/fs"
	"github.com/scylladb/go-set"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

func (m *Master) StartHttpServer() {

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

	statikFS, err := fs.New()
	if err != nil {
		base.Logger().Fatal("failed to load statik files", zap.Error(err))
	}
	http.Handle("/", http.FileServer(statikFS))

	http.HandleFunc("/api/bulk/items", m.importExportItems)
	http.HandleFunc("/api/bulk/feedback", m.importExportFeedback)

	m.RestServer.StartHttpServer()
}

func (m *Master) getCluster(request *restful.Request, response *restful.Response) {
	// collect nodes
	workers := make([]*Node, 0)
	servers := make([]*Node, 0)
	m.nodesInfoMutex.Lock()
	for _, info := range m.nodesInfo {
		switch info.Type {
		case WorkerNode:
			workers = append(workers, info)
		case ServerNode:
			servers = append(servers, info)
		}
	}
	m.nodesInfoMutex.Unlock()
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
	PRModel        string
	CTRModel       string
}

func (m *Master) getStats(request *restful.Request, response *restful.Response) {
	status := Status{}
	var err error
	// read number of users
	status.NumUsers, err = m.CacheStore.GetString(cache.GlobalMeta, cache.NumUsers)
	if err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	// read number of items
	status.NumItems, err = m.CacheStore.GetString(cache.GlobalMeta, cache.NumItems)
	if err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	// read number of positive feedback
	status.NumPosFeedback, err = m.CacheStore.GetString(cache.GlobalMeta, cache.NumPositiveFeedback)
	if err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	status.PRModel = m.prModelName
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

func (m *Master) getUsers(request *restful.Request, response *restful.Response) {
	// Authorize
	cursor := request.QueryParameter("cursor")
	n, err := server.ParseInt(request, "n", m.GorseConfig.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	// get all users
	cursor, users, err := m.DataStore.GetUsers(cursor, n)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	details := make([]User, len(users))
	for i, user := range users {
		details[i].User = user
		if details[i].LastActiveTime, err = m.CacheStore.GetString(cache.LastActiveTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
			server.InternalServerError(response, err)
			return
		}
		if details[i].LastUpdateTime, err = m.CacheStore.GetString(cache.LastUpdateRecommendTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, UserIterator{Cursor: cursor, Users: details})
}

func (m *Master) getUser(request *restful.Request, response *restful.Response) {
	// get user id
	userId := request.PathParameter("user-id")
	// get user
	user, err := m.DataStore.GetUser(userId)
	if err != nil {
		if err.Error() == data.ErrUserNotExist {
			server.PageNotFound(response, err)
		} else {
			server.InternalServerError(response, err)
		}
		return
	}
	detail := User{User: user}
	if detail.LastActiveTime, err = m.CacheStore.GetString(cache.LastActiveTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	if detail.LastUpdateTime, err = m.CacheStore.GetString(cache.LastUpdateRecommendTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	server.Ok(response, detail)
}

func (m *Master) getRecommend(request *restful.Request, response *restful.Response) {
	// parse arguments
	userId := request.PathParameter("user-id")
	n, err := server.ParseInt(request, "n", m.GorseConfig.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	// load offline recommendation
	start := time.Now()
	itemsChan := make(chan []string, 1)
	errChan := make(chan error, 1)
	go func() {
		var collaborativeFilteringItems []string
		collaborativeFilteringItems, err = m.CacheStore.GetList(cache.CollaborativeItems, userId, 0, m.GorseConfig.Database.CacheSize)
		if err != nil {
			itemsChan <- nil
			errChan <- err
		} else {
			itemsChan <- collaborativeFilteringItems
			errChan <- nil
			if len(collaborativeFilteringItems) == 0 {
				base.Logger().Warn("empty collaborative filtering", zap.String("user_id", userId))
			}
		}
	}()
	// load historical feedback
	userFeedback, err := m.DataStore.GetUserFeedback(userId, nil)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	excludeSet := set.NewStringSet()
	for _, feedback := range userFeedback {
		excludeSet.Add(feedback.ItemId)
	}
	// remove historical items
	items := <-itemsChan
	err = <-errChan
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	results := make([]string, 0, len(items))
	for _, itemId := range items {
		if !excludeSet.Has(itemId) {
			results = append(results, itemId)
		}
	}
	if len(results) > n {
		results = results[:n]
	}
	spent := time.Since(start)
	base.Logger().Info("complete recommendation",
		zap.Duration("total_time", spent))
	// Send result
	details := make([]data.Item, len(results))
	for i := range results {
		details[i], err = m.DataStore.GetItem(results[i])
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

// get feedback by user-id
func (m *Master) getFeedbackByUser(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")
	feedback, err := m.DataStore.GetUserFeedback(userId, nil)
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
		details[i].Item, err = m.DataStore.GetItem(feedback[i].ItemId)
		if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, details)
}

// get feedback by user-id with feedback type
func (m *Master) getTypedFeedbackByUser(request *restful.Request, response *restful.Response) {
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	feedback, err := m.DataStore.GetUserFeedback(userId, &feedbackType)
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
		details[i].Item, err = m.DataStore.GetItem(feedback[i].ItemId)
		if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, details)
}

func (m *Master) getList(prefix string, name string, request *restful.Request, response *restful.Response) {
	var begin, end int
	var err error
	if begin, err = server.ParseInt(request, "begin", 0); err != nil {
		server.BadRequest(response, err)
		return
	}
	if end, err = server.ParseInt(request, "end", m.GorseConfig.Server.DefaultN-1); err != nil {
		server.BadRequest(response, err)
		return
	}
	// Get the popular list
	items, err := m.CacheStore.GetList(prefix, name, begin, end)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	// Send result
	details := make([]data.Item, len(items))
	for i := range items {
		details[i], err = m.DataStore.GetItem(items[i])
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
		response.Header().Set("Content-Disposition", fmt.Sprint("attachment;filename=items.csv"))
		// write header
		if _, err = response.Write([]byte("item_id,time_stamp,labels\n")); err != nil {
			server.InternalServerError(restful.NewResponse(response), err)
			return
		}
		// write rows
		var cursor string
		const batchSize = 1024
		for {
			var items []data.Item
			cursor, items, err = m.DataStore.GetItems(cursor, batchSize, nil)
			if err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return
			}
			for _, item := range items {
				if _, err = response.Write([]byte(fmt.Sprintf("%v,%v,%v\n",
					item.ItemId, item.Timestamp, strings.Join(item.Labels, "|")))); err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
			}
			if cursor == "" {
				break
			}
		}
	}
}

func (m *Master) importExportFeedback(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		var err error
		response.Header().Set("Content-Type", "text/csv")
		response.Header().Set("Content-Disposition", fmt.Sprint("attachment;filename=feedback.csv"))
		// write header
		if _, err = response.Write([]byte("feedback_type,user_id,item_id,time_stamp\n")); err != nil {
			server.InternalServerError(restful.NewResponse(response), err)
			return
		}
		// write rows
		var cursor string
		const batchSize = 1024
		for {
			var feedback []data.Feedback
			cursor, feedback, err = m.DataStore.GetFeedback(cursor, batchSize, nil, nil)
			if err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return
			}
			for _, v := range feedback {
				if _, err = response.Write([]byte(fmt.Sprintf("%v%v%v%v\n",
					v.FeedbackType, v.UserId, v.ItemId, v.Timestamp))); err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
			}
			if cursor == "" {
				break
			}
		}
	}
}
