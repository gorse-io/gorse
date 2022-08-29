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
	"github.com/araddon/dateparse"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/gorilla/securecookie"
	_ "github.com/gorse-io/dashboard"
	"github.com/juju/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/rakyll/statik/fs"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func (m *Master) CreateWebService() {
	ws := m.WebService
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	ws.Path("/api/")
	ws.Filter(m.LoginFilter)

	ws.Route(ws.GET("/dashboard/cluster").To(m.getCluster).
		Doc("Get nodes in the cluster.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Writes([]Node{}))
	ws.Route(ws.GET("/dashboard/categories").To(m.getCategories).
		Doc("Get categories of items.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Writes([]string{}))
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
		Writes([]task.Task{}))
	ws.Route(ws.GET("/dashboard/rates").To(m.getRates).
		Doc("Get positive feedback rates.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Writes(map[string][]server.Measurement{}))
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
	ws.Route(ws.GET("/dashboard/popular/").To(m.getPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/popular/{category}").To(m.getPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	// Get latest items
	ws.Route(ws.GET("/dashboard/latest/").To(m.getLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/latest/{category}").To(m.getLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}/{recommender}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("recommender", "one of `final`, `collaborative`, `user_based` and `item_based`").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}/{recommender}/{category}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("recommender", "one of `final`, `collaborative`, `user_based` and `item_based`").DataType("string")).
		Param(ws.PathParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/item/{item-id}/neighbors").To(m.getItemNeighbors).
		Doc("get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/item/{item-id}/neighbors/{category}").To(m.getItemCategorizedNeighbors).
		Doc("get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.PathParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/user/{user-id}/neighbors").To(m.getUserNeighbors).
		Doc("get neighbors of a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
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
	container := restful.NewContainer()
	container.Handle("/", http.HandlerFunc(m.dashboard))
	container.Handle("/login", http.HandlerFunc(m.login))
	container.Handle("/logout", http.HandlerFunc(m.logout))
	container.Handle("/api/bulk/users", http.HandlerFunc(m.importExportUsers))
	container.Handle("/api/bulk/items", http.HandlerFunc(m.importExportItems))
	container.Handle("/api/bulk/feedback", http.HandlerFunc(m.importExportFeedback))
	//http.HandleFunc("/api/bulk/libfm", m.exportToLibFM)
	m.RestServer.StartHttpServer(container)
}

var (
	cookieHandler = securecookie.New(
		securecookie.GenerateRandomKey(64),
		securecookie.GenerateRandomKey(32))
	staticFileSystem http.FileSystem
	staticFileServer http.Handler
)

func init() {
	var err error
	staticFileSystem, err = fs.New()
	if err != nil {
		log.Logger().Fatal("failed to load statik files", zap.Error(err))
	}
	staticFileServer = http.FileServer(&SinglePageAppFileSystem{staticFileSystem})
}

// Taken from https://github.com/mytrile/nocache
var noCacheHeaders = map[string]string{
	"Expires":         time.Unix(0, 0).Format(time.RFC1123),
	"Cache-Control":   "no-cache, no-store, no-transform, must-revalidate, private, max-age=0",
	"Pragma":          "no-cache",
	"X-Accel-Expires": "0",
}

var etagHeaders = []string{
	"ETag",
	"If-Modified-Since",
	"If-Match",
	"If-None-Match",
	"If-Range",
	"If-Unmodified-Since",
}

// noCache is a simple piece of middleware that sets a number of HTTP headers to prevent
// a router (or subrouter) from being cached by an upstream proxy and/or client.
//
// As per http://wiki.nginx.org/HttpProxyModule - noCache sets:
//
//	Expires: Thu, 01 Jan 1970 00:00:00 UTC
//	Cache-Control: no-cache, private, max-age=0
//	X-Accel-Expires: 0
//	Pragma: no-cache (for HTTP/1.0 proxies/clients)
func noCache(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// Delete any ETag headers that may have been set
		for _, v := range etagHeaders {
			if r.Header.Get(v) != "" {
				r.Header.Del(v)
			}
		}
		// Set our noCache headers
		for k, v := range noCacheHeaders {
			w.Header().Set(k, v)
		}
		h.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func (m *Master) dashboard(response http.ResponseWriter, request *http.Request) {
	_, err := staticFileSystem.Open(request.RequestURI)
	if request.RequestURI == "/" || os.IsNotExist(err) {
		if !m.checkLogin(request) {
			http.Redirect(response, request, "/login", http.StatusFound)
			log.Logger().Info(fmt.Sprintf("%s %s", request.Method, request.URL), zap.Int("status_code", http.StatusFound))
			return
		}
		noCache(staticFileServer).ServeHTTP(response, request)
		return
	}
	staticFileServer.ServeHTTP(response, request)
}

func (m *Master) login(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		log.Logger().Info("GET /login", zap.Int("status_code", http.StatusOK))
		staticFileServer.ServeHTTP(response, request)
	case http.MethodPost:
		name := request.FormValue("user_name")
		pass := request.FormValue("password")
		if name == m.Config.Master.DashboardUserName && pass == m.Config.Master.DashboardPassword {
			value := map[string]string{
				"user_name": name,
				"password":  pass,
			}
			if encoded, err := cookieHandler.Encode("session", value); err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return
			} else {
				cookie := &http.Cookie{
					Name:  "session",
					Value: encoded,
					Path:  "/",
				}
				http.SetCookie(response, cookie)
				http.Redirect(response, request, "/", http.StatusFound)
				log.Logger().Info("POST /login", zap.Int("status_code", http.StatusFound))
				return
			}
		} else {
			http.Redirect(response, request, "login?msg=incorrect", http.StatusFound)
			log.Logger().Info("POST /login", zap.Int("status_code", http.StatusUnauthorized))
			return
		}
	default:
		server.BadRequest(restful.NewResponse(response), errors.New("unsupported method"))
	}
}

func (m *Master) logout(response http.ResponseWriter, request *http.Request) {
	cookie := &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(response, cookie)
	http.Redirect(response, request, "/login", http.StatusFound)
	log.Logger().Info(fmt.Sprintf("%s %s", request.Method, request.RequestURI), zap.Int("status_code", http.StatusFound))
}

func (m *Master) LoginFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if m.checkLogin(req.Request) {
		req.Request.Header.Set("X-API-Key", m.Config.Server.APIKey)
	}
	chain.ProcessFilter(req, resp)
}

func (m *Master) checkLogin(request *http.Request) bool {
	if m.Config.Master.DashboardUserName == "" || m.Config.Master.DashboardPassword == "" {
		return true
	}
	if cookie, err := request.Cookie("session"); err == nil {
		cookieValue := make(map[string]string)
		if err = cookieHandler.Decode("session", cookie.Value, &cookieValue); err == nil {
			userName := cookieValue["user_name"]
			password := cookieValue["password"]
			if userName == m.Config.Master.DashboardUserName && password == m.Config.Master.DashboardPassword {
				return true
			}
		}
	}
	return false
}

func (m *Master) getCategories(_ *restful.Request, response *restful.Response) {
	categories, err := m.CacheClient.GetSet(cache.ItemCategories)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	server.Ok(response, categories)
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

func formatConfig(configMap map[string]interface{}) map[string]interface{} {
	return lo.MapValues(configMap, func(v interface{}, _ string) interface{} {
		switch value := v.(type) {
		case time.Duration:
			s := value.String()
			if strings.HasSuffix(s, "m0s") {
				s = s[:len(s)-2]
			}
			if strings.HasSuffix(s, "h0m") {
				s = s[:len(s)-2]
			}
			return s
		case map[string]interface{}:
			return formatConfig(value)
		default:
			return v
		}
	})
}

func (m *Master) getConfig(_ *restful.Request, response *restful.Response) {
	var configMap map[string]interface{}
	err := mapstructure.Decode(m.Config, &configMap)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	server.Ok(response, formatConfig(configMap))
}

type Status struct {
	BinaryVersion           string
	NumServers              int
	NumWorkers              int
	NumUsers                int
	NumItems                int
	NumUserLabels           int
	NumItemLabels           int
	NumTotalPosFeedback     int
	NumValidPosFeedback     int
	NumValidNegFeedback     int
	PopularItemsUpdateTime  time.Time
	LatestItemsUpdateTime   time.Time
	MatchingModelFitTime    time.Time
	MatchingModelScore      ranking.Score
	RankingModelFitTime     time.Time
	RankingModelScore       click.Score
	UserNeighborIndexRecall float32
	ItemNeighborIndexRecall float32
	MatchingIndexRecall     float32
}

func (m *Master) getStats(_ *restful.Request, response *restful.Response) {
	status := Status{BinaryVersion: version.Version}
	var err error
	// read number of users
	if status.NumUsers, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.NumUsers)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of users", zap.Error(err))
	}
	// read number of items
	if status.NumItems, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.NumItems)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of items", zap.Error(err))
	}
	// read number of user labels
	if status.NumUserLabels, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.NumUserLabels)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of user labels", zap.Error(err))
	}
	// read number of item labels
	if status.NumItemLabels, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.NumItemLabels)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of item labels", zap.Error(err))
	}
	// read number of total positive feedback
	if status.NumTotalPosFeedback, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.NumTotalPosFeedbacks)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of total positive feedbacks", zap.Error(err))
	}
	// read number of valid positive feedback
	if status.NumValidPosFeedback, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.NumValidPosFeedbacks)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of valid positive feedbacks", zap.Error(err))
	}
	// read number of valid negative feedback
	if status.NumValidNegFeedback, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.NumValidNegFeedbacks)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of valid negative feedbacks", zap.Error(err))
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
	// read popular items update time
	if status.PopularItemsUpdateTime, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.LastUpdatePopularItemsTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get popular items update time", zap.Error(err))
	}
	// read the latest items update time
	if status.LatestItemsUpdateTime, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.LastUpdateLatestItemsTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get latest items update time", zap.Error(err))
	}
	status.MatchingModelScore = m.rankingScore
	status.RankingModelScore = m.clickScore
	// read last fit matching model time
	if status.MatchingModelFitTime, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.LastFitMatchingModelTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get last fit matching model time", zap.Error(err))
	}
	// read last fit ranking model time
	if status.RankingModelFitTime, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.LastFitRankingModelTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get last fit ranking model time", zap.Error(err))
	}
	// read user neighbor index recall
	var temp string
	if m.Config.Recommend.UserNeighbors.EnableIndex {
		if temp, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.UserNeighborIndexRecall)).String(); err != nil {
			log.ResponseLogger(response).Warn("failed to get user neighbor index recall", zap.Error(err))
		} else {
			status.UserNeighborIndexRecall = encoding.ParseFloat32(temp)
		}
	}
	// read item neighbor index recall
	if m.Config.Recommend.ItemNeighbors.EnableIndex {
		if temp, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.ItemNeighborIndexRecall)).String(); err != nil {
			log.ResponseLogger(response).Warn("failed to get item neighbor index recall", zap.Error(err))
		} else {
			status.ItemNeighborIndexRecall = encoding.ParseFloat32(temp)
		}
	}
	// read matching index recall
	if m.Config.Recommend.Collaborative.EnableIndex {
		if temp, err = m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.MatchingIndexRecall)).String(); err != nil {
			log.ResponseLogger(response).Warn("failed to get matching index recall", zap.Error(err))
		} else {
			status.MatchingIndexRecall = encoding.ParseFloat32(temp)
		}
	}
	server.Ok(response, status)
}

