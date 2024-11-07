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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	mapset "github.com/deckarep/golang-set/v2"
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
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
)

func (m *Master) CreateWebService() {
	ws := m.WebService
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	ws.Path("/api/")
	ws.Filter(m.LoginFilter)

	ws.Route(ws.GET("/dashboard/cluster").To(m.getCluster).
		Doc("Get nodes in the cluster.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Returns(http.StatusOK, "OK", []Node{}).
		Writes([]Node{}))
	ws.Route(ws.GET("/dashboard/categories").To(m.getCategories).
		Doc("Get categories of items.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Returns(http.StatusOK, "OK", []string{}).
		Writes([]string{}))
	ws.Route(ws.GET("/dashboard/config").To(m.getConfig).
		Doc("Get config.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Returns(http.StatusOK, "OK", config.Config{}).
		Writes(config.Config{}))
	ws.Route(ws.GET("/dashboard/stats").To(m.getStats).
		Doc("Get global status.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Returns(http.StatusOK, "OK", Status{}).
		Writes(Status{}))
	ws.Route(ws.GET("/dashboard/tasks").To(m.getTasks).
		Doc("Get tasks.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Returns(http.StatusOK, "OK", []progress.Progress{}).
		Writes([]progress.Progress{}))
	ws.Route(ws.GET("/dashboard/rates").To(m.getRates).
		Doc("Get positive feedback rates.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Returns(http.StatusOK, "OK", map[string][]cache.TimeSeriesPoint{}).
		Writes(map[string][]cache.TimeSeriesPoint{}))
	// Get a user
	ws.Route(ws.GET("/dashboard/user/{user-id}").To(m.getUser).
		Doc("Get a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Returns(http.StatusOK, "OK", User{}).
		Writes(User{}))
	// Get a user feedback
	ws.Route(ws.GET("/dashboard/user/{user-id}/feedback/{feedback-type}").To(m.getTypedFeedbackByUser).
		Doc("Get feedback by user id with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Returns(http.StatusOK, "OK", []Feedback{}).
		Writes([]Feedback{}))
	// Get users
	ws.Route(ws.GET("/dashboard/users").To(m.getUsers).
		Doc("Get users.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned users").DataType("int")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Returns(http.StatusOK, "OK", UserIterator{}).
		Writes(UserIterator{}))
	// Get popular items
	ws.Route(ws.GET("/dashboard/popular/").To(m.getPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(http.StatusOK, "OK", []ScoredItem{}).
		Writes([]ScoredItem{}))
	ws.Route(ws.GET("/dashboard/popular/{category}").To(m.getPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(http.StatusOK, "OK", []ScoredItem{}).
		Writes([]ScoredItem{}))
	// Get latest items
	ws.Route(ws.GET("/dashboard/latest/").To(m.getLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(http.StatusOK, "OK", []ScoredItem{}).
		Writes([]ScoredItem{}))
	ws.Route(ws.GET("/dashboard/latest/{category}").To(m.getLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(http.StatusOK, "OK", []ScoredItem{}).
		Writes([]ScoredItem{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Returns(http.StatusOK, "OK", []data.Item{}).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}/{recommender}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("recommender", "one of `final`, `collaborative`, `user_based` and `item_based`").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Returns(http.StatusOK, "OK", []data.Item{}).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}/{recommender}/{category}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("recommender", "one of `final`, `collaborative`, `user_based` and `item_based`").DataType("string")).
		Param(ws.PathParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Returns(http.StatusOK, "OK", []data.Item{}).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/item/{item-id}/neighbors").To(m.getItemNeighbors).
		Doc("get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(http.StatusOK, "OK", []ScoredItem{}).
		Writes([]ScoredItem{}))
	ws.Route(ws.GET("/dashboard/item/{item-id}/neighbors/{category}").To(m.getItemCategorizedNeighbors).
		Doc("get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.PathParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(http.StatusOK, "OK", []ScoredItem{}).
		Writes([]ScoredItem{}))
	ws.Route(ws.GET("/dashboard/user/{user-id}/neighbors").To(m.getUserNeighbors).
		Doc("get neighbors of a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned users").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(http.StatusOK, "OK", []ScoreUser{}).
		Writes([]ScoreUser{}))
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

func (m *Master) SetOneMode(workerScheduleHandler http.HandlerFunc) {
	m.workerScheduleHandler = workerScheduleHandler
}

func (m *Master) StartHttpServer() {
	m.CreateWebService()
	container := restful.NewContainer()
	container.Handle("/", http.HandlerFunc(m.dashboard))
	container.Handle("/login", http.HandlerFunc(m.login))
	container.Handle("/logout", http.HandlerFunc(m.logout))
	container.Handle("/api/purge", http.HandlerFunc(m.purge))
	container.Handle("/api/bulk/users", http.HandlerFunc(m.importExportUsers))
	container.Handle("/api/bulk/items", http.HandlerFunc(m.importExportItems))
	container.Handle("/api/bulk/feedback", http.HandlerFunc(m.importExportFeedback))
	if m.workerScheduleHandler == nil {
		container.Handle("/api/admin/schedule", http.HandlerFunc(m.scheduleAPIHandler))
	} else {
		container.Handle("/api/admin/schedule/master", http.HandlerFunc(m.scheduleAPIHandler))
		container.Handle("/api/admin/schedule/worker", m.workerScheduleHandler)
	}
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

	// Create temporary directory if not exist
	tempDir := os.TempDir()
	if err = os.MkdirAll(tempDir, 1777); err != nil {
		log.Logger().Fatal("failed to create temporary directory", zap.String("directory", tempDir), zap.Error(err))
	}
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

func (m *Master) checkToken(token string) (bool, error) {
	resp, err := http.Get(fmt.Sprintf("%s/auth/dashboard/%s", m.Config.Master.DashboardAuthServer, token))
	if err != nil {
		return false, errors.Trace(err)
	}
	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return false, nil
	} else {
		if message, err := io.ReadAll(resp.Body); err != nil {
			return false, errors.Trace(err)
		} else {
			return false, errors.New(string(message))
		}
	}
}

func (m *Master) login(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		log.Logger().Info("GET /login", zap.Int("status_code", http.StatusOK))
		staticFileServer.ServeHTTP(response, request)
	case http.MethodPost:
		token := request.FormValue("token")
		name := request.FormValue("user_name")
		pass := request.FormValue("password")
		if m.Config.Master.DashboardAuthServer != "" {
			// check access token
			if isValid, err := m.checkToken(token); err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return
			} else if !isValid {
				http.Redirect(response, request, "login?msg=incorrect", http.StatusFound)
				log.Logger().Info("POST /login", zap.Int("status_code", http.StatusUnauthorized))
				return
			}
			// save token to cache
			if encoded, err := cookieHandler.Encode("token", token); err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return
			} else {
				cookie := &http.Cookie{
					Name:  "token",
					Value: encoded,
					Path:  "/",
				}
				http.SetCookie(response, cookie)
				http.Redirect(response, request, "/", http.StatusFound)
				log.Logger().Info("POST /login", zap.Int("status_code", http.StatusUnauthorized))
				return
			}
		} else if m.Config.Master.DashboardUserName != "" || m.Config.Master.DashboardPassword != "" {
			if name != m.Config.Master.DashboardUserName || pass != m.Config.Master.DashboardPassword {
				http.Redirect(response, request, "login?msg=incorrect", http.StatusFound)
				log.Logger().Info("POST /login", zap.Int("status_code", http.StatusUnauthorized))
				return
			}
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
			http.Redirect(response, request, "/", http.StatusFound)
			log.Logger().Info("POST /login", zap.Int("status_code", http.StatusFound))
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
		chain.ProcessFilter(req, resp)
	} else if !strings.HasPrefix(req.SelectedRoutePath(), "/api/dashboard") {
		chain.ProcessFilter(req, resp)
	} else {
		if err := resp.WriteError(http.StatusUnauthorized, fmt.Errorf("unauthorized")); err != nil {
			log.ResponseLogger(resp).Error("failed to write error", zap.Error(err))
		}
	}
}

func (m *Master) checkLogin(request *http.Request) bool {
	if m.Config.Master.AdminAPIKey != "" && m.Config.Master.AdminAPIKey == request.Header.Get("X-Api-Key") {
		return true
	}
	if m.Config.Master.DashboardAuthServer != "" {
		if tokenCookie, err := request.Cookie("token"); err == nil {
			var token string
			if err = cookieHandler.Decode("token", tokenCookie.Value, &token); err == nil {
				if isValid, err := m.checkToken(token); err != nil {
					log.Logger().Error("failed to check access token", zap.Error(err))
				} else if isValid {
					return true
				}
			}
		}
		return false
	} else if m.Config.Master.DashboardUserName != "" || m.Config.Master.DashboardPassword != "" {
		if sessionCookie, err := request.Cookie("session"); err == nil {
			cookieValue := make(map[string]string)
			if err = cookieHandler.Decode("session", sessionCookie.Value, &cookieValue); err == nil {
				userName := cookieValue["user_name"]
				password := cookieValue["password"]
				if userName == m.Config.Master.DashboardUserName && password == m.Config.Master.DashboardPassword {
					return true
				}
			}
		}
		return false
	}
	return true
}

func (m *Master) getCategories(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	categories, err := m.CacheClient.GetSet(ctx, cache.ItemCategories)
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
	if m.Config.Master.DashboardRedacted {
		delete(configMap, "database")
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

func (m *Master) getStats(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	status := Status{BinaryVersion: version.Version}
	var err error
	// read number of users
	if status.NumUsers, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.NumUsers)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of users", zap.Error(err))
	}
	// read number of items
	if status.NumItems, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.NumItems)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of items", zap.Error(err))
	}
	// read number of user labels
	if status.NumUserLabels, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.NumUserLabels)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of user labels", zap.Error(err))
	}
	// read number of item labels
	if status.NumItemLabels, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.NumItemLabels)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of item labels", zap.Error(err))
	}
	// read number of total positive feedback
	if status.NumTotalPosFeedback, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.NumTotalPosFeedbacks)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of total positive feedbacks", zap.Error(err))
	}
	// read number of valid positive feedback
	if status.NumValidPosFeedback, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.NumValidPosFeedbacks)).Integer(); err != nil {
		log.ResponseLogger(response).Warn("failed to get number of valid positive feedbacks", zap.Error(err))
	}
	// read number of valid negative feedback
	if status.NumValidNegFeedback, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.NumValidNegFeedbacks)).Integer(); err != nil {
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
	if status.PopularItemsUpdateTime, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.LastUpdatePopularItemsTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get popular items update time", zap.Error(err))
	}
	// read the latest items update time
	if status.LatestItemsUpdateTime, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.LastUpdateLatestItemsTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get latest items update time", zap.Error(err))
	}
	status.MatchingModelScore = m.rankingScore
	status.RankingModelScore = m.clickScore
	// read last fit matching model time
	if status.MatchingModelFitTime, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.LastFitMatchingModelTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get last fit matching model time", zap.Error(err))
	}
	// read last fit ranking model time
	if status.RankingModelFitTime, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.LastFitRankingModelTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get last fit ranking model time", zap.Error(err))
	}
	// read user neighbor index recall
	var temp string
	if m.Config.Recommend.UserNeighbors.EnableIndex {
		if temp, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.UserNeighborIndexRecall)).String(); err != nil {
			log.ResponseLogger(response).Warn("failed to get user neighbor index recall", zap.Error(err))
		} else {
			status.UserNeighborIndexRecall = encoding.ParseFloat32(temp)
		}
	}
	// read item neighbor index recall
	if m.Config.Recommend.ItemNeighbors.EnableIndex {
		if temp, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.ItemNeighborIndexRecall)).String(); err != nil {
			log.ResponseLogger(response).Warn("failed to get item neighbor index recall", zap.Error(err))
		} else {
			status.ItemNeighborIndexRecall = encoding.ParseFloat32(temp)
		}
	}
	// read matching index recall
	if m.Config.Recommend.Collaborative.EnableIndex {
		if temp, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.MatchingIndexRecall)).String(); err != nil {
			log.ResponseLogger(response).Warn("failed to get matching index recall", zap.Error(err))
		} else {
			status.MatchingIndexRecall = encoding.ParseFloat32(temp)
		}
	}
	server.Ok(response, status)
}

