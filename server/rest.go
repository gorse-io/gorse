// Copyright 2020 gorse Project Authors
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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	mapset "github.com/deckarep/golang-set/v2"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/google/uuid"
	"github.com/juju/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/lo"
	"github.com/thoas/go-funk"
	"github.com/zhenghaoz/gorse/base/heap"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"modernc.org/mathutil"
)

const (
	HealthAPITag         = "health"
	UsersAPITag          = "users"
	ItemsAPITag          = "items"
	FeedbackAPITag       = "feedback"
	RecommendationAPITag = "recommendation"
	MeasurementsAPITag   = "measurements"
	DetractedAPITag      = "deprecated"
)

// RestServer implements a REST-ful API server.
type RestServer struct {
	*config.Settings

	HttpHost string
	HttpPort int

	DisableLog bool
	WebService *restful.WebService
	HttpServer *http.Server
}

// StartHttpServer starts the REST-ful API server.
func (s *RestServer) StartHttpServer(container *restful.Container) {
	// register restful APIs
	s.CreateWebService()
	container.Add(s.WebService)
	// register swagger UI
	specConfig := restfulspec.Config{
		WebServices: []*restful.WebService{s.WebService},
		APIPath:     "/apidocs.json",
	}
	container.Add(restfulspec.NewOpenAPIService(specConfig))
	swaggerFile = specConfig.APIPath
	container.Handle(apiDocsPath, http.HandlerFunc(handler))
	// register prometheus
	container.Handle("/metrics", promhttp.Handler())
	// register pprof
	container.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	container.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	container.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	container.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	container.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	container.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	container.Handle("/debug/pprof/block", pprof.Handler("block"))
	container.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	container.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	container.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	container.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))

	// Add container filter to enable CORS
	cors := restful.CrossOriginResourceSharing{
		AllowedHeaders: []string{"Content-Type", "Accept", "X-API-Key"},
		AllowedDomains: s.Config.Master.HttpCorsDomains,
		AllowedMethods: s.Config.Master.HttpCorsMethods,
		CookiesAllowed: false,
		Container:      container}
	container.Filter(cors.Filter)

	log.Logger().Info("start http server",
		zap.String("url", fmt.Sprintf("http://%s:%d", s.HttpHost, s.HttpPort)),
		zap.Strings("cors_methods", s.Config.Master.HttpCorsMethods),
		zap.Strings("cors_domains", s.Config.Master.HttpCorsDomains),
	)
	s.HttpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.HttpHost, s.HttpPort),
		Handler: container,
	}
	if err := s.HttpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Logger().Fatal("failed to start http server", zap.Error(err))
	}
}

func (s *RestServer) LogFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	// generate request id
	requestId := uuid.New().String()
	resp.AddHeader("X-Request-ID", requestId)

	start := time.Now()
	chain.ProcessFilter(req, resp)
	responseTime := time.Since(start)
	if !s.DisableLog && req.Request.URL.Path != "/api/dashboard/cluster" &&
		req.Request.URL.Path != "/api/dashboard/tasks" {
		log.ResponseLogger(resp).Info(fmt.Sprintf("%s %s", req.Request.Method, req.Request.URL),
			zap.Int("status_code", resp.StatusCode()),
			zap.Duration("response_time", responseTime))
	}
}

func (s *RestServer) AuthFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if strings.HasPrefix(req.SelectedRoute().Path(), "/api/health/") {
		// Health check APIs don't need API key,
		chain.ProcessFilter(req, resp)
		return
	}
	if s.Config.Server.APIKey == "" {
		chain.ProcessFilter(req, resp)
		return
	}
	apikey := req.HeaderParameter("X-API-Key")
	if apikey == s.Config.Server.APIKey {
		chain.ProcessFilter(req, resp)
		return
	}
	log.ResponseLogger(resp).Error("unauthorized",
		zap.String("api_key", s.Config.Server.APIKey),
		zap.String("X-API-Key", apikey))
	if err := resp.WriteError(http.StatusUnauthorized, fmt.Errorf("unauthorized")); err != nil {
		log.ResponseLogger(resp).Error("failed to write error", zap.Error(err))
	}
}

func (s *RestServer) MetricsFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	startTime := time.Now()
	chain.ProcessFilter(req, resp)
	if req.SelectedRoute() != nil && resp.StatusCode() == http.StatusOK {
		routePath := req.SelectedRoutePath()
		if !strings.HasPrefix(routePath, "/api/dashboard") {
			RestAPIRequestSecondsVec.WithLabelValues(fmt.Sprintf("%s %s", req.Request.Method, routePath)).
				Observe(time.Since(startTime).Seconds())
		}
	}
}