func (m *Master) getTasks(_ *restful.Request, response *restful.Response) {
	// List workers
	workers := make([]string, 0)
	m.nodesInfoMutex.RLock()
	for _, info := range m.nodesInfo {
		if info.Type == WorkerNode {
			workers = append(workers, info.Name)
		}
	}
	m.nodesInfoMutex.RUnlock()
	// List tasks
	tasks := m.taskMonitor.List(workers...)
	server.Ok(response, tasks)
}

func (m *Master) getRates(request *restful.Request, response *restful.Response) {
	// Parse parameters
	n, err := server.ParseInt(request, "n", 100)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	measurements := make(map[string][]server.Measurement, len(m.Config.Recommend.DataSource.PositiveFeedbackTypes))
	for _, feedbackType := range m.Config.Recommend.DataSource.PositiveFeedbackTypes {
		measurements[feedbackType], err = m.RestServer.GetMeasurements(cache.Key(PositiveFeedbackRate, feedbackType), n)
		if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, measurements)
}

type UserIterator struct {
	Cursor string
	Users  []User
}

type User struct {
	data.User
	LastActiveTime time.Time
	LastUpdateTime time.Time
}

func (m *Master) getUser(request *restful.Request, response *restful.Response) {
	// get user id
	userId := request.PathParameter("user-id")
	// get user
	user, err := m.DataClient.GetUser(userId)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			server.PageNotFound(response, err)
		} else {
			server.InternalServerError(response, err)
		}
		return
	}
	detail := User{User: user}
	if detail.LastActiveTime, err = m.CacheClient.Get(cache.Key(cache.LastModifyUserTime, user.UserId)).Time(); err != nil && !errors.Is(err, errors.NotFound) {
		server.InternalServerError(response, err)
		return
	}
	if detail.LastUpdateTime, err = m.CacheClient.Get(cache.Key(cache.LastUpdateUserRecommendTime, user.UserId)).Time(); err != nil && !errors.Is(err, errors.NotFound) {
		server.InternalServerError(response, err)
		return
	}
	server.Ok(response, detail)
}