func (m *Master) getTasks(_ *restful.Request, response *restful.Response) {
	// List workers
	workers := mapset.NewSet[string]()
	m.nodesInfoMutex.RLock()
	for _, info := range m.nodesInfo {
		if info.Type == WorkerNode {
			workers.Add(info.Name)
		}
	}
	m.nodesInfoMutex.RUnlock()
	// List local progress
	progressList := m.tracer.List()
	// list remote progress
	m.remoteProgress.Range(func(key, value interface{}) bool {
		if workers.Contains(key.(string)) {
			progressList = append(progressList, value.([]progress.Progress)...)
		}
		return true
	})
	server.Ok(response, progressList)
}

func (m *Master) getRates(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// Parse parameters
	n, err := server.ParseInt(request, "n", 100)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	measurements := make(map[string][]cache.TimeSeriesPoint, len(m.Config.Recommend.DataSource.PositiveFeedbackTypes))
	for _, feedbackType := range m.Config.Recommend.DataSource.PositiveFeedbackTypes {
		measurements[feedbackType], err = m.CacheClient.GetTimeSeriesPoints(ctx, cache.Key(PositiveFeedbackRate, feedbackType),
			time.Now().Add(-24*time.Hour*time.Duration(n)), time.Now())
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
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// get user id
	userId := request.PathParameter("user-id")
	// get user
	user, err := m.DataClient.GetUser(ctx, userId)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			server.PageNotFound(response, err)
		} else {
			server.InternalServerError(response, err)
		}
		return
	}
	detail := User{User: user}
	if detail.LastActiveTime, err = m.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, user.UserId)).Time(); err != nil && !errors.Is(err, errors.NotFound) {
		server.InternalServerError(response, err)
		return
	}
	if detail.LastUpdateTime, err = m.CacheClient.Get(ctx, cache.Key(cache.LastUpdateUserRecommendTime, user.UserId)).Time(); err != nil && !errors.Is(err, errors.NotFound) {
		server.InternalServerError(response, err)
		return
	}
	server.Ok(response, detail)
}