// CreateWebService creates web service.
func (s *RestServer) CreateWebService() {
	// Create a server
	ws := s.WebService
	ws.Path("/api/").
		Produces(restful.MIME_JSON).
		Filter(s.LogFilter).
		Filter(s.AuthFilter).
		Filter(s.MetricsFilter).
		Filter(otelrestful.OTelFilter("gorse"))

	/* Health check */
	ws.Route(ws.GET("/health/live").To(s.checkLive).
		Doc("Probe the liveness of this node. Return OK once the server starts.").
		Metadata(restfulspec.KeyOpenAPITags, []string{HealthAPITag}).
		Returns(http.StatusOK, "OK", HealthStatus{}).
		Writes(HealthStatus{}))
	ws.Route(ws.GET("/health/ready").To(s.checkReady).
		Doc("Probe the readiness of this node. Return OK if the server is able to handle requests.").
		Metadata(restfulspec.KeyOpenAPITags, []string{HealthAPITag}).
		Returns(http.StatusOK, "OK", HealthStatus{}).
		Writes(HealthStatus{}))

	// Insert a user
	ws.Route(ws.POST("/user").To(s.insertUser).
		Doc("Insert a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{UsersAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Reads(data.User{}).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	// Modify a user
	ws.Route(ws.PATCH("/user/{user-id}").To(s.modifyUser).
		Doc("Modify a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{UsersAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "ID of the user to modify").DataType("string")).
		Reads(data.UserPatch{}).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	// Get a user
	ws.Route(ws.GET("/user/{user-id}").To(s.getUser).
		Doc("Get a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{UsersAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "ID of the user to get").DataType("string")).
		Returns(http.StatusOK, "OK", data.User{}).
		Writes(data.User{}))
	// Insert users
	ws.Route(ws.POST("/users").To(s.insertUsers).
		Doc("Insert users.").
		Metadata(restfulspec.KeyOpenAPITags, []string{UsersAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Reads([]data.User{}).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	// Get users
	ws.Route(ws.GET("/users").To(s.getUsers).
		Doc("Get users.").
		Metadata(restfulspec.KeyOpenAPITags, []string{UsersAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned users").DataType("integer")).
		Param(ws.QueryParameter("cursor", "Cursor for the next page").DataType("string")).
		Returns(http.StatusOK, "OK", UserIterator{}).
		Writes(UserIterator{}))
	// Delete a user
	ws.Route(ws.DELETE("/user/{user-id}").To(s.deleteUser).
		Doc("Delete a user and his or her feedback.").
		Metadata(restfulspec.KeyOpenAPITags, []string{UsersAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "ID of the user to delete").DataType("string")).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))

	// Insert an item
	ws.Route(ws.POST("/item").To(s.insertItem).
		Doc("Insert an item. Overwrite if the item exists.").
		Metadata(restfulspec.KeyOpenAPITags, []string{ItemsAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Reads(data.Item{}).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	// Modify an item
	ws.Route(ws.PATCH("/item/{item-id}").To(s.modifyItem).
		Doc("Modify an item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{ItemsAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("item-id", "ID of the item to modify").DataType("string")).
		Reads(data.ItemPatch{}).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	// Get items
	ws.Route(ws.GET("/items").To(s.getItems).
		Doc("Get items.").
		Metadata(restfulspec.KeyOpenAPITags, []string{ItemsAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("cursor", "Cursor for the next page").DataType("string")).
		Returns(http.StatusOK, "OK", ItemIterator{}).
		Writes(ItemIterator{}))
	// Get item
	ws.Route(ws.GET("/item/{item-id}").To(s.getItem).
		Doc("Get a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{ItemsAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("item-id", "ID of the item to get.").DataType("string")).
		Returns(http.StatusOK, "OK", data.Item{}).
		Writes(data.Item{}))
	// Insert items
	ws.Route(ws.POST("/items").To(s.insertItems).
		Doc("Insert items. Overwrite if items exist").
		Metadata(restfulspec.KeyOpenAPITags, []string{ItemsAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Reads([]data.Item{}).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	// Delete item
	ws.Route(ws.DELETE("/item/{item-id}").To(s.deleteItem).
		Doc("Delete an item and its feedback.").
		Metadata(restfulspec.KeyOpenAPITags, []string{ItemsAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("item-id", "ID of the item to delete").DataType("string")).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	// Insert category
	ws.Route(ws.PUT("/item/{item-id}/category/{category}").To(s.insertItemCategory).
		Doc("Insert a category for a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{ItemsAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("item-id", "ID of the item to insert category").DataType("string")).
		Param(ws.PathParameter("category", "Category to insert").DataType("string")).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	// Delete category
	ws.Route(ws.DELETE("/item/{item-id}/category/{category}").To(s.deleteItemCategory).
		Doc("Delete a category from a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{ItemsAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("item-id", "ID of the item to delete categoryßßß").DataType("string")).
		Param(ws.PathParameter("category", "Category to delete").DataType("string")).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	// Insert feedback
	ws.Route(ws.POST("/feedback").To(s.insertFeedback(false)).
		Doc("Insert feedbacks. Ignore insertion if feedback exists.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Reads([]data.Feedback{}).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	ws.Route(ws.PUT("/feedback").To(s.insertFeedback(true)).
		Doc("Insert feedbacks. Existed feedback will be overwritten.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Reads([]data.Feedback{}).
		Returns(http.StatusOK, "OK", Success{}).
		Writes(Success{}))
	// Get feedback
	ws.Route(ws.GET("/feedback").To(s.getFeedback).
		Doc("Get feedbacks.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.QueryParameter("cursor", "Cursor for the next page").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned feedback").DataType("integer")).
		Returns(http.StatusOK, "OK", FeedbackIterator{}).
		Writes(FeedbackIterator{}))
	ws.Route(ws.GET("/feedback/{user-id}/{item-id}").To(s.getUserItemFeedback).
		Doc("Get feedbacks between a user and a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "User ID of returned feedbacks").DataType("string")).
		Param(ws.PathParameter("item-id", "Item ID of returned feedbacks").DataType("string")).
		Returns(http.StatusOK, "OK", []data.Feedback{}).
		Writes([]data.Feedback{}))
	ws.Route(ws.DELETE("/feedback/{user-id}/{item-id}").To(s.deleteUserItemFeedback).
		Doc("Delete feedbacks between a user and a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "User ID of returned feedbacks").DataType("string")).
		Param(ws.PathParameter("item-id", "Item ID of returned feedbacks").DataType("string")).
		Returns(http.StatusOK, "OK", []data.Feedback{}).
		Writes([]data.Feedback{}))
	ws.Route(ws.GET("/feedback/{feedback-type}").To(s.getTypedFeedback).
		Doc("Get feedbacks with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("feedback-type", "Type of returned feedbacks").DataType("string")).
		Param(ws.QueryParameter("cursor", "Cursor for the next page").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned feedbacks").DataType("integer")).
		Returns(http.StatusOK, "OK", FeedbackIterator{}).
		Writes(FeedbackIterator{}))
	ws.Route(ws.GET("/feedback/{feedback-type}/{user-id}/{item-id}").To(s.getTypedUserItemFeedback).
		Doc("Get feedbacks between a user and a item with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("feedback-type", "Type of returned feedbacks").DataType("string")).
		Param(ws.PathParameter("user-id", "User ID of returned feedbacks").DataType("string")).
		Param(ws.PathParameter("item-id", "Item ID of returned feedbacks").DataType("string")).
		Returns(http.StatusOK, "OK", data.Feedback{}).
		Writes(data.Feedback{}))
	ws.Route(ws.DELETE("/feedback/{feedback-type}/{user-id}/{item-id}").To(s.deleteTypedUserItemFeedback).
		Doc("Delete feedbacks between a user and a item with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("feedback-type", "Type of returned feedbacks").DataType("string")).
		Param(ws.PathParameter("user-id", "User ID of returned feedbacks").DataType("string")).
		Param(ws.PathParameter("item-id", "Item ID of returned feedbacks").DataType("string")).
		Returns(http.StatusOK, "OK", data.Feedback{}).
		Writes(data.Feedback{}))
	// Get feedback by user id
	ws.Route(ws.GET("/user/{user-id}/feedback/{feedback-type}").To(s.getTypedFeedbackByUser).
		Doc("Get feedbacks by user id with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "User ID of returned feedbacks").DataType("string")).
		Param(ws.PathParameter("feedback-type", "Type of returned feedbacks").DataType("string")).
		Returns(http.StatusOK, "OK", []data.Feedback{}).
		Writes([]data.Feedback{}))
	ws.Route(ws.GET("/user/{user-id}/feedback").To(s.getFeedbackByUser).
		Doc("Get feedbacks by user id.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "User ID of returned feedbacks").DataType("string")).
		Returns(http.StatusOK, "OK", []data.Feedback{}).
		Writes([]data.Feedback{}))
	// Get feedback by item-id
	ws.Route(ws.GET("/item/{item-id}/feedback/{feedback-type}").To(s.getTypedFeedbackByItem).
		Doc("Get feedbacks by item id with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("item-id", "Item ID of returned feedbacks").DataType("string")).
		Param(ws.PathParameter("feedback-type", "Type of returned feedbacks").DataType("string")).
		Returns(http.StatusOK, "OK", []data.Feedback{}).
		Writes([]data.Feedback{}))
	ws.Route(ws.GET("/item/{item-id}/feedback/").To(s.getFeedbackByItem).
		Doc("Get feedbacks by item id.").
		Metadata(restfulspec.KeyOpenAPITags, []string{FeedbackAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("item-id", "Item ID of returned feedbacks").DataType("string")).
		Returns(http.StatusOK, "OK", []data.Feedback{}).
		Writes([]data.Feedback{}))

	// Get collaborative filtering recommendation by user id
	ws.Route(ws.GET("/intermediate/recommend/{user-id}").To(s.getCollaborative).
		Doc("Get the collaborative filtering recommendation for a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{DetractedAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "ID of the user to get recommendation").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))
	ws.Route(ws.GET("/intermediate/recommend/{user-id}/{category}").To(s.getCollaborative).
		Doc("Get the collaborative filtering recommendation for a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{DetractedAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "ID of the user to get recommendation").DataType("string")).
		Param(ws.PathParameter("category", "Category of returned items.").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))

	// Get popular items
	ws.Route(ws.GET("/popular").To(s.getPopular).
		Doc("Get popular items.").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned recommendations").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned recommendations").DataType("integer")).
		Param(ws.QueryParameter("user-id", "Remove read items of a user").DataType("string")).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))
	ws.Route(ws.GET("/popular/{category}").To(s.getPopular).
		Doc("Get popular items in category.").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("category", "Category of returned items.").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Param(ws.QueryParameter("user-id", "Remove read items of a user").DataType("string")).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))
	// Get latest items
	ws.Route(ws.GET("/latest").To(s.getLatest).
		Doc("Get the latest items.").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Param(ws.QueryParameter("user-id", "Remove read items of a user").DataType("string")).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))
	ws.Route(ws.GET("/latest/{category}").To(s.getLatest).
		Doc("Get the latest items in category.").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("category", "Category of returned items.").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Param(ws.QueryParameter("user-id", "Remove read items of a user").DataType("string")).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))
	// Get neighbors
	ws.Route(ws.GET("/item/{item-id}/neighbors/").To(s.getItemNeighbors).
		Doc("Get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("item-id", "ID of the item to get neighbors").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))
	ws.Route(ws.GET("/item/{item-id}/neighbors/{category}").To(s.getItemNeighbors).
		Doc("Get neighbors of a item in category.").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("item-id", "ID of the item to get neighbors").DataType("string")).
		Param(ws.PathParameter("category", "Category of returned items").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))
	ws.Route(ws.GET("/user/{user-id}/neighbors/").To(s.getUserNeighbors).
		Doc("Get neighbors of a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "ID of the user to get neighbors").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned users").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned users").DataType("integer")).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))
	ws.Route(ws.GET("/recommend/{user-id}").To(s.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "ID of the user to get recommendation").DataType("string")).
		Param(ws.QueryParameter("category", "Category of the returned items (support multi-categories filtering)").DataType("string")).
		Param(ws.QueryParameter("write-back-type", "Type of write back feedback").DataType("string")).
		Param(ws.QueryParameter("write-back-delay", "Timestamp delay of write back feedback (format 0h0m0s)").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Returns(http.StatusOK, "OK", []string{}).
		Writes([]string{}))
	ws.Route(ws.GET("/recommend/{user-id}/{category}").To(s.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("user-id", "ID of the user to get recommendation").DataType("string")).
		Param(ws.PathParameter("category", "Category of the returned items").DataType("string")).
		Param(ws.QueryParameter("write-back-type", "Type of write back feedback").DataType("string")).
		Param(ws.QueryParameter("write-back-delay", "Timestamp delay of write back feedback (format 0h0m0s)").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Returns(http.StatusOK, "OK", []string{}).
		Writes([]string{}))
	ws.Route(ws.POST("/session/recommend").To(s.sessionRecommend).
		Doc("Get recommendation for session.").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Reads([]Feedback{}).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))
	ws.Route(ws.POST("/session/recommend/{category}").To(s.sessionRecommend).
		Doc("Get recommendation for session.").
		Metadata(restfulspec.KeyOpenAPITags, []string{RecommendationAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("category", "Category of the returned items").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned items").DataType("integer")).
		Param(ws.QueryParameter("offset", "Offset of returned items").DataType("integer")).
		Reads([]Feedback{}).
		Returns(http.StatusOK, "OK", []cache.Score{}).
		Writes([]cache.Score{}))

	ws.Route(ws.GET("/measurements/{name}").To(s.getMeasurements).
		Doc("Get measurements.").
		Metadata(restfulspec.KeyOpenAPITags, []string{MeasurementsAPITag}).
		Param(ws.HeaderParameter("X-API-Key", "API key").DataType("string")).
		Param(ws.PathParameter("name", "Name of returned measurements").DataType("string")).
		Param(ws.QueryParameter("n", "Number of returned measurements").DataType("integer")).
		Returns(http.StatusOK, "OK", []cache.TimeSeriesPoint{}).
		Writes([]cache.TimeSeriesPoint{}))
}

// ParseInt parses integers from the query parameter.
func ParseInt(request *restful.Request, name string, fallback int) (value int, err error) {
	valueString := request.QueryParameter(name)
	value, err = strconv.Atoi(valueString)
	if err != nil && valueString == "" {
		value = fallback
		err = nil
	}
	return
}

// ParseDuration parses duration from the query parameter.
func ParseDuration(request *restful.Request, name string) (time.Duration, error) {
	valueString := request.QueryParameter(name)
	if valueString == "" {
		return 0, nil
	}
	return time.ParseDuration(valueString)
}

func (s *RestServer) searchDocuments(collection, subset, category string, isItem bool, request *restful.Request, response *restful.Response) {
	var (
		ctx    = request.Request.Context()
		n      int
		offset int
		userId string
		err    error
	)

	// parse arguments
	if offset, err = ParseInt(request, "offset", 0); err != nil {
		BadRequest(response, err)
		return
	}
	if n, err = ParseInt(request, "n", s.Config.Server.DefaultN); err != nil {
		BadRequest(response, err)
		return
	}
	userId = request.QueryParameter("user-id")

	readItems := mapset.NewSet[string]()
	if userId != "" {
		feedback, err := s.DataClient.GetUserFeedback(ctx, userId, s.Config.Now())
		if err != nil {
			InternalServerError(response, err)
			return
		}
		for _, f := range feedback {
			readItems.Add(f.ItemId)
		}
	}

	end := offset + n
	if end > 0 && readItems.Cardinality() > 0 {
		end += readItems.Cardinality()
	}

	// Get the sorted list
	items, err := s.CacheClient.SearchScores(ctx, collection, subset, []string{category}, offset, end)
	if err != nil {
		InternalServerError(response, err)
		return
	}

	// Remove read items
	if userId != "" {
		prunedItems := make([]cache.Score, 0, len(items))
		for _, item := range items {
			if !readItems.Contains(item.Id) {
				prunedItems = append(prunedItems, item)
			}
		}
		items = prunedItems
	}

	// Send result
	if n > 0 && len(items) > n {
		items = items[:n]
	}
	Ok(response, items)
}

func (s *RestServer) getPopular(request *restful.Request, response *restful.Response) {
	category := request.PathParameter("category")
	log.ResponseLogger(response).Debug("get category popular items in category", zap.String("category", category))
	s.searchDocuments(cache.PopularItems, "", category, true, request, response)
}

func (s *RestServer) getLatest(request *restful.Request, response *restful.Response) {
	category := request.PathParameter("category")
	log.ResponseLogger(response).Debug("get category latest items in category", zap.String("category", category))
	s.searchDocuments(cache.LatestItems, "", category, true, request, response)
}

// get feedback by item-id with feedback type
func (s *RestServer) getTypedFeedbackByItem(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	feedbackType := request.PathParameter("feedback-type")
	itemId := request.PathParameter("item-id")
	feedback, err := s.DataClient.GetItemFeedback(ctx, itemId, feedbackType)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, feedback)
}

// get feedback by item-id
func (s *RestServer) getFeedbackByItem(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	itemId := request.PathParameter("item-id")
	feedback, err := s.DataClient.GetItemFeedback(ctx, itemId)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, feedback)
}

// getItemNeighbors gets neighbors of a item from database.
func (s *RestServer) getItemNeighbors(request *restful.Request, response *restful.Response) {
	// Get item id
	itemId := request.PathParameter("item-id")
	category := request.PathParameter("category")
	s.searchDocuments(cache.ItemNeighbors, itemId, category, true, request, response)
}

// getUserNeighbors gets neighbors of a user from database.
func (s *RestServer) getUserNeighbors(request *restful.Request, response *restful.Response) {
	// Get item id
	userId := request.PathParameter("user-id")
	s.searchDocuments(cache.UserNeighbors, userId, "", false, request, response)
}

// getCollaborative gets cached recommended items from database.
func (s *RestServer) getCollaborative(request *restful.Request, response *restful.Response) {
	// Get user id
	userId := request.PathParameter("user-id")
	category := request.PathParameter("category")
	s.searchDocuments(cache.OfflineRecommend, userId, category, true, request, response)
}

// Recommend items to users.
// 1. If there are recommendations in cache, return cached recommendations.
// 2. If there are historical interactions of the users, return similar items.
// 3. Otherwise, return fallback recommendation (popular/latest).
func (s *RestServer) Recommend(ctx context.Context, response *restful.Response, userId string, categories []string, n int, recommenders ...Recommender) ([]string, error) {
	initStart := time.Now()

	// create context
	recommendCtx, err := s.createRecommendContext(ctx, userId, categories, n)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// execute recommenders
	for _, recommender := range recommenders {
		err = recommender(recommendCtx)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	// return recommendations
	if len(recommendCtx.results) > n {
		recommendCtx.results = recommendCtx.results[:n]
	}
	totalTime := time.Since(initStart)
	log.ResponseLogger(response).Info("complete recommendation",
		zap.Int("num_from_final", recommendCtx.numFromOffline),
		zap.Int("num_from_collaborative", recommendCtx.numFromCollaborative),
		zap.Int("num_from_item_based", recommendCtx.numFromItemBased),
		zap.Int("num_from_user_based", recommendCtx.numFromUserBased),
		zap.Int("num_from_latest", recommendCtx.numFromLatest),
		zap.Int("num_from_poplar", recommendCtx.numFromPopular),
		zap.Duration("total_time", totalTime),
		zap.Duration("load_final_recommend_time", recommendCtx.loadOfflineRecTime),
		zap.Duration("load_col_recommend_time", recommendCtx.loadColRecTime),
		zap.Duration("load_hist_time", recommendCtx.loadLoadHistTime),
		zap.Duration("item_based_recommend_time", recommendCtx.itemBasedTime),
		zap.Duration("user_based_recommend_time", recommendCtx.userBasedTime),
		zap.Duration("load_latest_time", recommendCtx.loadLatestTime),
		zap.Duration("load_popular_time", recommendCtx.loadPopularTime))
	return recommendCtx.results, nil
}

type recommendContext struct {
	context      context.Context
	userId       string
	categories   []string
	userFeedback []data.Feedback
	n            int
	results      []string
	excludeSet   mapset.Set[string]

	numPrevStage         int
	numFromLatest        int
	numFromPopular       int
	numFromUserBased     int
	numFromItemBased     int
	numFromCollaborative int
	numFromOffline       int

	loadOfflineRecTime time.Duration
	loadColRecTime     time.Duration
	loadLoadHistTime   time.Duration
	itemBasedTime      time.Duration
	userBasedTime      time.Duration
	loadLatestTime     time.Duration
	loadPopularTime    time.Duration
}

func (s *RestServer) createRecommendContext(ctx context.Context, userId string, categories []string, n int) (*recommendContext, error) {
	// pull historical feedback
	userFeedback, err := s.DataClient.GetUserFeedback(ctx, userId, s.Config.Now())
	if err != nil {
		return nil, errors.Trace(err)
	}
	excludeSet := mapset.NewSet[string]()
	for _, item := range userFeedback {
		if !s.Config.Recommend.Replacement.EnableReplacement {
			excludeSet.Add(item.ItemId)
		}
	}
	return &recommendContext{
		userId:       userId,
		categories:   categories,
		n:            n,
		excludeSet:   excludeSet,
		userFeedback: userFeedback,
		context:      ctx,
	}, nil
}

type Recommender func(ctx *recommendContext) error

func (s *RestServer) RecommendOffline(ctx *recommendContext) error {
	if len(ctx.results) < ctx.n {
		start := time.Now()
		recommendation, err := s.CacheClient.SearchScores(ctx.context, cache.OfflineRecommend, ctx.userId, ctx.categories, 0, s.Config.Recommend.CacheSize)
		if err != nil {
			return errors.Trace(err)
		}
		for _, item := range recommendation {
			if !ctx.excludeSet.Contains(item.Id) {
				ctx.results = append(ctx.results, item.Id)
				ctx.excludeSet.Add(item.Id)
			}
		}
		ctx.loadOfflineRecTime = time.Since(start)
		ctx.numFromOffline = len(ctx.results) - ctx.numPrevStage
		ctx.numPrevStage = len(ctx.results)
	}
	return nil
}

func (s *RestServer) RecommendCollaborative(ctx *recommendContext) error {
	if len(ctx.results) < ctx.n {
		start := time.Now()
		collaborativeRecommendation, err := s.CacheClient.SearchScores(ctx.context, cache.CollaborativeRecommend, ctx.userId, ctx.categories, 0, s.Config.Recommend.CacheSize)
		if err != nil {
			return errors.Trace(err)
		}
		for _, item := range collaborativeRecommendation {
			if !ctx.excludeSet.Contains(item.Id) {
				ctx.results = append(ctx.results, item.Id)
				ctx.excludeSet.Add(item.Id)
			}
		}
		ctx.loadColRecTime = time.Since(start)
		ctx.numFromCollaborative = len(ctx.results) - ctx.numPrevStage
		ctx.numPrevStage = len(ctx.results)
	}
	return nil
}

func (s *RestServer) RecommendUserBased(ctx *recommendContext) error {
	if len(ctx.results) < ctx.n {
		start := time.Now()
		candidates := make(map[string]float64)
		// load similar users
		similarUsers, err := s.CacheClient.SearchScores(ctx.context, cache.UserNeighbors, ctx.userId, []string{""}, 0, s.Config.Recommend.CacheSize)
		if err != nil {
			return errors.Trace(err)
		}
		for _, user := range similarUsers {
			// load historical feedback
			feedbacks, err := s.DataClient.GetUserFeedback(ctx.context, user.Id, s.Config.Now(), s.Config.Recommend.DataSource.PositiveFeedbackTypes...)
			if err != nil {
				return errors.Trace(err)
			}
			// add unseen items
			for _, feedback := range feedbacks {
				if !ctx.excludeSet.Contains(feedback.ItemId) {
					item, err := s.DataClient.GetItem(ctx.context, feedback.ItemId)
					if err != nil {
						return errors.Trace(err)
					}
					if funk.Equal(ctx.categories, []string{""}) || funk.Subset(ctx.categories, item.Categories) {
						candidates[feedback.ItemId] += user.Score
					}
				}
			}
		}
		// collect top k
		k := ctx.n - len(ctx.results)
		filter := heap.NewTopKFilter[string, float64](k)
		for id, score := range candidates {
			filter.Push(id, score)
		}
		ids, _ := filter.PopAll()
		ctx.results = append(ctx.results, ids...)
		ctx.excludeSet.Append(ids...)
		ctx.userBasedTime = time.Since(start)
		ctx.numFromUserBased = len(ctx.results) - ctx.numPrevStage
		ctx.numPrevStage = len(ctx.results)
	}
	return nil
}

func (s *RestServer) RecommendItemBased(ctx *recommendContext) error {
	if len(ctx.results) < ctx.n {
		start := time.Now()
		// truncate user feedback
		data.SortFeedbacks(ctx.userFeedback)
		userFeedback := make([]data.Feedback, 0, s.Config.Recommend.Online.NumFeedbackFallbackItemBased)
		for _, feedback := range ctx.userFeedback {
			if s.Config.Recommend.Online.NumFeedbackFallbackItemBased <= len(userFeedback) {
				break
			}
			if funk.ContainsString(s.Config.Recommend.DataSource.PositiveFeedbackTypes, feedback.FeedbackType) {
				userFeedback = append(userFeedback, feedback)
			}
		}
		// collect candidates
		candidates := make(map[string]float64)
		for _, feedback := range userFeedback {
			// load similar items
			similarItems, err := s.CacheClient.SearchScores(ctx.context, cache.ItemNeighbors, feedback.ItemId, ctx.categories, 0, s.Config.Recommend.CacheSize)
			if err != nil {
				return errors.Trace(err)
			}
			// add unseen items
			for _, item := range similarItems {
				if !ctx.excludeSet.Contains(item.Id) {
					candidates[item.Id] += item.Score
				}
			}
		}
		// collect top k
		k := ctx.n - len(ctx.results)
		filter := heap.NewTopKFilter[string, float64](k)
		for id, score := range candidates {
			filter.Push(id, score)
		}
		ids, _ := filter.PopAll()
		ctx.results = append(ctx.results, ids...)
		ctx.excludeSet.Append(ids...)
		ctx.itemBasedTime = time.Since(start)
		ctx.numFromItemBased = len(ctx.results) - ctx.numPrevStage
		ctx.numPrevStage = len(ctx.results)
	}
	return nil
}

func (s *RestServer) RecommendLatest(ctx *recommendContext) error {
	if len(ctx.results) < ctx.n {
		start := time.Now()
		items, err := s.CacheClient.SearchScores(ctx.context, cache.LatestItems, "", ctx.categories, 0, s.Config.Recommend.CacheSize)
		if err != nil {
			return errors.Trace(err)
		}
		for _, item := range items {
			if !ctx.excludeSet.Contains(item.Id) {
				ctx.results = append(ctx.results, item.Id)
				ctx.excludeSet.Add(item.Id)
			}
		}
		ctx.loadLatestTime = time.Since(start)
		ctx.numFromLatest = len(ctx.results) - ctx.numPrevStage
		ctx.numPrevStage = len(ctx.results)
	}
	return nil
}

func (s *RestServer) RecommendPopular(ctx *recommendContext) error {
	if len(ctx.results) < ctx.n {
		start := time.Now()
		items, err := s.CacheClient.SearchScores(ctx.context, cache.PopularItems, "", ctx.categories, 0, s.Config.Recommend.CacheSize)
		if err != nil {
			return errors.Trace(err)
		}
		for _, item := range items {
			if !ctx.excludeSet.Contains(item.Id) {
				ctx.results = append(ctx.results, item.Id)
				ctx.excludeSet.Add(item.Id)
			}
		}
		ctx.loadPopularTime = time.Since(start)
		ctx.numFromPopular = len(ctx.results) - ctx.numPrevStage
		ctx.numPrevStage = len(ctx.results)
	}
	return nil
}

func (s *RestServer) getRecommend(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// parse arguments
	userId := request.PathParameter("user-id")
	n, err := ParseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	categories := request.QueryParameters("category")
	if len(categories) == 0 {
		categories = []string{request.PathParameter("category")}
	}
	offset, err := ParseInt(request, "offset", 0)
	if err != nil {
		BadRequest(response, err)
		return
	}
	writeBackFeedback := request.QueryParameter("write-back-type")
	writeBackDelay, err := ParseDuration(request, "write-back-delay")
	if err != nil {
		BadRequest(response, err)
		return
	}
	// online recommendation
	recommenders := []Recommender{s.RecommendOffline}
	for _, recommender := range s.Config.Recommend.Online.FallbackRecommend {
		switch recommender {
		case "collaborative":
			recommenders = append(recommenders, s.RecommendCollaborative)
		case "item_based":
			recommenders = append(recommenders, s.RecommendItemBased)
		case "user_based":
			recommenders = append(recommenders, s.RecommendUserBased)
		case "latest":
			recommenders = append(recommenders, s.RecommendLatest)
		case "popular":
			recommenders = append(recommenders, s.RecommendPopular)
		default:
			InternalServerError(response, fmt.Errorf("unknown fallback recommendation method `%s`", recommender))
			return
		}
	}
	results, err := s.Recommend(ctx, response, userId, categories, offset+n, recommenders...)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	results = results[mathutil.Min(offset, len(results)):]
	// write back
	if writeBackFeedback != "" {
		startTime := time.Now()
		for _, itemId := range results {
			// insert to data store
			feedback := data.Feedback{
				FeedbackKey: data.FeedbackKey{
					UserId:       userId,
					ItemId:       itemId,
					FeedbackType: writeBackFeedback,
				},
				Timestamp: startTime.Add(writeBackDelay),
			}
			err = s.DataClient.BatchInsertFeedback(ctx, []data.Feedback{feedback}, false, false, false)
			if err != nil {
				InternalServerError(response, err)
				return
			}
		}
	}
	// Send result
	Ok(response, results)
}

func (s *RestServer) sessionRecommend(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// parse arguments
	var feedbacks []Feedback
	if err := request.ReadEntity(&feedbacks); err != nil {
		BadRequest(response, err)
		return
	}
	n, err := ParseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	category := request.PathParameter("category")
	offset, err := ParseInt(request, "offset", 0)
	if err != nil {
		BadRequest(response, err)
		return
	}

	// pre-process feedback
	dataFeedback := make([]data.Feedback, len(feedbacks))
	for i := range dataFeedback {
		var err error
		dataFeedback[i], err = feedbacks[i].ToDataFeedback()
		if err != nil {
			BadRequest(response, err)
			return
		}
	}
	data.SortFeedbacks(dataFeedback)

	// item-based recommendation
	var excludeSet = mapset.NewSet[string]()
	var userFeedback []data.Feedback
	for _, feedback := range dataFeedback {
		excludeSet.Add(feedback.ItemId)
		if funk.ContainsString(s.Config.Recommend.DataSource.PositiveFeedbackTypes, feedback.FeedbackType) {
			userFeedback = append(userFeedback, feedback)
		}
	}
	// collect candidates
	candidates := make(map[string]float64)
	usedFeedbackCount := 0
	for _, feedback := range userFeedback {
		// load similar items
		similarItems, err := s.CacheClient.SearchScores(ctx, cache.ItemNeighbors, feedback.ItemId, []string{category}, 0, s.Config.Recommend.CacheSize)
		if err != nil {
			BadRequest(response, err)
			return
		}
		// add unseen items
		// similarItems = s.FilterOutHiddenScores(response, similarItems, "")
		for _, item := range similarItems {
			if !excludeSet.Contains(item.Id) {
				candidates[item.Id] += item.Score
			}
		}
		// finish recommendation if the number of used feedbacks is enough
		if len(similarItems) > 0 {
			usedFeedbackCount++
			if usedFeedbackCount >= s.Config.Recommend.Online.NumFeedbackFallbackItemBased {
				break
			}
		}
	}
	// collect top k
	filter := heap.NewTopKFilter[string, float64](n + offset)
	for id, score := range candidates {
		filter.Push(id, score)
	}
	names, scores := filter.PopAll()
	result := lo.Map(names, func(_ string, i int) cache.Score {
		return cache.Score{
			Id:    names[i],
			Score: scores[i],
		}
	})
	if len(result) > offset {
		result = result[offset:]
	} else {
		result = nil
	}
	result = result[:lo.Min([]int{len(result), n})]
	// Send result
	Ok(response, result)
}

// Success is the returned data structure for data insert operations.
type Success struct {
	RowAffected int
}

func (s *RestServer) insertUser(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	temp := data.User{}
	// get userInfo from request and put into temp
	if err := request.ReadEntity(&temp); err != nil {
		BadRequest(response, err)
		return
	}
	// validate labels
	if err := data.ValidateLabels(temp.Labels); err != nil {
		BadRequest(response, err)
		return
	}
	if err := s.DataClient.BatchInsertUsers(ctx, []data.User{temp}); err != nil {
		InternalServerError(response, err)
		return
	}
	// insert modify timestamp
	if err := s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, temp.UserId), time.Now())); err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, Success{RowAffected: 1})
}

func (s *RestServer) modifyUser(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// get user id
	userId := request.PathParameter("user-id")
	// modify user
	var patch data.UserPatch
	if err := request.ReadEntity(&patch); err != nil {
		BadRequest(response, err)
		return
	}
	// validate labels
	if err := data.ValidateLabels(patch.Labels); err != nil {
		BadRequest(response, err)
		return
	}
	if err := s.DataClient.ModifyUser(ctx, userId, patch); err != nil {
		InternalServerError(response, err)
		return
	}
	// insert modify timestamp
	if err := s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, userId), time.Now())); err != nil {
		return
	}
	Ok(response, Success{RowAffected: 1})
}

func (s *RestServer) getUser(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// get user id
	userId := request.PathParameter("user-id")
	// get user
	user, err := s.DataClient.GetUser(ctx, userId)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			PageNotFound(response, err)
		} else {
			InternalServerError(response, err)
		}
		return
	}
	Ok(response, user)
}

func (s *RestServer) insertUsers(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	var temp []data.User
	// get param from request and put into temp
	if err := request.ReadEntity(&temp); err != nil {
		BadRequest(response, err)
		return
	}
	// validate labels
	for _, user := range temp {
		if err := data.ValidateLabels(user.Labels); err != nil {
			BadRequest(response, err)
			return
		}
	}
	// range temp and achieve user
	if err := s.DataClient.BatchInsertUsers(ctx, temp); err != nil {
		InternalServerError(response, err)
		return
	}
	// insert modify timestamp
	values := make([]cache.Value, len(temp))
	for i, user := range temp {
		values[i] = cache.Time(cache.Key(cache.LastModifyUserTime, user.UserId), time.Now())
	}
	if err := s.CacheClient.Set(ctx, values...); err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, Success{RowAffected: len(temp)})
}

type UserIterator struct {
	Cursor string
	Users  []data.User
}

func (s *RestServer) getUsers(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	cursor := request.QueryParameter("cursor")
	n, err := ParseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	// get all users
	cursor, users, err := s.DataClient.GetUsers(ctx, cursor, n)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, UserIterator{Cursor: cursor, Users: users})
}

// delete a user by user-id
func (s *RestServer) deleteUser(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// get user-id and put into temp
	userId := request.PathParameter("user-id")
	if err := s.DataClient.DeleteUser(ctx, userId); err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, Success{RowAffected: 1})
}

// get feedback by user-id with feedback type
func (s *RestServer) getTypedFeedbackByUser(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	feedback, err := s.DataClient.GetUserFeedback(ctx, userId, s.Config.Now(), feedbackType)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, feedback)
}

// get feedback by user-id
func (s *RestServer) getFeedbackByUser(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	userId := request.PathParameter("user-id")
	feedback, err := s.DataClient.GetUserFeedback(ctx, userId, s.Config.Now())
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, feedback)
}

// Item is the data structure for the item but stores the timestamp using string.
type Item struct {
	ItemId     string
	IsHidden   bool
	Categories []string
	Timestamp  string
	Labels     any
	Comment    string
}

func (s *RestServer) batchInsertItems(ctx context.Context, response *restful.Response, temp []Item) {
	var (
		count int
		items = make([]data.Item, 0, len(temp))
		// popularScore = lo.Map(temp, func(item Item, i int) float64 {
		// 	return s.PopularItemsCache.GetSortedScore(item.ItemId)
		// })

		loadExistedItemsTime time.Duration
		parseTimesatmpTime   time.Duration
		insertItemsTime      time.Duration
		insertCacheTime      time.Duration
	)
	// load existed items
	start := time.Now()
	existedItems, err := s.DataClient.BatchGetItems(ctx, lo.Map(temp, func(t Item, i int) string {
		return t.ItemId
	}))
	if err != nil {
		InternalServerError(response, err)
		return
	}
	existedItemsSet := make(map[string]data.Item)
	for _, item := range existedItems {
		existedItemsSet[item.ItemId] = item
	}
	loadExistedItemsTime = time.Since(start)

	start = time.Now()
	for _, item := range temp {
		// parse datetime
		var timestamp time.Time
		var err error
		if item.Timestamp != "" {
			if timestamp, err = dateparse.ParseAny(item.Timestamp); err != nil {
				BadRequest(response, err)
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
		// insert to latest items cache
		if err = s.CacheClient.AddScores(ctx, cache.LatestItems, "", []cache.Score{{
			Id:         item.ItemId,
			Score:      float64(timestamp.Unix()),
			Categories: withWildCard(item.Categories),
			Timestamp:  time.Now(),
		}}); err != nil {
			InternalServerError(response, err)
			return
		}
		// update items cache
		if err = s.CacheClient.UpdateScores(ctx, cache.ItemCache, item.ItemId, cache.ScorePatch{
			Categories: withWildCard(item.Categories),
			IsHidden:   &item.IsHidden,
		}); err != nil {
			InternalServerError(response, err)
			return
		}
		count++
	}
	parseTimesatmpTime = time.Since(start)

	// insert items
	start = time.Now()
	if err = s.DataClient.BatchInsertItems(ctx, items); err != nil {
		InternalServerError(response, err)
		return
	}
	insertItemsTime = time.Since(start)

	// insert modify timestamp
	start = time.Now()
	categories := mapset.NewSet[string]()
	values := make([]cache.Value, len(items))
	for i, item := range items {
		values[i] = cache.Time(cache.Key(cache.LastModifyItemTime, item.ItemId), time.Now())
		categories.Append(item.Categories...)
	}
	if err = s.CacheClient.Set(ctx, values...); err != nil {
		InternalServerError(response, err)
		return
	}
	// insert categories
	if err = s.CacheClient.AddSet(ctx, cache.ItemCategories, categories.ToSlice()...); err != nil {
		InternalServerError(response, err)
		return
	}

	insertCacheTime = time.Since(start)
	log.ResponseLogger(response).Info("batch insert items",
		zap.Duration("load_existed_items_time", loadExistedItemsTime),
		zap.Duration("parse_timestamp_time", parseTimesatmpTime),
		zap.Duration("insert_items_time", insertItemsTime),
		zap.Duration("insert_cache_time", insertCacheTime))
	Ok(response, Success{RowAffected: count})
}

func (s *RestServer) insertItems(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	var items []Item
	if err := request.ReadEntity(&items); err != nil {
		BadRequest(response, err)
		return
	}
	// validate labels
	for _, user := range items {
		if err := data.ValidateLabels(user.Labels); err != nil {
			BadRequest(response, err)
			return
		}
	}
	// Insert items
	s.batchInsertItems(ctx, response, items)
}

func (s *RestServer) insertItem(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	var item Item
	var err error
	if err = request.ReadEntity(&item); err != nil {
		BadRequest(response, err)
		return
	}
	// validate labels
	if err := data.ValidateLabels(item.Labels); err != nil {
		BadRequest(response, err)
		return
	}
	s.batchInsertItems(ctx, response, []Item{item})
}

func (s *RestServer) modifyItem(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	itemId := request.PathParameter("item-id")
	var patch data.ItemPatch
	if err := request.ReadEntity(&patch); err != nil {
		BadRequest(response, err)
		return
	}
	// validate labels
	if err := data.ValidateLabels(patch.Labels); err != nil {
		BadRequest(response, err)
		return
	}
	// remove hidden item from cache
	if patch.IsHidden != nil {
		if err := s.CacheClient.UpdateScores(ctx, cache.ItemCache, itemId, cache.ScorePatch{IsHidden: patch.IsHidden}); err != nil {
			InternalServerError(response, err)
			return
		}
	}
	// add item to latest items cache
	if patch.Timestamp != nil {
		if err := s.CacheClient.UpdateScores(ctx, []string{cache.LatestItems}, itemId, cache.ScorePatch{Score: proto.Float64(float64(patch.Timestamp.Unix()))}); err != nil {
			InternalServerError(response, err)
			return
		}
	}
	// update categories in cache
	if patch.Categories != nil {
		if err := s.CacheClient.UpdateScores(ctx, cache.ItemCache, itemId, cache.ScorePatch{Categories: withWildCard(patch.Categories)}); err != nil {
			InternalServerError(response, err)
			return
		}
	}
	// modify item
	if err := s.DataClient.ModifyItem(ctx, itemId, patch); err != nil {
		InternalServerError(response, err)
		return
	}
	// insert modify timestamp
	if err := s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, itemId), time.Now())); err != nil {
		return
	}
	Ok(response, Success{RowAffected: 1})
}

// ItemIterator is the iterator for items.
type ItemIterator struct {
	Cursor string
	Items  []data.Item
}

func (s *RestServer) getItems(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	cursor := request.QueryParameter("cursor")
	n, err := ParseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	cursor, items, err := s.DataClient.GetItems(ctx, cursor, n, nil)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, ItemIterator{Cursor: cursor, Items: items})
}

func (s *RestServer) getItem(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// Get item id
	itemId := request.PathParameter("item-id")
	// Get item
	item, err := s.DataClient.GetItem(ctx, itemId)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			PageNotFound(response, err)
		} else {
			InternalServerError(response, err)
		}
		return
	}
	Ok(response, item)
}

func (s *RestServer) deleteItem(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	itemId := request.PathParameter("item-id")
	// delete item from database
	if err := s.DataClient.DeleteItem(ctx, itemId); err != nil {
		InternalServerError(response, err)
		return
	}
	// delete item from cache
	if err := s.CacheClient.DeleteScores(ctx, cache.ItemCache, cache.ScoreCondition{Id: &itemId}); err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, Success{RowAffected: 1})
}

func (s *RestServer) insertItemCategory(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// fetch item id and category
	itemId := request.PathParameter("item-id")
	category := request.PathParameter("category")
	// fetch item
	item, err := s.DataClient.GetItem(ctx, itemId)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	if !funk.ContainsString(item.Categories, category) {
		item.Categories = append(item.Categories, category)
	}
	// insert category to database
	if err = s.DataClient.BatchInsertItems(ctx, []data.Item{item}); err != nil {
		InternalServerError(response, err)
		return
	}
	// insert category to cache
	if err = s.CacheClient.UpdateScores(ctx, cache.ItemCache, itemId, cache.ScorePatch{Categories: withWildCard(item.Categories)}); err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, Success{RowAffected: 1})
}

func (s *RestServer) deleteItemCategory(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// fetch item id and category
	itemId := request.PathParameter("item-id")
	category := request.PathParameter("category")
	// fetch item
	item, err := s.DataClient.GetItem(ctx, itemId)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	categories := make([]string, 0, len(item.Categories))
	for _, cat := range item.Categories {
		if cat != category {
			categories = append(categories, cat)
		}
	}
	item.Categories = categories
	// delete category from cache
	if err = s.CacheClient.UpdateScores(ctx, cache.ItemCache, itemId, cache.ScorePatch{Categories: withWildCard(categories)}); err != nil {
		InternalServerError(response, err)
		return
	}
	// delete category from database
	if err = s.DataClient.BatchInsertItems(ctx, []data.Item{item}); err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, Success{RowAffected: 1})
}

// Feedback is the data structure for the feedback but stores the timestamp using string.
type Feedback struct {
	data.FeedbackKey
	Timestamp string
	Comment   string
}

func (f Feedback) ToDataFeedback() (data.Feedback, error) {
	var feedback data.Feedback
	feedback.FeedbackKey = f.FeedbackKey
	feedback.Comment = f.Comment
	if f.Timestamp != "" {
		var err error
		feedback.Timestamp, err = dateparse.ParseAny(f.Timestamp)
		if err != nil {
			return data.Feedback{}, err
		}
	}
	return feedback, nil
}

func (s *RestServer) insertFeedback(overwrite bool) func(request *restful.Request, response *restful.Response) {
	return func(request *restful.Request, response *restful.Response) {
		ctx := context.Background()
		if request != nil && request.Request != nil {
			ctx = request.Request.Context()
		}
		// add ratings
		var feedbackLiterTime []Feedback
		if err := request.ReadEntity(&feedbackLiterTime); err != nil {
			BadRequest(response, err)
			return
		}
		// parse datetime
		var err error
		feedback := make([]data.Feedback, len(feedbackLiterTime))
		users := mapset.NewSet[string]()
		items := mapset.NewSet[string]()
		for i := range feedback {
			users.Add(feedbackLiterTime[i].UserId)
			items.Add(feedbackLiterTime[i].ItemId)
			feedback[i], err = feedbackLiterTime[i].ToDataFeedback()
			if err != nil {
				BadRequest(response, err)
				return
			}
		}
		// insert feedback to data store
		err = s.DataClient.BatchInsertFeedback(ctx, feedback,
			s.Config.Server.AutoInsertUser,
			s.Config.Server.AutoInsertItem, overwrite)
		if err != nil {
			InternalServerError(response, err)
			return
		}
		values := make([]cache.Value, 0, users.Cardinality()+items.Cardinality())
		for _, userId := range users.ToSlice() {
			values = append(values, cache.Time(cache.Key(cache.LastModifyUserTime, userId), time.Now()))
		}
		for _, itemId := range items.ToSlice() {
			values = append(values, cache.Time(cache.Key(cache.LastModifyItemTime, itemId), time.Now()))
		}
		if err = s.CacheClient.Set(ctx, values...); err != nil {
			InternalServerError(response, err)
			return
		}
		log.ResponseLogger(response).Info("Insert feedback successfully", zap.Int("num_feedback", len(feedback)))
		Ok(response, Success{RowAffected: len(feedback)})
	}
}

// FeedbackIterator is the iterator for feedback.
type FeedbackIterator struct {
	Cursor   string
	Feedback []data.Feedback
}

func (s *RestServer) getFeedback(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// Parse parameters
	cursor := request.QueryParameter("cursor")
	n, err := ParseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	cursor, feedback, err := s.DataClient.GetFeedback(ctx, cursor, n, nil, s.Config.Now())
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, FeedbackIterator{Cursor: cursor, Feedback: feedback})
}

func (s *RestServer) getTypedFeedback(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// Parse parameters
	feedbackType := request.PathParameter("feedback-type")
	cursor := request.QueryParameter("cursor")
	n, err := ParseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	cursor, feedback, err := s.DataClient.GetFeedback(ctx, cursor, n, nil, s.Config.Now(), feedbackType)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, FeedbackIterator{Cursor: cursor, Feedback: feedback})
}

func (s *RestServer) getUserItemFeedback(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// Parse parameters
	userId := request.PathParameter("user-id")
	itemId := request.PathParameter("item-id")
	if feedback, err := s.DataClient.GetUserItemFeedback(ctx, userId, itemId); err != nil {
		InternalServerError(response, err)
	} else {
		Ok(response, feedback)
	}
}

func (s *RestServer) deleteUserItemFeedback(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// Parse parameters
	userId := request.PathParameter("user-id")
	itemId := request.PathParameter("item-id")
	if deleteCount, err := s.DataClient.DeleteUserItemFeedback(ctx, userId, itemId); err != nil {
		InternalServerError(response, err)
	} else {
		Ok(response, Success{RowAffected: deleteCount})
	}
}

func (s *RestServer) getTypedUserItemFeedback(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// Parse parameters
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	itemId := request.PathParameter("item-id")
	if feedback, err := s.DataClient.GetUserItemFeedback(ctx, userId, itemId, feedbackType); err != nil {
		InternalServerError(response, err)
	} else if feedbackType == "" {
		Text(response, "{}")
	} else {
		Ok(response, feedback[0])
	}
}

func (s *RestServer) deleteTypedUserItemFeedback(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// Parse parameters
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	itemId := request.PathParameter("item-id")
	if deleteCount, err := s.DataClient.DeleteUserItemFeedback(ctx, userId, itemId, feedbackType); err != nil {
		InternalServerError(response, err)
	} else {
		Ok(response, Success{deleteCount})
	}
}

type HealthStatus struct {
	Ready               bool
	DataStoreError      error
	CacheStoreError     error
	DataStoreConnected  bool
	CacheStoreConnected bool
}

func (s *RestServer) checkHealth() HealthStatus {
	healthStatus := HealthStatus{}
	healthStatus.DataStoreError = s.DataClient.Ping()
	healthStatus.CacheStoreError = s.CacheClient.Ping()
	healthStatus.DataStoreConnected = healthStatus.DataStoreError == nil
	healthStatus.CacheStoreConnected = healthStatus.CacheStoreError == nil
	healthStatus.Ready = healthStatus.DataStoreConnected && healthStatus.CacheStoreConnected
	return healthStatus
}

func (s *RestServer) checkReady(_ *restful.Request, response *restful.Response) {
	healthStatus := s.checkHealth()
	if healthStatus.Ready {
		Ok(response, healthStatus)
	} else {
		errReason, err := json.Marshal(healthStatus)
		if err != nil {
			Error(response, http.StatusInternalServerError, err)
		} else {
			Error(response, http.StatusServiceUnavailable, errors.New(string(errReason)))
		}
	}
}

func (s *RestServer) checkLive(_ *restful.Request, response *restful.Response) {
	healthStatus := s.checkHealth()
	Ok(response, healthStatus)
}

func (s *RestServer) getMeasurements(request *restful.Request, response *restful.Response) {
	ctx := context.Background()
	if request != nil && request.Request != nil {
		ctx = request.Request.Context()
	}
	// Parse parameters
	name := request.PathParameter("name")
	n, err := ParseInt(request, "n", 100)
	if err != nil {
		BadRequest(response, err)
		return
	}
	measurements, err := s.CacheClient.GetTimeSeriesPoints(ctx, name, time.Now().Add(-24*time.Hour*time.Duration(n)), time.Now())
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, measurements)
}

// BadRequest returns a bad request error.
func BadRequest(response *restful.Response, err error) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	log.ResponseLogger(response).Error("bad request", zap.Error(err))
	if err = response.WriteError(http.StatusBadRequest, err); err != nil {
		log.ResponseLogger(response).Error("failed to write error", zap.Error(err))
	}
}

// InternalServerError returns a internal server error.
func InternalServerError(response *restful.Response, err error) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	log.ResponseLogger(response).Error("internal server error", zap.Error(err))
	if err = response.WriteError(http.StatusInternalServerError, err); err != nil {
		log.ResponseLogger(response).Error("failed to write error", zap.Error(err))
	}
}

// PageNotFound returns a not found error.
func PageNotFound(response *restful.Response, err error) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	if err := response.WriteError(http.StatusNotFound, err); err != nil {
		log.ResponseLogger(response).Error("failed to write error", zap.Error(err))
	}
}

// Ok sends the content as JSON to the client.
func Ok(response *restful.Response, content interface{}) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	if err := response.WriteAsJson(content); err != nil {
		log.ResponseLogger(response).Error("failed to write json", zap.Error(err))
	}
}

func Error(response *restful.Response, httpStatus int, responseError error) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	if err := response.WriteError(httpStatus, responseError); err != nil {
		log.ResponseLogger(response).Error("failed to write error", zap.Error(err))
	}
}

// Text returns a plain text.
func Text(response *restful.Response, content string) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	if _, err := response.Write([]byte(content)); err != nil {
		log.ResponseLogger(response).Error("failed to write text", zap.Error(err))
	}
}

func withWildCard(categories []string) []string {
	result := make([]string, len(categories), len(categories)+1)
	copy(result, categories)
	result = append(result, "")
	return result
}