func (m *Master) getUsers(request *restful.Request, response *restful.Response) {
	// Authorize
	cursor := request.QueryParameter("cursor")
	n, err := server.ParseInt(request, "n", m.Config.Server.DefaultN)
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
		if details[i].LastActiveTime, err = m.CacheClient.Get(cache.Key(cache.LastModifyUserTime, user.UserId)).Time(); err != nil && !errors.Is(err, errors.NotFound) {
			server.InternalServerError(response, err)
			return
		}
		if details[i].LastUpdateTime, err = m.CacheClient.Get(cache.Key(cache.LastUpdateUserRecommendTime, user.UserId)).Time(); err != nil && !errors.Is(err, errors.NotFound) {
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
	category := request.PathParameter("category")
	n, err := server.ParseInt(request, "n", m.Config.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	var results []string
	switch recommender {
	case "offline":
		results, err = m.Recommend(response, userId, category, n, m.RecommendOffline)
	case "collaborative":
		results, err = m.Recommend(response, userId, category, n, m.RecommendCollaborative)
	case "user_based":
		results, err = m.Recommend(response, userId, category, n, m.RecommendUserBased)
	case "item_based":
		results, err = m.Recommend(response, userId, category, n, m.RecommendItemBased)
	case "_":
		recommenders := []server.Recommender{m.RecommendOffline}
		for _, recommender := range m.Config.Recommend.Online.FallbackRecommend {
			switch recommender {
			case "collaborative":
				recommenders = append(recommenders, m.RecommendCollaborative)
			case "item_based":
				recommenders = append(recommenders, m.RecommendItemBased)
			case "user_based":
				recommenders = append(recommenders, m.RecommendUserBased)
			case "latest":
				recommenders = append(recommenders, m.RecommendLatest)
			case "popular":
				recommenders = append(recommenders, m.RecommendPopular)
			default:
				server.InternalServerError(response, fmt.Errorf("unknown fallback recommendation method `%s`", recommender))
				return
			}
		}
		results, err = m.Recommend(response, userId, category, n, recommenders...)
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
	feedback, err := m.DataClient.GetUserFeedback(userId, false, feedbackType)
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
		if errors.Is(err, errors.NotFound) {
			details[i].Item = data.Item{ItemId: feedback[i].ItemId, Comment: "** This item doesn't exist in Gorse **"}
		} else if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, details)
}

func (m *Master) getSort(key, category string, isItem bool, request *restful.Request, response *restful.Response, retType interface{}) {
	var n, offset int
	var err error
	// read arguments
	if offset, err = server.ParseInt(request, "offset", 0); err != nil {
		server.BadRequest(response, err)
		return
	}
	if n, err = server.ParseInt(request, "n", m.Config.Server.DefaultN); err != nil {
		server.BadRequest(response, err)
		return
	}
	// Get the popular list
	scores, err := m.CacheClient.GetSorted(cache.Key(key, category), offset, m.Config.Recommend.CacheSize)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	if isItem {
		scores = m.FilterOutHiddenScores(response, scores, category)
	}
	if n > 0 && len(scores) > n {
		scores = scores[:n]
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
		log.ResponseLogger(response).Fatal("unknown return type", zap.Any("ret_type", reflect.TypeOf(retType)))
	}
}

// getPopular gets popular items from database.
func (m *Master) getPopular(request *restful.Request, response *restful.Response) {
	category := request.PathParameter("category")
	m.getSort(cache.PopularItems, category, true, request, response, data.Item{})
}

func (m *Master) getLatest(request *restful.Request, response *restful.Response) {
	category := request.PathParameter("category")
	m.getSort(cache.LatestItems, category, true, request, response, data.Item{})
}

func (m *Master) getItemNeighbors(request *restful.Request, response *restful.Response) {
	itemId := request.PathParameter("item-id")
	m.getSort(cache.Key(cache.ItemNeighbors, itemId), "", true, request, response, data.Item{})
}

func (m *Master) getItemCategorizedNeighbors(request *restful.Request, response *restful.Response) {
	itemId := request.PathParameter("item-id")
	category := request.PathParameter("category")
	m.getSort(cache.Key(cache.ItemNeighbors, itemId), category, true, request, response, data.Item{})
}

func (m *Master) getUserNeighbors(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")
	m.getSort(cache.Key(cache.UserNeighbors, userId), "", false, request, response, data.User{})
}

func (m *Master) importExportUsers(response http.ResponseWriter, request *http.Request) {
	if !m.checkLogin(request) {
		resp := restful.NewResponse(response)
		err := resp.WriteErrorString(http.StatusUnauthorized, "unauthorized")
		if err != nil {
			server.InternalServerError(resp, err)
			return
		}
		return
	}
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
	log.Logger().Info("complete import users",
		zap.Duration("time_used", timeUsed),
		zap.Int("num_users", lineCount))
	server.Ok(restful.NewResponse(response), server.Success{RowAffected: lineCount})
}

func (m *Master) importExportItems(response http.ResponseWriter, request *http.Request) {
	if !m.checkLogin(request) {
		resp := restful.NewResponse(response)
		err := resp.WriteErrorString(http.StatusUnauthorized, "unauthorized")
		if err != nil {
			server.InternalServerError(resp, err)
			return
		}
		return
	}
	switch request.Method {
	case http.MethodGet:
		var err error
		response.Header().Set("Content-Type", "text/csv")
		response.Header().Set("Content-Disposition", "attachment;filename=items.csv")
		// write header
		if _, err = response.Write([]byte("item_id,is_hidden,categories,time_stamp,labels,description\r\n")); err != nil {
			server.InternalServerError(restful.NewResponse(response), err)
			return
		}
		// write rows
		itemChan, errChan := m.DataClient.GetItemStream(batchSize, nil)
		for items := range itemChan {
			for _, item := range items {
				if _, err = response.Write([]byte(fmt.Sprintf("%s,%t,%s,%v,%s,%s\r\n",
					base.Escape(item.ItemId), item.IsHidden, base.Escape(strings.Join(item.Categories, "|")),
					item.Timestamp, base.Escape(strings.Join(item.Labels, "|")), base.Escape(item.Comment)))); err != nil {
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
		fmtString := formValue(request, "format", "ihctld")
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
		splits, err = format(fmtString, "ihctld", splits, lineNumber)
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
		// 2. hidden
		if splits[1] != "" {
			item.IsHidden, err = strconv.ParseBool(splits[1])
			if err != nil {
				server.BadRequest(restful.NewResponse(response),
					fmt.Errorf("invalid hidden value `%v` at line %d (%s)", splits[1], lineNumber, err.Error()))
				return false
			}
		}
		// 3. categories
		if splits[2] != "" {
			item.Categories = strings.Split(splits[2], labelSep)
			for _, category := range item.Categories {
				if err = base.ValidateId(category); err != nil {
					server.BadRequest(restful.NewResponse(response),
						fmt.Errorf("invalid category `%v` at line %d (%s)", category, lineNumber, err.Error()))
					return false
				}
			}
		}
		// 4. timestamp
		if splits[3] != "" {
			item.Timestamp, err = dateparse.ParseAny(splits[3])
			if err != nil {
				server.BadRequest(restful.NewResponse(response),
					fmt.Errorf("failed to parse datetime `%v` at line %v", splits[1], lineNumber))
				return false
			}
		}
		// 5. labels
		if splits[4] != "" {
			item.Labels = strings.Split(splits[4], labelSep)
			for _, label := range item.Labels {
				if err = base.ValidateLabel(label); err != nil {
					server.BadRequest(restful.NewResponse(response),
						fmt.Errorf("invalid label `%v` at line %d (%s)", label, lineNumber, err.Error()))
					return false
				}
			}
		}
		// 6. comment
		item.Comment = splits[5]
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
	log.Logger().Info("complete import items",
		zap.Duration("time_used", timeUsed),
		zap.Int("num_items", lineCount))
	server.Ok(restful.NewResponse(response), server.Success{RowAffected: lineCount})
}

func format(inFmt, outFmt string, s []string, lineCount int) ([]string, error) {
	if len(s) < len(inFmt) {
		log.Logger().Error("number of fields mismatch",
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
	if !m.checkLogin(request) {
		resp := restful.NewResponse(response)
		err := resp.WriteErrorString(http.StatusUnauthorized, "unauthorized")
		if err != nil {
			server.InternalServerError(resp, err)
			return
		}
		return
	}
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
	err = base.ReadLines(scanner, sep, func(lineNumber int, splits []string) bool {
		if hasHeader {
			hasHeader = false
			return true
		}
		// reorder fields
		splits, err = format(fmtString, "fuit", splits, lineNumber)
		if err != nil {
			server.BadRequest(restful.NewResponse(response), err)
			return false
		}
		feedback := data.Feedback{}
		// 1. feedback type
		feedback.FeedbackType = splits[0]
		if err = base.ValidateId(splits[0]); err != nil {
			server.BadRequest(restful.NewResponse(response),
				fmt.Errorf("invalid feedback type `%v` at line %d (%s)", splits[0], lineNumber, err.Error()))
			return false
		}
		// 2. user id
		if err = base.ValidateId(splits[1]); err != nil {
			server.BadRequest(restful.NewResponse(response),
				fmt.Errorf("invalid user id `%v` at line %d (%s)", splits[1], lineNumber, err.Error()))
			return false
		}
		feedback.UserId = splits[1]
		// 3. item id
		if err = base.ValidateId(splits[2]); err != nil {
			server.BadRequest(restful.NewResponse(response),
				fmt.Errorf("invalid item id `%v` at line %d (%s)", splits[2], lineNumber, err.Error()))
			return false
		}
		feedback.ItemId = splits[2]
		feedback.Timestamp, err = dateparse.ParseAny(splits[3])
		if err != nil {
			server.BadRequest(restful.NewResponse(response),
				fmt.Errorf("failed to parse datetime `%v` at line %d", splits[3], lineNumber))
			return false
		}
		feedbacks = append(feedbacks, feedback)
		// batch insert
		if len(feedbacks) == batchSize {
			err = m.InsertFeedbackToCache(feedbacks)
			if err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return false
			}
			feedbacks = nil
		}
		lineCount++
		return true
	})
	if err != nil {
		server.BadRequest(restful.NewResponse(response), err)
		return
	}
	// insert to data store
	err = m.DataClient.BatchInsertFeedback(feedbacks,
		m.Config.Server.AutoInsertUser,
		m.Config.Server.AutoInsertItem, true)
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
	log.Logger().Info("complete import feedback",
		zap.Duration("time_used", timeUsed),
		zap.Int("num_items", lineCount))
	server.Ok(restful.NewResponse(response), server.Success{RowAffected: lineCount})
}

//func (m *Master) exportToLibFM(response http.ResponseWriter, _ *http.Request) {
//// load dataset
//dataSet, err := click.LoadDataFromDatabase(m.DataClient,
//	m.Config.Database.PositiveFeedbackTypes,
//	m.Config.Database.ReadFeedbackType)
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