func (m *Master) getUsers(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// Authorize
	cursor := request.QueryParameter("cursor")
	n, err := server.ParseInt(request, "n", m.Config.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	// get all users
	cursor, users, err := m.DataClient.GetUsers(ctx, cursor, n)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	details := make([]User, len(users))
	for i, user := range users {
		details[i].User = user
		if details[i].LastActiveTime, err = m.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, user.UserId)).Time(); err != nil && !errors.Is(err, errors.NotFound) {
			server.InternalServerError(response, err)
			return
		}
		if details[i].LastUpdateTime, err = m.CacheClient.Get(ctx, cache.Key(cache.LastUpdateUserRecommendTime, user.UserId)).Time(); err != nil && !errors.Is(err, errors.NotFound) {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, UserIterator{Cursor: cursor, Users: details})
}

func (m *Master) getRecommend(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// parse arguments
	recommender := request.PathParameter("recommender")
	userId := request.PathParameter("user-id")
	categories := []string{request.PathParameter("category")}
	n, err := server.ParseInt(request, "n", m.Config.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	var results []string
	switch recommender {
	case "offline":
		results, err = m.Recommend(ctx, response, userId, categories, n, m.RecommendOffline)
	case "collaborative":
		results, err = m.Recommend(ctx, response, userId, categories, n, m.RecommendCollaborative)
	case "user_based":
		results, err = m.Recommend(ctx, response, userId, categories, n, m.RecommendUserBased)
	case "item_based":
		results, err = m.Recommend(ctx, response, userId, categories, n, m.RecommendItemBased)
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
		results, err = m.Recommend(ctx, response, userId, categories, n, recommenders...)
	}
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	// Send result
	details := make([]data.Item, len(results))
	for i := range results {
		details[i], err = m.DataClient.GetItem(ctx, results[i])
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
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	feedback, err := m.DataClient.GetUserFeedback(ctx, userId, m.Config.Now(), feedbackType)
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
		details[i].Item, err = m.DataClient.GetItem(ctx, feedback[i].ItemId)
		if errors.Is(err, errors.NotFound) {
			details[i].Item = data.Item{ItemId: feedback[i].ItemId, Comment: "** This item doesn't exist in Gorse **"}
		} else if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, details)
}

type ScoredItem struct {
	data.Item
	Score float64
}

type ScoreUser struct {
	data.User
	Score float64
}

func (m *Master) searchDocuments(collection, subset, category string, request *restful.Request, response *restful.Response, retType interface{}) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
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
	scores, err := m.CacheClient.SearchScores(ctx, collection, subset, []string{category}, offset, m.Config.Recommend.CacheSize)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	if n > 0 && len(scores) > n {
		scores = scores[:n]
	}
	// Send result
	switch retType.(type) {
	case data.Item:
		details := make([]ScoredItem, len(scores))
		for i := range scores {
			details[i].Score = scores[i].Score
			details[i].Item, err = m.DataClient.GetItem(ctx, scores[i].Id)
			if err != nil {
				server.InternalServerError(response, err)
				return
			}
		}
		server.Ok(response, details)
	case data.User:
		details := make([]ScoreUser, len(scores))
		for i := range scores {
			details[i].Score = scores[i].Score
			details[i].User, err = m.DataClient.GetUser(ctx, scores[i].Id)
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
	m.searchDocuments(cache.PopularItems, "", category, request, response, data.Item{})
}

func (m *Master) getLatest(request *restful.Request, response *restful.Response) {
	category := request.PathParameter("category")
	m.searchDocuments(cache.LatestItems, "", category, request, response, data.Item{})
}

func (m *Master) getItemNeighbors(request *restful.Request, response *restful.Response) {
	itemId := request.PathParameter("item-id")
	m.searchDocuments(cache.ItemNeighbors, itemId, "", request, response, data.Item{})
}

func (m *Master) getItemCategorizedNeighbors(request *restful.Request, response *restful.Response) {
	itemId := request.PathParameter("item-id")
	category := request.PathParameter("category")
	m.searchDocuments(cache.ItemNeighbors, itemId, category, request, response, data.Item{})
}

func (m *Master) getUserNeighbors(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")
	m.searchDocuments(cache.UserNeighbors, userId, "", request, response, data.User{})
}

func (m *Master) importExportUsers(response http.ResponseWriter, request *http.Request) {
	ctx := context.Background()
	if request != nil {
		ctx = request.Context()
	}
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
		userChan, errChan := m.DataClient.GetUserStream(ctx, batchSize)
		for users := range userChan {
			for _, user := range users {
				labels, err := json.Marshal(user.Labels)
				if err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
				if _, err = response.Write([]byte(fmt.Sprintf("%s,%s\r\n",
					base.Escape(user.UserId), base.Escape(string(labels))))); err != nil {
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
		m.importUsers(ctx, response, file, hasHeader, sep, labelSep, fmtString)
	}
}

func (m *Master) importUsers(ctx context.Context, response http.ResponseWriter, file io.Reader, hasHeader bool, sep, labelSep, fmtString string) {

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
			var labels any
			if err = json.Unmarshal([]byte(splits[1]), &labels); err != nil {
				server.BadRequest(restful.NewResponse(response),
					fmt.Errorf("invalid labels `%v` at line %d (%s)", splits[1], lineNumber, err.Error()))
				return false
			}
			user.Labels = labels
		}
		users = append(users, user)
		// batch insert
		if len(users) == batchSize {
			err = m.DataClient.BatchInsertUsers(ctx, users)
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
		err = m.DataClient.BatchInsertUsers(ctx, users)
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
	ctx := context.Background()
	if request != nil {
		ctx = request.Context()
	}
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
		itemChan, errChan := m.DataClient.GetItemStream(ctx, batchSize, nil)
		for items := range itemChan {
			for _, item := range items {
				labels, err := json.Marshal(item.Labels)
				if err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
				if _, err = response.Write([]byte(fmt.Sprintf("%s,%t,%s,%v,%s,%s\r\n",
					base.Escape(item.ItemId), item.IsHidden, base.Escape(strings.Join(item.Categories, "|")),
					item.Timestamp, base.Escape(string(labels)), base.Escape(item.Comment)))); err != nil {
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
		m.importItems(ctx, response, file, hasHeader, sep, labelSep, fmtString)
	default:
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (m *Master) importItems(ctx context.Context, response http.ResponseWriter, file io.Reader, hasHeader bool, sep, labelSep, fmtString string) {
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
			var labels any
			if err = json.Unmarshal([]byte(splits[4]), &labels); err != nil {
				server.BadRequest(restful.NewResponse(response),
					fmt.Errorf("failed to parse labels `%v` at line %v", splits[4], lineNumber))
				return false
			}
			item.Labels = labels
		}
		// 6. comment
		item.Comment = splits[5]
		items = append(items, item)
		// batch insert
		if len(items) == batchSize {
			err = m.DataClient.BatchInsertItems(ctx, items)
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
		err = m.DataClient.BatchInsertItems(ctx, items)
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
	ctx := context.Background()
	if request != nil {
		ctx = request.Context()
	}
	if !m.checkLogin(request) {
		writeError(response, http.StatusUnauthorized, "unauthorized")
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
		feedbackChan, errChan := m.DataClient.GetFeedbackStream(ctx, batchSize, data.WithEndTime(*m.Config.Now()))
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
		m.importFeedback(ctx, response, file, hasHeader, sep, fmtString)
	default:
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (m *Master) importFeedback(ctx context.Context, response http.ResponseWriter, file io.Reader, hasHeader bool, sep, fmtString string) {
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
			// batch insert to data store
			err = m.DataClient.BatchInsertFeedback(ctx, feedbacks,
				m.Config.Server.AutoInsertUser,
				m.Config.Server.AutoInsertItem, true)
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
	// insert to cache store
	if len(feedbacks) > 0 {
		// insert to data store
		err = m.DataClient.BatchInsertFeedback(ctx, feedbacks,
			m.Config.Server.AutoInsertUser,
			m.Config.Server.AutoInsertItem, true)
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

var checkList = mapset.NewSet("delete_users", "delete_items", "delete_feedback", "delete_cache")

func (m *Master) purge(response http.ResponseWriter, request *http.Request) {
	// check method
	if request.Method != http.MethodPost {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// check login
	if !m.checkLogin(request) {
		resp := restful.NewResponse(response)
		err := resp.WriteErrorString(http.StatusUnauthorized, "unauthorized")
		if err != nil {
			server.InternalServerError(resp, err)
			return
		}
		return
	}
	// check password
	if m.Config.Master.DashboardPassword == "" {
		writeError(response, http.StatusUnauthorized, "purge is not allowed without dashboard password")
		return
	}
	// check list
	if err := request.ParseForm(); err != nil {
		server.BadRequest(restful.NewResponse(response), err)
		return
	}
	checkedList := strings.Split(request.Form.Get("check_list"), ",")
	if !checkList.Equal(mapset.NewSet(checkedList...)) {
		writeError(response, http.StatusUnauthorized, "please confirm by checking all")
		return
	}
	// purge data
	if err := m.DataClient.Purge(); err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	if err := m.CacheClient.Purge(); err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
}

func (m *Master) scheduleAPIHandler(writer http.ResponseWriter, request *http.Request) {
	if !m.checkAdmin(request) {
		writeError(writer, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch request.Method {
	case http.MethodGet:
		writer.WriteHeader(http.StatusOK)
		bytes, err := json.Marshal(m.scheduleState)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
		}
		if _, err = writer.Write(bytes); err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
		}
	case http.MethodPost:
		s := request.FormValue("search_model")
		if s != "" {
			if searchModel, err := strconv.ParseBool(s); err != nil {
				writeError(writer, http.StatusBadRequest, err.Error())
			} else {
				m.scheduleState.SearchModel = searchModel
			}
		}
		m.triggerChan.Signal()
	default:
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func writeError(response http.ResponseWriter, httpStatus int, message string) {
	log.Logger().Error(strings.ToLower(http.StatusText(httpStatus)), zap.String("error", message))
	response.Header().Set("Access-Control-Allow-Origin", "*")
	response.WriteHeader(httpStatus)
	if _, err := response.Write([]byte(message)); err != nil {
		log.Logger().Error("failed to write error", zap.Error(err))
	}
}

func (s *Master) checkAdmin(request *http.Request) bool {
	if s.Config.Master.AdminAPIKey == "" {
		return true
	}
	if request.FormValue("X-API-Key") == s.Config.Master.AdminAPIKey {
		return true
	}
	return false
}
