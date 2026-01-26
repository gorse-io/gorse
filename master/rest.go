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
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	mapset "github.com/deckarep/golang-set/v2"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/go-viper/mapstructure/v2"
	"github.com/gorilla/securecookie"
	_ "github.com/gorse-io/dashboard"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/model/cf"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/server"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/meta"
	"github.com/invopop/jsonschema"
	"github.com/juju/errors"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/rakyll/statik/fs"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserInfo struct {
	Name       string `json:"name"`
	FamilyName string `json:"family_name"`
	GivenName  string `json:"given_name"`
	MiddleName string `json:"middle_name"`
	NickName   string `json:"nickname"`
	Picture    string `json:"picture"`
	UpdatedAt  string `json:"updated_at"`
	Email      string `json:"email"`
	Verified   bool   `json:"email_verified"`
	AuthType   string `json:"auth_type"`
}

func (m *Master) CreateWebService() {
	ws := m.WebService
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	ws.Path("/api/")
	ws.Filter(m.LoginFilter)

	ws.Route(ws.GET("/dashboard/userinfo").To(m.handleUserInfo).
		Doc("Get login user information.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Returns(http.StatusOK, "OK", UserInfo{}).
		Writes(UserInfo{}))
	ws.Route(ws.GET("/dashboard/cluster").To(m.getCluster).
		Doc("Get nodes in the cluster.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Returns(http.StatusOK, "OK", []meta.Node{}).
		Writes([]meta.Node{}))
	ws.Route(ws.GET("/dashboard/categories").To(m.getCategories).
		Doc("Get categories of items.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Returns(http.StatusOK, "OK", []string{}).
		Writes([]string{}))
	ws.Route(ws.POST("/dashboard/config").To(m.postConfig).
		Doc("Update config.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Returns(http.StatusOK, "OK", config.Config{}).
		Writes(config.Config{}))
	ws.Route(ws.GET("/dashboard/config").To(m.getConfig).
		Doc("Get config.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Returns(http.StatusOK, "OK", config.Config{}).
		Writes(config.Config{}))
	ws.Route(ws.GET("/dashboard/config/schema").To(m.getConfigSchema).
		Doc("Get config schema.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Returns(http.StatusOK, "OK", config.Config{}).
		Writes(jsonschema.Schema{}))
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
		Returns(http.StatusOK, "OK", []monitor.Progress{}).
		Writes([]monitor.Progress{}))
	ws.Route(ws.GET("/dashboard/timeseries/{name}").To(m.getTimeseries).
		Doc("Get time series data.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("name", "name of the time series").DataType("string")).
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
	ws.Route(ws.GET("/dashboard/user/{user-id}/feedback/").To(m.getUserFeedback).
		Doc("Get feedback by user id.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned feedback").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of returned feedback").DataType("int")).
		Returns(http.StatusOK, "OK", []DetailedFeedback{}).
		Writes([]DetailedFeedback{}))
	ws.Route(ws.GET("/dashboard/user/{user-id}/feedback/{feedback-type}").To(m.getUserFeedback).
		Doc("Get feedback by user id with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned feedback").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of returned feedback").DataType("int")).
		Returns(http.StatusOK, "OK", []DetailedFeedback{}).
		Writes([]DetailedFeedback{}))
	// Get users
	ws.Route(ws.GET("/dashboard/users").To(m.getUsers).
		Doc("Get users.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned users").DataType("int")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Returns(http.StatusOK, "OK", UserIterator{}).
		Writes(UserIterator{}))
	// Get non-personalized recommendation
	ws.Route(ws.GET("/dashboard/latest").To(m.getLatest).
		Doc("Get latest items.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("category", "Category of returned items.").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Returns(http.StatusOK, "OK", []data.Item{}).
		Writes([]ScoredItem{}))
	ws.Route(ws.GET("/dashboard/non-personalized/{name}").To(m.getNonPersonalized).
		Doc("Get non-personalized recommendations.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("category", "Category of returned items.").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Param(ws.QueryParameter("user-id", "Remove read items of a user").DataType("string")).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Returns(http.StatusOK, "OK", []data.Item{}).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}/{recommender}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("recommender", "one of `final`, `collaborative`, `user_based` and `item_based`").DataType("string")).
		Param(ws.QueryParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Returns(http.StatusOK, "OK", []data.Item{}).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}/{recommender}/{name}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("recommender", "one of `final`, `collaborative`, `user_based` and `item_based`").DataType("string")).
		Param(ws.PathParameter("name", "name of the recommender").DataType("string")).
		Param(ws.QueryParameter("category", "category of items").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Returns(http.StatusOK, "OK", []data.Item{}).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/item-to-item/{name}/{item-id}").To(m.getItemToItem).
		Doc("get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(http.StatusOK, "OK", []ScoredItem{}).
		Writes([]ScoredItem{}))
	ws.Route(ws.GET("/dashboard/user-to-user/{name}/{user-id}").To(m.getUserToUser).
		Doc("get neighbors of a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned users").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(http.StatusOK, "OK", []ScoreUser{}).
		Writes([]ScoreUser{}))
	ws.Route(ws.GET("/dashboard/external").To(m.getExternal).
		Doc("get external recommendations preview").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("script", "external script").DataType("string")).
		Param(ws.QueryParameter("user-id", "identifier of the user").DataType("string")).
		Returns(http.StatusOK, "OK", []string{}).
		Writes([]string{}))
	ws.Route(ws.GET("/dashboard/ranker/prompt").To(m.getRankerPrompt).
		Doc("Get ranker prompt.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("prompt", "prompt template").DataType("string")).
		Param(ws.QueryParameter("user-id", "identifier of the user").DataType("string")).
		Returns(http.StatusOK, "OK", "").
		Writes(""))
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
	container.Handle("/callback/oauth2", http.HandlerFunc(m.handleOAuth2Callback))
	container.Handle("/api/purge", http.HandlerFunc(m.purge))
	container.Handle("/api/bulk/users", http.HandlerFunc(m.importExportUsers))
	container.Handle("/api/bulk/items", http.HandlerFunc(m.importExportItems))
	container.Handle("/api/bulk/feedback", http.HandlerFunc(m.importExportFeedback))
	container.Handle("/api/dump", http.HandlerFunc(m.dump))
	container.Handle("/api/restore", http.HandlerFunc(m.restore))
	container.Handle("/api/chat", http.HandlerFunc(m.chat))
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
			if m.Config.OIDC.Enable {
				// Redirect to OIDC login
				http.Redirect(response, request, m.oauth2Config.AuthCodeURL(""), http.StatusFound)
			} else {
				http.Redirect(response, request, "/login", http.StatusFound)
				log.Logger().Info(fmt.Sprintf("%s %s", request.Method, request.URL), zap.Int("status_code", http.StatusFound))
			}
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
		if m.Config.Master.DashboardUserName != "" || m.Config.Master.DashboardPassword != "" {
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
	if m.Config.OIDC.Enable {
		if tokenCookie, err := request.Cookie("id_token"); err == nil {
			var token string
			if err = cookieHandler.Decode("id_token", tokenCookie.Value, &token); err == nil {
				if m.tokenCache.Get(token) != nil {
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

func (m *Master) handleUserInfo(request *restful.Request, response *restful.Response) {
	if m.Config.OIDC.Enable {
		if tokenCookie, err := request.Request.Cookie("id_token"); err == nil {
			var token string
			if err = cookieHandler.Decode("id_token", tokenCookie.Value, &token); err == nil {
				if item := m.tokenCache.Get(token); item != nil {
					userInfo := item.Value()
					userInfo.AuthType = "OIDC"
					server.Ok(response, userInfo)
					return
				}
			}
		}
	} else if m.Config.Master.DashboardUserName != "" {
		server.Ok(response, UserInfo{
			Name: m.Config.Master.DashboardUserName,
		})
	} else {
		response.Header().Set("Content-Type", "application/json")
		if _, err := response.Write([]byte("null")); err != nil {
			log.ResponseLogger(response).Error("failed to write response", zap.Error(err))
		}
	}
}

func (m *Master) getCategories(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	categoryScores, err := m.CacheClient.SearchScores(ctx, cache.ItemCategories, "", nil, 0, -1)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	categories := make([]string, len(categoryScores))
	for i, score := range categoryScores {
		categories[i] = score.Id
	}
	server.Ok(response, categories)
}

func (m *Master) getCluster(_ *restful.Request, response *restful.Response) {
	nodes, err := m.metaStore.ListNodes()
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Type < nodes[j].Type
	})
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

func (m *Master) postConfig(request *restful.Request, response *restful.Response) {
	var newConfigMap map[string]any
	if err := request.ReadEntity(&newConfigMap); err != nil {
		server.BadRequest(response, err)
		return
	}
	var newConfig config.Config
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			config.StringToFeedbackTypeHookFunc(),
		),
		Result: &newConfig,
	})
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	if err = decoder.Decode(newConfigMap); err != nil {
		server.BadRequest(response, err)
		return
	}
	configForValidation := *m.Config
	configForValidation.Recommend = newConfig.Recommend
	if err = configForValidation.Validate(); err != nil {
		server.BadRequest(response, err)
		return
	}
	recommendConfigBytes, err := json.Marshal(newConfig.Recommend)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	if err = m.metaStore.Put(meta.RECOMMEND_CONFIG, string(recommendConfigBytes)); err != nil {
		server.InternalServerError(response, err)
		return
	}
	m.Config.Recommend = newConfig.Recommend

	m.cancel()
	select {
	case m.scheduled <- struct{}{}:
	default:
	}
	server.Ok(response, newConfig)
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

func (m *Master) getConfigSchema(_ *restful.Request, response *restful.Response) {
	server.Ok(response, jsonschema.Reflect(m.Config))
}

type Status struct {
	BinaryVersion          string
	NumServers             int
	NumWorkers             int
	NumUsers               int
	NumItems               int
	NumUserLabels          int
	NumItemLabels          int
	NumTotalPosFeedback    int
	NumValidPosFeedback    int
	NumValidNegFeedback    int
	PopularItemsUpdateTime time.Time
	LatestItemsUpdateTime  time.Time
	MatchingModelFitTime   time.Time
	MatchingModelScore     cf.Score
	RankingModelFitTime    time.Time
	RankingModelScore      ctr.Score
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
	nodes, err := m.metaStore.ListNodes()
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	for _, node := range nodes {
		switch node.Type {
		case protocol.NodeType_Server.String():
			status.NumServers++
		case protocol.NodeType_Worker.String():
			status.NumWorkers++
		}
	}
	// read popular items update time
	if status.PopularItemsUpdateTime, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.LastUpdatePopularItemsTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get popular items update time", zap.Error(err))
	}
	// read the latest items update time
	if status.LatestItemsUpdateTime, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.LastUpdateLatestItemsTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get latest items update time", zap.Error(err))
	}
	// read last fit matching model time
	if status.MatchingModelFitTime, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.LastFitMatchingModelTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get last fit matching model time", zap.Error(err))
	}
	// read last fit ranking model time
	if status.RankingModelFitTime, err = m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.LastFitRankingModelTime)).Time(); err != nil {
		log.ResponseLogger(response).Warn("failed to get last fit ranking model time", zap.Error(err))
	}
	server.Ok(response, status)
}

func (m *Master) getTasks(_ *restful.Request, response *restful.Response) {
	// List workers
	workers := mapset.NewSet[string]()
	nodes, err := m.metaStore.ListNodes()
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	for _, node := range nodes {
		if node.Type == protocol.NodeType_Worker.String() {
			workers.Add(node.UUID)
		}
	}
	// List local progress
	progressList := m.tracer.List()
	// list remote progress
	m.remoteProgress.Range(func(key, value interface{}) bool {
		if workers.Contains(key.(string)) {
			progressList = append(progressList, value.([]monitor.Progress)...)
		}
		return true
	})
	server.Ok(response, progressList)
}

func (m *Master) getTimeseries(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// get time series name
	name := request.PathParameter("name")
	// get begin time
	beginStr := request.QueryParameter("begin")
	if beginStr == "" {
		beginStr = time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	}
	begin, err := dateparse.ParseAny(beginStr)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	// get end time
	endStr := request.QueryParameter("end")
	if endStr == "" {
		endStr = time.Now().Format(time.RFC3339)
	}
	end, err := dateparse.ParseAny(endStr)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	// get duration
	durationStr := request.QueryParameter("duration")
	if durationStr == "" {
		durationStr = "24h"
	}
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	// get time series data
	data, err := m.CacheClient.GetTimeSeriesPoints(ctx, cache.Key(name), begin, end, duration)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	server.Ok(response, data)
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
	if detail.LastUpdateTime, err = m.CacheClient.Get(ctx, cache.Key(cache.RecommendUpdateTime, user.UserId)).Time(); err != nil && !errors.Is(err, errors.NotFound) {
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
		if details[i].LastUpdateTime, err = m.CacheClient.Get(ctx, cache.Key(cache.RecommendUpdateTime, user.UserId)).Time(); err != nil && !errors.Is(err, errors.NotFound) {
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
	recommenderType := request.PathParameter("recommender")
	recommenderName := request.PathParameter("name")
	userId := request.PathParameter("user-id")
	categories := server.ReadCategories(request, nil)
	n, err := server.ParseInt(request, "n", m.Config.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}

	recommender, err := logics.NewRecommender(m.Config.Recommend, m.CacheClient, m.DataClient, true, userId, categories)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	var scores []cache.Score
	if recommenderType != "" {
		var name string
		if recommenderName != "" {
			name = recommenderType + "/" + recommenderName
		} else {
			name = recommenderType
		}
		scores, _, err = recommender.RecommendSequential(ctx, scores, n, name)
	} else {
		scores, err = recommender.Recommend(ctx, n)
	}
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	results := lo.Map(scores, func(item cache.Score, index int) string {
		return item.Id
	})

	// Get item details
	items, err := m.DataClient.BatchGetItems(ctx, lo.Map(results, func(id string, _ int) string {
		return id
	}))
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	itemsMap := make(map[string]data.Item, len(items))
	for _, item := range items {
		itemsMap[item.ItemId] = item
	}

	// Send result
	details := make([]ScoredItem, 0, len(results))
	for i := range results {
		detail, exist := itemsMap[results[i]]
		if exist {
			details = append(details, ScoredItem{Item: detail, Score: scores[i].Score})
		} else {
			log.Logger().Warn("recommended item doesn't exist", zap.String("item_id", results[i]))
		}
	}
	server.Ok(response, details)
}

type DetailedFeedback struct {
	FeedbackType string
	UserId       string
	Item         data.Item
	Value        float64
	Timestamp    time.Time
	Comment      string
}

// get feedback by user-id with feedback type
func (m *Master) getUserFeedback(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}

	// parse feedback type
	feedbackType := request.PathParameter("feedback-type")
	var feedbackTypeExpressions []expression.FeedbackTypeExpression
	if feedbackType != "" {
		feedbackTypeExpressions = make([]expression.FeedbackTypeExpression, 1)
		if err := feedbackTypeExpressions[0].FromString(feedbackType); err != nil {
			server.BadRequest(response, fmt.Errorf("invalid feedback type `%s`: %w", feedbackType, err))
			return
		}
	} else {
		feedbackTypeExpressions = m.Config.Recommend.DataSource.PositiveFeedbackTypes
	}

	// parse n
	nStr := request.QueryParameter("n")
	n := m.Config.Server.DefaultN
	if nStr != "" {
		var err error
		n, err = server.ParseInt(request, "n", m.Config.Server.DefaultN)
		if err != nil {
			server.BadRequest(response, err)
			return
		}
	}

	// parse offset
	offsetStr := request.QueryParameter("offset")
	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = server.ParseInt(request, "offset", 0)
		if err != nil {
			server.BadRequest(response, err)
			return
		}
	}

	userId := request.PathParameter("user-id")
	feedback, err := m.DataClient.GetUserFeedback(ctx, userId, m.Config.Now(), feedbackTypeExpressions...)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	sort.Slice(feedback, func(i, j int) bool {
		return feedback[i].Timestamp.After(feedback[j].Timestamp)
	})
	if offset <= len(feedback) {
		feedback = feedback[offset:]
	}
	if n < len(feedback) {
		feedback = feedback[:n]
	}

	// Get item details
	items, err := m.DataClient.BatchGetItems(ctx, lo.Map(feedback, func(f data.Feedback, _ int) string {
		return f.ItemId
	}))
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	itemsMap := make(map[string]data.Item, len(items))
	for _, item := range items {
		itemsMap[item.ItemId] = item
	}

	details := make([]DetailedFeedback, len(feedback))
	for i := range feedback {
		details[i].FeedbackType = feedback[i].FeedbackType
		details[i].UserId = feedback[i].UserId
		details[i].Value = feedback[i].Value
		details[i].Timestamp = feedback[i].Timestamp
		details[i].Comment = feedback[i].Comment
		var exist bool
		details[i].Item, exist = itemsMap[feedback[i].ItemId]
		if !exist {
			details[i].Item = data.Item{ItemId: feedback[i].ItemId, Comment: "** This item doesn't exist in Gorse **"}
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

func (m *Master) GetItem(score cache.Score) (any, error) {
	var item ScoredItem
	var err error
	item.Score = score.Score
	item.Item, err = m.DataClient.GetItem(context.Background(), score.Id)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (m *Master) GetUser(score cache.Score) (any, error) {
	var user ScoreUser
	var err error
	user.Score = score.Score
	user.User, err = m.DataClient.GetUser(context.Background(), score.Id)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (m *Master) getLatest(request *restful.Request, response *restful.Response) {
	var (
		offset int
		n      int
		err    error
	)
	categories := server.ReadCategories(request, nil)
	if offset, err = server.ParseInt(request, "offset", 0); err != nil {
		server.BadRequest(response, err)
		return
	}
	if n, err = server.ParseInt(request, "n", m.Config.Server.DefaultN); err != nil {
		server.BadRequest(response, err)
		return
	}
	items, err := m.DataClient.GetLatestItems(context.Background(), offset+n, categories)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	if offset < len(items) {
		items = items[offset:]
	}
	if n < len(items) {
		items = items[:n]
	}
	scores := make([]ScoredItem, len(items))
	for i := range items {
		scores[i] = ScoredItem{Item: items[i], Score: float64(items[i].Timestamp.Unix())}
	}
	m.SetLastModified(request, response, time.Now().String())
	server.Ok(response, scores)
}

func (m *Master) getNonPersonalized(request *restful.Request, response *restful.Response) {
	name := request.PathParameter("name")
	categories := server.ReadCategories(request, []string{""})
	m.SetLastModified(request, response, cache.Key(cache.NonPersonalizedUpdateTime, name))
	m.SearchDocuments(cache.NonPersonalized, name, categories, m.GetItem, request, response)
}

func (m *Master) getItemToItem(request *restful.Request, response *restful.Response) {
	name := request.PathParameter("name")
	itemId := request.PathParameter("item-id")
	categories := request.QueryParameters("category")
	m.SetLastModified(request, response, cache.Key(cache.ItemToItemUpdateTime, name, itemId))
	m.SearchDocuments(cache.ItemToItem, cache.Key(name, itemId), categories, m.GetItem, request, response)
}

func (m *Master) getUserToUser(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")
	name := request.PathParameter("name")
	m.SetLastModified(request, response, cache.Key(cache.UserToUserUpdateTime, name, userId))
	m.SearchDocuments(cache.UserToUser, cache.Key(name, userId), nil, m.GetUser, request, response)
}

func (m *Master) getExternal(request *restful.Request, response *restful.Response) {
	scriptBase64 := request.QueryParameter("script")
	if scriptBase64 == "" {
		server.BadRequest(response, fmt.Errorf("script is required"))
		return
	}
	userId := request.QueryParameter("user-id")

	scriptBytes, err := base64.StdEncoding.DecodeString(scriptBase64)
	if err != nil {
		server.BadRequest(response, fmt.Errorf("invalid script encoding: %w", err))
		return
	}

	external, err := logics.NewExternal(config.ExternalConfig{
		Name:   "preview",
		Script: string(scriptBytes),
	})
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	defer external.Close()

	items, err := external.Pull(userId)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}

	m.SetLastModified(request, response, time.Now().String())
	server.Ok(response, items)
}

func (m *Master) getRankerPrompt(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	promptBase64 := request.QueryParameter("prompt")
	if promptBase64 == "" {
		server.BadRequest(response, fmt.Errorf("prompt is required"))
		return
	}
	userId := request.QueryParameter("user-id")
	if userId == "" {
		server.BadRequest(response, fmt.Errorf("user-id is required"))
		return
	}
	promptBytes, err := base64.StdEncoding.DecodeString(promptBase64)
	if err != nil {
		server.BadRequest(response, fmt.Errorf("invalid prompt encoding: %w", err))
		return
	}
	template, err := gonja.FromString(string(promptBytes))
	if err != nil {
		server.BadRequest(response, err)
		return
	}

	user, err := m.DataClient.GetUser(ctx, userId)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			server.PageNotFound(response, err)
		} else {
			server.InternalServerError(response, err)
		}
		return
	}
	feedbacks, err := m.DataClient.GetUserFeedback(ctx, userId, m.Config.Now(), m.Config.Recommend.DataSource.PositiveFeedbackTypes...)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	data.SortFeedbacks(feedbacks)
	if len(feedbacks) > 10 {
		feedbacks = feedbacks[:10]
	}
	itemsById := map[string]data.Item{}
	if len(feedbacks) > 0 {
		feedbackItemIds := lo.Map(feedbacks, func(fb data.Feedback, _ int) string {
			return fb.ItemId
		})
		feedbackItems, err := m.DataClient.BatchGetItems(ctx, feedbackItemIds)
		if err != nil {
			server.InternalServerError(response, err)
			return
		}
		itemsById = make(map[string]data.Item, len(feedbackItems))
		for _, item := range feedbackItems {
			itemsById[item.ItemId] = item
		}
	}
	feedbackItems := make([]*logics.FeedbackItem, 0, len(feedbacks))
	for _, fb := range feedbacks {
		if item, exist := itemsById[fb.ItemId]; exist {
			feedbackItems = append(feedbackItems, &logics.FeedbackItem{
				FeedbackType: fb.FeedbackType,
				Item:         item,
			})
		}
	}

	latestItems, err := m.DataClient.GetLatestItems(ctx, 100, nil)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}

	var buf strings.Builder
	tplCtx := exec.NewContext(map[string]any{
		"user":     &user,
		"feedback": feedbackItems,
		"items":    latestItems,
	})
	if err := template.Execute(&buf, tplCtx); err != nil {
		server.InternalServerError(response, err)
		return
	}
	server.Text(response, buf.String())
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
		response.Header().Set("Content-Type", "application/jsonl")
		response.Header().Set("Content-Disposition", "attachment;filename=users.jsonl")
		encoder := json.NewEncoder(response)
		userStream, errChan := m.DataClient.GetUserStream(ctx, batchSize)
		for users := range userStream {
			for _, user := range users {
				if err = encoder.Encode(user); err != nil {
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
		// open file
		file, _, err := request.FormFile("file")
		if err != nil {
			server.BadRequest(restful.NewResponse(response), err)
			return
		}
		defer file.Close()
		// parse and import users
		decoder := json.NewDecoder(file)
		lineCount := 0
		timeStart := time.Now()
		users := make([]data.User, 0, batchSize)
		for {
			// parse line
			var user data.User
			if err = decoder.Decode(&user); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				server.BadRequest(restful.NewResponse(response), err)
				return
			}
			// validate user id
			if err = util.ValidateId(user.UserId); err != nil {
				server.BadRequest(restful.NewResponse(response),
					fmt.Errorf("invalid user id `%v` at line %d (%s)", user.UserId, lineCount, err.Error()))
				return
			}
			users = append(users, user)
			// batch insert
			if len(users) == batchSize {
				err = m.DataClient.BatchInsertUsers(ctx, users)
				if err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
				users = make([]data.User, 0, batchSize)
			}
			lineCount++
		}
		if len(users) > 0 {
			err = m.DataClient.BatchInsertUsers(ctx, users)
			if err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return
			}
		}

		m.cancel()
		select {
		case m.scheduled <- struct{}{}:
		default:
		}

		timeUsed := time.Since(timeStart)
		log.Logger().Info("complete import users",
			zap.Duration("time_used", timeUsed),
			zap.Int("num_users", lineCount))
		server.Ok(restful.NewResponse(response), server.Success{RowAffected: lineCount})
	default:
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
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
		response.Header().Set("Content-Type", "application/jsonl")
		response.Header().Set("Content-Disposition", "attachment;filename=items.jsonl")
		encoder := json.NewEncoder(response)
		itemStream, errChan := m.DataClient.GetItemStream(ctx, batchSize, nil)
		for items := range itemStream {
			for _, item := range items {
				if err = encoder.Encode(item); err != nil {
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
		// open file
		file, _, err := request.FormFile("file")
		if err != nil {
			server.BadRequest(restful.NewResponse(response), err)
			return
		}
		defer file.Close()
		// parse and import items
		decoder := json.NewDecoder(file)
		lineCount := 0
		timeStart := time.Now()
		items := make([]data.Item, 0, batchSize)
		for {
			// parse line
			var item server.Item
			if err = decoder.Decode(&item); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				server.BadRequest(restful.NewResponse(response), err)
				return
			}
			// validate item id
			if err = util.ValidateId(item.ItemId); err != nil {
				server.BadRequest(restful.NewResponse(response),
					fmt.Errorf("invalid item id `%v` at line %d (%s)", item.ItemId, lineCount, err.Error()))
				return
			}
			// validate categories
			for _, category := range item.Categories {
				if err = util.ValidateId(category); err != nil {
					server.BadRequest(restful.NewResponse(response),
						fmt.Errorf("invalid category `%v` at line %d (%s)", category, lineCount, err.Error()))
					return
				}
			}
			// parse timestamp
			var timestamp time.Time
			if item.Timestamp != "" {
				timestamp, err = dateparse.ParseAny(item.Timestamp)
				if err != nil {
					server.BadRequest(restful.NewResponse(response),
						fmt.Errorf("failed to parse datetime `%v` at line %v", item.Timestamp, lineCount))
					return
				}
			}
			items = append(items, data.Item{
				ItemId:     item.ItemId,
				IsHidden:   item.IsHidden,
				Categories: item.Categories,
				Timestamp:  timestamp,
				Labels:     item.Labels,
				Comment:    item.Comment,
			})
			// batch insert
			if len(items) == batchSize {
				err = m.DataClient.BatchInsertItems(ctx, items)
				if err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
				items = make([]data.Item, 0, batchSize)
			}
			lineCount++
		}
		if len(items) > 0 {
			err = m.DataClient.BatchInsertItems(ctx, items)
			if err != nil {
				server.InternalServerError(restful.NewResponse(response), err)
				return
			}
		}

		m.cancel()
		select {
		case m.scheduled <- struct{}{}:
		default:
		}

		timeUsed := time.Since(timeStart)
		log.Logger().Info("complete import items",
			zap.Duration("time_used", timeUsed),
			zap.Int("num_items", lineCount))
		server.Ok(restful.NewResponse(response), server.Success{RowAffected: lineCount})
	default:
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
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
		response.Header().Set("Content-Type", "application/jsonl")
		response.Header().Set("Content-Disposition", "attachment;filename=feedback.jsonl")
		encoder := json.NewEncoder(response)
		feedbackStream, errChan := m.DataClient.GetFeedbackStream(ctx, batchSize, data.WithEndTime(*m.Config.Now()))
		for feedback := range feedbackStream {
			for _, v := range feedback {
				if err = encoder.Encode(v); err != nil {
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
		// open file
		file, _, err := request.FormFile("file")
		if err != nil {
			server.BadRequest(restful.NewResponse(response), err)
			return
		}
		defer file.Close()
		// parse and import feedback
		decoder := json.NewDecoder(file)
		lineCount := 0
		timeStart := time.Now()
		feedbacks := make([]data.Feedback, 0, batchSize)
		for {
			// parse line
			var feedback server.Feedback
			if err = decoder.Decode(&feedback); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				server.BadRequest(restful.NewResponse(response), err)
				return
			}
			// validate feedback type
			if err = util.ValidateId(feedback.FeedbackType); err != nil {
				server.BadRequest(restful.NewResponse(response),
					fmt.Errorf("invalid feedback type `%v` at line %d (%s)", feedback.FeedbackType, lineCount, err.Error()))
				return
			}
			// validate user id
			if err = util.ValidateId(feedback.UserId); err != nil {
				server.BadRequest(restful.NewResponse(response),
					fmt.Errorf("invalid user id `%v` at line %d (%s)", feedback.UserId, lineCount, err.Error()))
				return
			}
			// validate item id
			if err = util.ValidateId(feedback.ItemId); err != nil {
				server.BadRequest(restful.NewResponse(response),
					fmt.Errorf("invalid item id `%v` at line %d (%s)", feedback.ItemId, lineCount, err.Error()))
				return
			}
			// parse timestamp
			var timestamp time.Time
			if feedback.Timestamp != "" {
				timestamp, err = dateparse.ParseAny(feedback.Timestamp)
				if err != nil {
					server.BadRequest(restful.NewResponse(response),
						fmt.Errorf("failed to parse datetime `%v` at line %d", feedback.Timestamp, lineCount))
					return
				}
			}
			feedbacks = append(feedbacks, data.Feedback{
				FeedbackKey: feedback.FeedbackKey,
				Value:       feedback.Value,
				Timestamp:   timestamp,
				Comment:     feedback.Comment,
			})
			// batch insert
			if len(feedbacks) == batchSize {
				// batch insert to data store
				err = m.DataClient.BatchInsertFeedback(ctx, feedbacks,
					m.Config.Server.AutoInsertUser,
					m.Config.Server.AutoInsertItem, true)
				if err != nil {
					server.InternalServerError(restful.NewResponse(response), err)
					return
				}
				feedbacks = make([]data.Feedback, 0, batchSize)
			}
			lineCount++
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

		m.cancel()
		select {
		case m.scheduled <- struct{}{}:
		default:
		}

		timeUsed := time.Since(timeStart)
		log.Logger().Info("complete import feedback",
			zap.Duration("time_used", timeUsed),
			zap.Int("num_items", lineCount))
		server.Ok(restful.NewResponse(response), server.Success{RowAffected: lineCount})
	default:
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
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

func writeError(response http.ResponseWriter, httpStatus int, message string) {
	log.Logger().Error(strings.ToLower(http.StatusText(httpStatus)), zap.String("error", message))
	response.Header().Set("Access-Control-Allow-Origin", "*")
	response.WriteHeader(httpStatus)
	if _, err := response.Write([]byte(message)); err != nil {
		log.Logger().Error("failed to write error", zap.Error(err))
	}
}

func (m *Master) checkAdmin(request *http.Request) bool {
	if m.Config.Master.AdminAPIKey == "" {
		return true
	}
	if request.FormValue("X-API-Key") == m.Config.Master.AdminAPIKey {
		return true
	}
	return false
}

const (
	EOF            = int64(0)
	UserStream     = int64(-1)
	ItemStream     = int64(-2)
	FeedbackStream = int64(-3)
)

type DumpStats struct {
	Users    int
	Items    int
	Feedback int
	Duration time.Duration
}

func writeDump[T proto.Message](w io.Writer, data T) error {
	bytes, err := proto.Marshal(data)
	if err != nil {
		return err
	}
	if err = binary.Write(w, binary.LittleEndian, int64(len(bytes))); err != nil {
		return err
	}
	if _, err = w.Write(bytes); err != nil {
		return err
	}
	return nil
}

func readDump[T proto.Message](r io.Reader, data T) (int64, error) {
	var size int64
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return 0, err
	}
	if size <= 0 {
		return size, nil
	}
	bytes := make([]byte, size)
	if _, err := io.ReadFull(r, bytes); err != nil {
		return 0, err
	}
	return size, proto.Unmarshal(bytes, data)
}

func (m *Master) dump(response http.ResponseWriter, request *http.Request) {
	if !m.checkAdmin(request) {
		writeError(response, http.StatusUnauthorized, "unauthorized")
		return
	}
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	response.Header().Set("Content-Type", "application/octet-stream")
	var stats DumpStats
	start := time.Now()
	// dump users
	if err := binary.Write(response, binary.LittleEndian, UserStream); err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	userStream, errChan := m.DataClient.GetUserStream(context.Background(), batchSize)
	for users := range userStream {
		for _, user := range users {
			labels, err := json.Marshal(user.Labels)
			if err != nil {
				writeError(response, http.StatusInternalServerError, err.Error())
				return
			}
			if err := writeDump(response, &protocol.User{
				UserId:  user.UserId,
				Labels:  labels,
				Comment: user.Comment,
			}); err != nil {
				writeError(response, http.StatusInternalServerError, err.Error())
				return
			}
			stats.Users++
		}
	}
	if err := <-errChan; err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	// dump items
	if err := binary.Write(response, binary.LittleEndian, ItemStream); err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	itemStream, errChan := m.DataClient.GetItemStream(context.Background(), batchSize, nil)
	for items := range itemStream {
		for _, item := range items {
			labels, err := json.Marshal(item.Labels)
			if err != nil {
				writeError(response, http.StatusInternalServerError, err.Error())
				return
			}
			if err := writeDump(response, &protocol.Item{
				ItemId:     item.ItemId,
				IsHidden:   item.IsHidden,
				Categories: item.Categories,
				Timestamp:  timestamppb.New(item.Timestamp),
				Labels:     labels,
				Comment:    item.Comment,
			}); err != nil {
				writeError(response, http.StatusInternalServerError, err.Error())
				return
			}
			stats.Items++
		}
	}
	if err := <-errChan; err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	// dump feedback
	if err := binary.Write(response, binary.LittleEndian, FeedbackStream); err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	feedbackStream, errChan := m.DataClient.GetFeedbackStream(context.Background(), batchSize, data.WithEndTime(*m.Config.Now()))
	for feedbacks := range feedbackStream {
		for _, feedback := range feedbacks {
			if err := writeDump(response, &protocol.Feedback{
				FeedbackType: feedback.FeedbackType,
				UserId:       feedback.UserId,
				ItemId:       feedback.ItemId,
				Value:        feedback.Value,
				Timestamp:    timestamppb.New(feedback.Timestamp),
				Comment:      feedback.Comment,
			}); err != nil {
				writeError(response, http.StatusInternalServerError, err.Error())
				return
			}
			stats.Feedback++
		}
	}
	if err := <-errChan; err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	// dump EOF
	if err := binary.Write(response, binary.LittleEndian, EOF); err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	stats.Duration = time.Since(start)
	log.Logger().Info("complete dump",
		zap.Int("users", stats.Users),
		zap.Int("items", stats.Items),
		zap.Int("feedback", stats.Feedback),
		zap.Duration("duration", stats.Duration))
	server.Ok(restful.NewResponse(response), stats)
}

func (m *Master) Restore(r io.ReadCloser) (stats DumpStats, err error) {
	flag := EOF
	if err = binary.Read(r, binary.LittleEndian, &flag); err != nil {
		return
	}
	for flag != EOF {
		switch flag {
		case UserStream:
			users := make([]data.User, 0, batchSize)
			for {
				var user protocol.User
				if flag, err = readDump(r, &user); err != nil {
					return
				}
				if flag <= 0 {
					break
				}
				var labels any
				if err = json.Unmarshal(user.Labels, &labels); err != nil {
					return
				}
				users = append(users, data.User{
					UserId:  user.UserId,
					Labels:  labels,
					Comment: user.Comment,
				})
				stats.Users++
				if len(users) == batchSize {
					if err = m.DataClient.BatchInsertUsers(context.Background(), users); err != nil {
						return
					}
					users = users[:0]
				}
			}
			if len(users) > 0 {
				if err = m.DataClient.BatchInsertUsers(context.Background(), users); err != nil {
					return
				}
			}
		case ItemStream:
			items := make([]data.Item, 0, batchSize)
			for {
				var item protocol.Item
				if flag, err = readDump(r, &item); err != nil {
					return
				}
				if flag <= 0 {
					break
				}
				var labels any
				if err = json.Unmarshal(item.Labels, &labels); err != nil {
					return
				}
				items = append(items, data.Item{
					ItemId:     item.ItemId,
					IsHidden:   item.IsHidden,
					Categories: item.Categories,
					Timestamp:  item.Timestamp.AsTime(),
					Labels:     labels,
					Comment:    item.Comment,
				})
				stats.Items++
				if len(items) == batchSize {
					if err = m.DataClient.BatchInsertItems(context.Background(), items); err != nil {
						return
					}
					items = items[:0]
				}
			}
			if len(items) > 0 {
				if err = m.DataClient.BatchInsertItems(context.Background(), items); err != nil {
					return
				}
			}
		case FeedbackStream:
			feedbacks := make([]data.Feedback, 0, batchSize)
			for {
				var feedback protocol.Feedback
				if flag, err = readDump(r, &feedback); err != nil {
					return
				}
				if flag <= 0 {
					break
				}
				feedbacks = append(feedbacks, data.Feedback{
					FeedbackKey: data.FeedbackKey{
						FeedbackType: feedback.FeedbackType,
						UserId:       feedback.UserId,
						ItemId:       feedback.ItemId,
					},
					Value:     feedback.Value,
					Timestamp: feedback.Timestamp.AsTime(),
					Comment:   feedback.Comment,
				})
				stats.Feedback++
				if len(feedbacks) == batchSize {
					if err = m.DataClient.BatchInsertFeedback(context.Background(), feedbacks, false, false, true); err != nil {
						return
					}
					feedbacks = feedbacks[:0]
				}
			}
			if len(feedbacks) > 0 {
				if err = m.DataClient.BatchInsertFeedback(context.Background(), feedbacks, false, false, true); err != nil {
					return
				}
			}
		default:
			err = fmt.Errorf("unknown flag %v", flag)
			return
		}
	}
	return
}

func (m *Master) restore(response http.ResponseWriter, request *http.Request) {
	if !m.checkAdmin(request) {
		writeError(response, http.StatusUnauthorized, "unauthorized")
		return
	}
	if request.Method != http.MethodPost {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	start := time.Now()
	stats, err := m.Restore(request.Body)
	if err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}

	m.cancel()
	select {
	case m.scheduled <- struct{}{}:
	default:
	}

	stats.Duration = time.Since(start)
	log.Logger().Info("complete restore",
		zap.Int("users", stats.Users),
		zap.Int("items", stats.Items),
		zap.Int("feedback", stats.Feedback),
		zap.Duration("duration", stats.Duration))
	server.Ok(restful.NewResponse(response), stats)
}

func (m *Master) handleOAuth2Callback(w http.ResponseWriter, r *http.Request) {
	// Verify state and errors.
	oauth2Token, err := m.oauth2Config.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		server.InternalServerError(restful.NewResponse(w), err)
		return
	}
	// Extract the ID Token from OAuth2 token.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		server.InternalServerError(restful.NewResponse(w), errors.New("missing id_token"))
		return
	}
	// Parse and verify ID Token payload.
	idToken, err := m.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		server.InternalServerError(restful.NewResponse(w), err)
		return
	}
	// Extract custom claims
	var claims UserInfo
	if err := idToken.Claims(&claims); err != nil {
		server.InternalServerError(restful.NewResponse(w), err)
		return
	}
	// Set token cache and cookie
	m.tokenCache.Set(rawIDToken, claims, time.Until(idToken.Expiry))
	if encoded, err := cookieHandler.Encode("id_token", rawIDToken); err != nil {
		server.InternalServerError(restful.NewResponse(w), err)
		return
	} else {
		http.SetCookie(w, &http.Cookie{
			Name:    "id_token",
			Value:   encoded,
			Path:    "/",
			Expires: idToken.Expiry,
		})
		http.Redirect(w, r, "/", http.StatusFound)
		log.Logger().Info("login success via OIDC",
			zap.String("name", claims.Name),
			zap.String("email", claims.Email))
	}
}

func (m *Master) chat(response http.ResponseWriter, request *http.Request) {
	if !m.checkAdmin(request) {
		writeError(response, http.StatusUnauthorized, "unauthorized")
		return
	}
	content, err := io.ReadAll(request.Body)
	if err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	stream, err := m.openAIClient.CreateChatCompletionStream(
		request.Context(),
		openai.ChatCompletionRequest{
			Model: m.Config.OpenAI.ChatCompletionModel,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: string(content),
				},
			},
			Stream: true,
		},
	)
	if err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	// read response
	defer stream.Close()
	for {
		var resp openai.ChatCompletionStreamResponse
		resp, err = stream.Recv()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			writeError(response, http.StatusInternalServerError, err.Error())
			return
		}
		if len(resp.Choices) == 0 {
			continue
		}
		if _, err = response.Write([]byte(resp.Choices[0].Delta.Content)); err != nil {
			log.Logger().Error("failed to write response", zap.Error(err))
			return
		}
		// flush response
		if f, ok := response.(http.Flusher); ok {
			f.Flush()
		}
	}
}
