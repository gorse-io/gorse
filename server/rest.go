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
	"fmt"
	"github.com/juju/errors"
	"net/http"
	"strconv"
	"time"

	"github.com/araddon/dateparse"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/scylladb/go-set"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
)

// RestServer implements a REST-ful API server.
type RestServer struct {
	CacheClient cache.Database
	DataClient  data.Database
	GorseConfig *config.Config
	HttpHost    string
	HttpPort    int
	EnableAuth  bool
	WebService  *restful.WebService
}

// StartHttpServer starts the REST-ful API server.
func (s *RestServer) StartHttpServer() {
	// register restful APIs
	s.CreateWebService()
	restful.DefaultContainer.Add(s.WebService)
	// register swagger UI
	specConfig := restfulspec.Config{
		WebServices: restful.RegisteredWebServices(),
		APIPath:     "/apidocs.json",
	}
	restful.DefaultContainer.Add(restfulspec.NewOpenAPIService(specConfig))
	swaggerFile = specConfig.APIPath
	http.HandleFunc(apiDocsPath, handler)
	// register prometheus
	http.Handle("/metrics", promhttp.Handler())

	base.Logger().Info("start http server",
		zap.String("url", fmt.Sprintf("http://%s:%d", s.HttpHost, s.HttpPort)))
	base.Logger().Fatal("failed to start http server",
		zap.Error(http.ListenAndServe(fmt.Sprintf("%s:%d", s.HttpHost, s.HttpPort), nil)))
}

func LogFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	chain.ProcessFilter(req, resp)
	if req.Request.URL.Path != "/api/dashboard/cluster" &&
		req.Request.URL.Path != "/api/dashboard/tasks" {
		base.Logger().Info(fmt.Sprintf("%s %s", req.Request.Method, req.Request.URL),
			zap.Int("status_code", resp.StatusCode()))
	}
}

// CreateWebService creates web service.
func (s *RestServer) CreateWebService() {
	// Create a server
	ws := s.WebService
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	ws.Path("/api/")
	ws.Filter(LogFilter)

	/* Interactions with data store */

	// Insert a user
	ws.Route(ws.POST("/user").To(s.insertUser).
		Doc("Insert a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"user"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Returns(200, "OK", Success{}).
		Reads(data.User{}))
	// Get a user
	ws.Route(ws.GET("/user/{user-id}").To(s.getUser).
		Doc("Get a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"user"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Returns(200, "OK", data.User{}).
		Writes(data.User{}))
	// Insert users
	ws.Route(ws.POST("/users").To(s.insertUsers).
		Doc("Insert users.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"user"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Returns(200, "OK", Success{}).
		Reads([]data.User{}))
	// Get users
	ws.Route(ws.GET("/users").To(s.getUsers).
		Doc("Get users.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"user"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned users").DataType("int")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Returns(200, "OK", UserIterator{}).
		Writes(UserIterator{}))
	// Delete a user
	ws.Route(ws.DELETE("/user/{user-id}").To(s.deleteUser).
		Doc("Delete a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"user"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Returns(200, "OK", Success{}).
		Writes(Success{}))

	// Insert an item
	ws.Route(ws.POST("/item").To(s.insertItem).
		Doc("Insert an item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"item"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Returns(200, "OK", Success{}).
		Reads(data.Item{}))
	// Get items
	ws.Route(ws.GET("/items").To(s.getItems).
		Doc("Get items.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"item"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Returns(200, "OK", ItemIterator{}).
		Writes(ItemIterator{}))
	// Get item
	ws.Route(ws.GET("/item/{item-id}").To(s.getItem).
		Doc("Get a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"item"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")).
		Returns(200, "OK", data.Item{}).
		Writes(data.Item{}))
	// Insert items
	ws.Route(ws.POST("/items").To(s.insertItems).
		Doc("Insert items.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"item"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Reads([]data.Item{}))
	// Delete item
	ws.Route(ws.DELETE("/item/{item-id}").To(s.deleteItem).
		Doc("Delete a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"item"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("item-id", "identified of the item").DataType("string")).
		Returns(200, "OK", Success{}).
		Writes(Success{}))

	// Insert feedback
	ws.Route(ws.POST("/feedback").To(s.insertFeedback).
		Doc("Insert multiple feedback.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Reads([]data.Feedback{}).
		Returns(200, "OK", Success{}))
	// Get feedback
	ws.Route(ws.GET("/feedback").To(s.getFeedback).
		Doc("Get multiple feedback.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned feedback").DataType("integer")).
		Returns(200, "OK", FeedbackIterator{}).
		Writes(FeedbackIterator{}))
	ws.Route(ws.GET("/feedback/{user-id}/{item-id}").To(s.getUserItemFeedback).
		Doc("Get feedback between a user and a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Returns(200, "OK", []data.Feedback{}).
		Writes([]data.Feedback{}))
	ws.Route(ws.DELETE("/feedback/{user-id}/{item-id}").To(s.deleteUserItemFeedback).
		Doc("Delete feedback between a user and a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Returns(200, "OK", []data.Feedback{}).
		Writes([]data.Feedback{}))
	ws.Route(ws.GET("/feedback/{feedback-type}").To(s.getTypedFeedback).
		Doc("Get multiple feedback with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned feedback").DataType("integer")).
		Returns(200, "OK", FeedbackIterator{}).
		Writes(FeedbackIterator{}))
	ws.Route(ws.GET("/feedback/{feedback-type}/{user-id}/{item-id}").To(s.getTypedUserItemFeedback).
		Doc("Get feedback between a user and a item with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Returns(200, "OK", data.Feedback{}).
		Writes(data.Feedback{}))
	ws.Route(ws.DELETE("/feedback/{feedback-type}/{user-id}/{item-id}").To(s.deleteTypedUserItemFeedback).
		Doc("Delete feedback between a user and a item with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Returns(200, "OK", data.Feedback{}).
		Writes(data.Feedback{}))
	// Get feedback by user id
	ws.Route(ws.GET("/user/{user-id}/feedback/{feedback-type}").To(s.getTypedFeedbackByUser).
		Doc("Get feedback by user id with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Returns(200, "OK", []data.Feedback{}).
		Writes([]data.Feedback{}))
	ws.Route(ws.GET("/user/{user-id}/feedback/").To(s.getFeedbackByUser).
		Doc("Get feedback by user id.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Returns(200, "OK", []data.Feedback{}).
		Writes([]data.Feedback{}))
	// Get feedback by item-id
	ws.Route(ws.GET("/item/{item-id}/feedback/{feedback-type}").To(s.getTypedFeedbackByItem).
		Doc("Get feedback by item id with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("strung")).
		Returns(200, "OK", []string{}).
		Writes([]string{}))
	ws.Route(ws.GET("/item/{item-id}/feedback/").To(s.getFeedbackByItem).
		Doc("Get feedback by item id.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Returns(200, "OK", []string{}).
		Writes([]string{}))

	/* Interaction with intermediate result */

	// Get collaborative filtering recommendation by user id
	ws.Route(ws.GET("/intermediate/recommend/{user-id}").To(s.getCollaborative).
		Doc("get the collaborative filtering recommendation for a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{"intermediate"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(200, "OK", []string{}).
		Writes([]string{}))

	/* Rank recommendation */

	// Get popular items
	ws.Route(ws.GET("/popular").To(s.getPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(200, "OK", []string{}).
		Writes([]string{}))
	// Disable popular items under labels temporarily
	//ws.Route(ws.GET("/popular/{label}").To(s.getLabelPopular).
	//	Doc("get popular items").
	//	Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
	//	Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
	//	Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
	//	Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
	//	Writes([]string{}))
	// Get latest items
	ws.Route(ws.GET("/latest").To(s.getLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(200, "OK", []cache.Scored{}).
		Writes([]cache.Scored{}))
	// Disable the latest items under labels temporarily
	//ws.Route(ws.GET("/latest/{label}").To(s.getLabelLatest).
	//	Doc("get latest items").
	//	Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
	//	Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
	//	Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
	//	Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
	//	Writes([]string{}))
	// Get neighbors
	ws.Route(ws.GET("/item/{item-id}/neighbors/").To(s.getItemNeighbors).
		Doc("get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(200, "OK", []string{}).
		Writes([]string{}))
	ws.Route(ws.GET("/user/{user-id}/neighbors/").To(s.getUserNeighbors).
		Doc("get neighbors of a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned users").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Returns(200, "OK", []string{}).
		Writes([]string{}))
	ws.Route(ws.GET("/recommend/{user-id}").To(s.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("write-back", "write recommendation back to feedback").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Returns(200, "OK", []string{}).
		Writes([]string{}))

	/* Interaction with measurements */

	ws.Route(ws.GET("/measurements/{name}").To(s.getMeasurements).
		Doc("Get measurements").
		Metadata(restfulspec.KeyOpenAPITags, []string{"measurements"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Returns(200, "OK", []data.Measurement{}).
		Writes([]data.Measurement{}))
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

func (s *RestServer) getList(prefix, name string, request *restful.Request, response *restful.Response) {
	var n, begin, end int
	var err error
	// read arguments
	if begin, err = ParseInt(request, "offset", 0); err != nil {
		BadRequest(response, err)
		return
	}
	if n, err = ParseInt(request, "n", s.GorseConfig.Server.DefaultN); err != nil {
		BadRequest(response, err)
		return
	}
	end = begin + n - 1
	// Get the popular list
	items, err := s.CacheClient.GetScores(prefix, name, begin, end)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	// Send result
	Ok(response, items)
}

// getPopular gets popular items from database.
func (s *RestServer) getPopular(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	base.Logger().Debug("get popular items")
	s.getList(cache.PopularItems, "", request, response)
}

func (s *RestServer) getLatest(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	base.Logger().Debug("get latest items")
	s.getList(cache.LatestItems, "", request, response)
}

//func (s *RestServer) getLabelPopular(request *restful.Request, response *restful.Response) {
//	// Authorize
//	if !s.auth(request, response) {
//		return
//	}
//	label := request.PathParameter("label")
//	base.Logger().Debug("get label popular items", zap.String("label", label))
//	s.getList(cache.PopularItems, label, request, response)
//}
//
//func (s *RestServer) getLabelLatest(request *restful.Request, response *restful.Response) {
//	// Authorize
//	if !s.auth(request, response) {
//		return
//	}
//	label := request.PathParameter("label")
//	base.Logger().Debug("get label latest items", zap.String("label", label))
//	s.getList(cache.LatestItems, label, request, response)
//}

// get feedback by item-id with feedback type
func (s *RestServer) getTypedFeedbackByItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	feedbackType := request.PathParameter("feedback-type")
	itemId := request.PathParameter("item-id")
	feedback, err := s.DataClient.GetItemFeedback(itemId, feedbackType)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, feedback)
}

// get feedback by item-id
func (s *RestServer) getFeedbackByItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	itemId := request.PathParameter("item-id")
	feedback, err := s.DataClient.GetItemFeedback(itemId)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, feedback)
}

// getItemNeighbors gets neighbors of a item from database.
func (s *RestServer) getItemNeighbors(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get item id
	itemId := request.PathParameter("item-id")
	s.getList(cache.ItemNeighbors, itemId, request, response)
}

// getUserNeighbors gets neighbors of a user from database.
func (s *RestServer) getUserNeighbors(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get item id
	userId := request.PathParameter("user-id")
	s.getList(cache.UserNeighbors, userId, request, response)
}

// getSubscribe gets subscribed items of a user from database.
//func (s *RestServer) getSubscribe(request *restful.Request, response *restful.Response) {
//	// Authorize
//	if !s.auth(request, response) {
//		return
//	}
//	// Get user id
//	userId := request.PathParameter("user-id")
//	s.getList(cache.SubscribeItems, userId, request, response)
//}

// getCollaborative gets cached recommended items from database.
func (s *RestServer) getCollaborative(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get user id
	userId := request.PathParameter("user-id")
	s.getList(cache.CTRRecommend, userId, request, response)
}

type Recommender byte

const (
	CTRRecommender           Recommender = 0x1
	CollaborativeRecommender Recommender = 0x2
	UserBasedRecommender     Recommender = 0x4
	ItemBasedRecommender     Recommender = 0x8
	LatestRecommender        Recommender = 0x10
	PopularRecommender       Recommender = 0x20
)

// Recommend items to users.
// 1. If there are recommendations in cache, return cached recommendations.
// 2. If there are historical interactions of the users, return similar items.
// 3. Otherwise, return fallback recommendation (popular/latest).
func (s *RestServer) Recommend(userId string, n int, recommenders ...Recommender) ([]string, error) {
	var (
		err              error
		loadFinalRecTime time.Duration
		loadColRecTime   time.Duration
		loadLoadHistTime time.Duration
		itemBasedTime    time.Duration
		userBasedTime    time.Duration
		loadLatestTime   time.Duration
		loadPopularTime  time.Duration
		numPrevStage     int
		results          []string
		excludeSet       = set.NewStringSet()
	)

	// create recommender mask
	var recommenderMask Recommender
	if len(recommenders) == 0 {
		recommenderMask |= CTRRecommender | ItemBasedRecommender | PopularRecommender
	} else {
		for _, recommender := range recommenders {
			recommenderMask |= recommender
		}
	}

	initStart := time.Now()

	if recommenderMask&CTRRecommender > 0 {
		// read recommendations in cache.
		itemsChan := make(chan []string, 1)
		errChan := make(chan error, 1)
		go func() {
			var collaborativeFilteringItems []cache.Scored
			collaborativeFilteringItems, err = s.CacheClient.GetScores(cache.CTRRecommend, userId, 0, s.GorseConfig.Database.CacheSize)
			if err != nil {
				itemsChan <- nil
				errChan <- err
			} else {
				itemsChan <- cache.RemoveScores(collaborativeFilteringItems)
				errChan <- nil
				if len(collaborativeFilteringItems) == 0 {
					base.Logger().Warn("empty collaborative filtering", zap.String("user_id", userId))
				}
			}
		}()

		// read ignore items
		loadCachedReadStart := time.Now()
		ignoreItems, err := s.CacheClient.GetList(cache.IgnoreItems, userId)
		if err != nil {
			return nil, errors.Trace(err)
		}
		for _, item := range ignoreItems {
			excludeSet.Add(item)
		}
		loadFinalRecTime = time.Since(loadCachedReadStart)
		// remove ignore items
		items := <-itemsChan
		err = <-errChan
		if err != nil {
			return nil, err
		}
		results = make([]string, 0, len(items))
		for _, itemId := range items {
			if !excludeSet.Has(itemId) {
				results = append(results, itemId)
				excludeSet.Add(itemId)
			}
		}
	}
	numFromFinal := len(results) - numPrevStage
	numPrevStage = len(results)

	// Load collaborative recommendation.  The running condition is:
	// 1. The number of current results is less than n.
	// 2. There are recommenders other than CollaborativeRecommender.
	if len(results) < n && recommenderMask&CollaborativeRecommender > 0 {
		start := time.Now()
		collaborativeRecommendation, err := s.CacheClient.GetScores(cache.CollaborativeRecommend, userId, 0, s.GorseConfig.Database.CacheSize)
		if err != nil {
			return nil, err
		}
		for _, item := range collaborativeRecommendation {
			if !excludeSet.Has(item.Id) {
				results = append(results, item.Id)
				excludeSet.Add(item.Id)
			}
		}
		loadColRecTime = time.Since(start)
	}
	numFromCollaborative := len(results) - numPrevStage
	numPrevStage = len(results)

	// Load historical feedback. The running condition is:
	// 1. The number of current results is less than n.
	// 2. There are recommenders other than CTRRecommender.
	var userFeedback []data.Feedback
	if len(results) < n && recommenderMask-CTRRecommender > 0 {
		start := time.Now()
		userFeedback, err = s.DataClient.GetUserFeedback(userId)
		if err != nil {
			return nil, err
		}
		for _, feedback := range userFeedback {
			excludeSet.Add(feedback.ItemId)
		}
		loadLoadHistTime = time.Since(start)
	}

	// Item-based recommendation. The running condition is:
	// 1. The number of current results is less than n.
	// 2. Recommenders include CTRRecommender.
	if len(results) < n && recommenderMask&ItemBasedRecommender > 0 {
		start := time.Now()
		// collect candidates
		candidates := make(map[string]float32)
		for _, feedback := range userFeedback {
			// load similar items
			similarItems, err := s.CacheClient.GetScores(cache.ItemNeighbors, feedback.ItemId, 0, s.GorseConfig.Database.CacheSize)
			if err != nil {
				return nil, err
			}
			// add unseen items
			for _, item := range similarItems {
				if !excludeSet.Has(item.Id) {
					candidates[item.Id] += item.Score
				}
			}
		}
		// collect top k
		k := n - len(results)
		filter := base.NewTopKStringFilter(k)
		for id, score := range candidates {
			filter.Push(id, score)
		}
		ids, _ := filter.PopAll()
		results = append(results, ids...)
		excludeSet.Add(ids...)
		itemBasedTime = time.Since(start)
	}
	numFromItemBased := len(results) - numPrevStage
	numPrevStage = len(results)

	// User-based recommendation. The running condition is:
	// 1. The number of current results is less than n.
	// 2. Recommenders include UserBasedRecommender.
	if len(results) < n && recommenderMask&UserBasedRecommender > 0 {
		start := time.Now()
		candidates := make(map[string]float32)
		// load similar users
		similarUsers, err := s.CacheClient.GetScores(cache.UserNeighbors, userId, 0, s.GorseConfig.Database.CacheSize)
		if err != nil {
			return nil, err
		}
		for _, user := range similarUsers {
			// load historical feedback
			feedbacks, err := s.DataClient.GetUserFeedback(user.Id, s.GorseConfig.Database.PositiveFeedbackType...)
			if err != nil {
				return nil, err
			}
			// add unseen items
			for _, feedback := range feedbacks {
				if !excludeSet.Has(feedback.ItemId) {
					candidates[feedback.ItemId] += user.Score
				}
			}
		}
		// collect top k
		k := n - len(results)
		filter := base.NewTopKStringFilter(k)
		for id, score := range candidates {
			filter.Push(id, score)
		}
		ids, _ := filter.PopAll()
		results = append(results, ids...)
		excludeSet.Add(ids...)
		userBasedTime = time.Since(start)
	}
	numFromUserBased := len(results) - numPrevStage
	numPrevStage = len(results)

	// Latest recommendation. The running condition is:
	// 1. The number of current results is less than n.
	// 2. Recommenders include LatestRecommender.
	if len(results) < n && recommenderMask&LatestRecommender > 0 {
		start := time.Now()
		items, err := s.CacheClient.GetScores(cache.LatestItems, "", 0, n-len(results))
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !excludeSet.Has(item.Id) {
				results = append(results, item.Id)
				excludeSet.Add(item.Id)
			}
		}
		loadLatestTime = time.Since(start)
	}
	numFromLatest := len(results) - numPrevStage
	numPrevStage = len(results)

	// Popular recommendation. The running condition is:
	// 1. The number of current results is less than n.
	// 2. Recommenders include PopularRecommender.
	if len(results) < n && recommenderMask&PopularRecommender > 0 {
		start := time.Now()
		items, err := s.CacheClient.GetScores(cache.PopularItems, "", 0, n-len(results))
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !excludeSet.Has(item.Id) {
				results = append(results, item.Id)
				excludeSet.Add(item.Id)
			}
		}
		loadLatestTime = time.Since(start)
	}
	numFromPopular := len(results) - numPrevStage

	// return recommendations
	if len(results) > n {
		results = results[:n]
	}
	totalTime := time.Since(initStart)
	base.Logger().Info("complete recommendation",
		zap.Int("num_from_final", numFromFinal),
		zap.Int("num_from_collaborative", numFromCollaborative),
		zap.Int("num_from_item_based", numFromItemBased),
		zap.Int("num_from_user_based", numFromUserBased),
		zap.Int("num_from_latest", numFromLatest),
		zap.Int("num_from_poplar", numFromPopular),
		zap.Duration("total_time", totalTime),
		zap.Duration("load_final_recommend_time", loadFinalRecTime),
		zap.Duration("load_col_recommend_time", loadColRecTime),
		zap.Duration("load_hist_time", loadLoadHistTime),
		zap.Duration("item_based_recommend_time", itemBasedTime),
		zap.Duration("user_based_recommend_time", userBasedTime),
		zap.Duration("load_latest_time", loadLatestTime),
		zap.Duration("load_popular_time", loadPopularTime))
	return results, nil
}

func (s *RestServer) getRecommend(request *restful.Request, response *restful.Response) {
	// authorize
	if !s.auth(request, response) {
		return
	}
	// parse arguments
	userId := request.PathParameter("user-id")
	n, err := ParseInt(request, "n", s.GorseConfig.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	writeBackFeedback := request.QueryParameter("write-back")
	// online recommendation
	recommenders := []Recommender{CTRRecommender}
	for _, recommender := range s.GorseConfig.Recommend.FallbackRecommend {
		switch recommender {
		case "collaborative":
			recommenders = append(recommenders, CollaborativeRecommender)
		case "user_based":
			recommenders = append(recommenders, UserBasedRecommender)
		case "item_based":
			recommenders = append(recommenders, ItemBasedRecommender)
		case "latest":
			recommenders = append(recommenders, LatestRecommender)
		case "popular":
			recommenders = append(recommenders, PopularRecommender)
		default:
			InternalServerError(response, fmt.Errorf("unknown fallback recommendation method `%s`", recommender))
			return
		}
	}
	results, err := s.Recommend(userId, n, recommenders...)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	// write back
	if writeBackFeedback != "" {
		for _, itemId := range results {
			// insert to data store
			feedback := data.Feedback{
				FeedbackKey: data.FeedbackKey{
					UserId:       userId,
					ItemId:       itemId,
					FeedbackType: writeBackFeedback,
				},
				Timestamp: time.Now(),
			}
			err = s.DataClient.BatchInsertFeedback([]data.Feedback{feedback}, false, false)
			if err != nil {
				InternalServerError(response, err)
				return
			}
			// insert to cache store
			err = s.InsertFeedbackToCache([]data.Feedback{feedback})
			if err != nil {
				InternalServerError(response, err)
				return
			}
		}
	}
	// Send result
	Ok(response, results)
}

// Success is the returned data structure for data insert operations.
type Success struct {
	RowAffected int
}

func (s *RestServer) insertUser(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	temp := data.User{}
	// get userInfo from request and put into temp
	if err := request.ReadEntity(&temp); err != nil {
		BadRequest(response, err)
		return
	}
	if err := s.DataClient.BatchInsertUsers([]data.User{temp}); err != nil {
		InternalServerError(response, err)
		return
	}
	// insert modify timestamp
	if err := s.CacheClient.SetTime(cache.LastModifyUserTime, temp.UserId, time.Now()); err != nil {
		return
	}
	Ok(response, Success{RowAffected: 1})
}

func (s *RestServer) getUser(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// get user id
	userId := request.PathParameter("user-id")
	// get user
	user, err := s.DataClient.GetUser(userId)
	if err != nil {
		if err == data.ErrUserNotExist {
			PageNotFound(response, err)
		} else {
			InternalServerError(response, err)
		}
		return
	}
	Ok(response, user)
}

func (s *RestServer) insertUsers(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	temp := new([]data.User)
	// get param from request and put into temp
	if err := request.ReadEntity(temp); err != nil {
		BadRequest(response, err)
		return
	}
	var count int
	// range temp and achieve user
	if err := s.DataClient.BatchInsertUsers(*temp); err != nil {
		InternalServerError(response, err)
		return
	}
	for _, user := range *temp {
		// insert modify timestamp
		if err := s.CacheClient.SetTime(cache.LastModifyUserTime, user.UserId, time.Now()); err != nil {
			return
		}
		count++
	}
	Ok(response, Success{RowAffected: count})
}

type UserIterator struct {
	Cursor string
	Users  []data.User
}

func (s *RestServer) getUsers(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	cursor := request.QueryParameter("cursor")
	n, err := ParseInt(request, "n", s.GorseConfig.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	// get all users
	cursor, users, err := s.DataClient.GetUsers(cursor, n)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, UserIterator{Cursor: cursor, Users: users})
}

// delete a user by user-id
func (s *RestServer) deleteUser(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// get user-id and put into temp
	userId := request.PathParameter("user-id")
	if err := s.DataClient.DeleteUser(userId); err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, Success{RowAffected: 1})
}

// get feedback by user-id with feedback type
func (s *RestServer) getTypedFeedbackByUser(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	feedback, err := s.DataClient.GetUserFeedback(userId, feedbackType)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, feedback)
}

// get feedback by user-id
func (s *RestServer) getFeedbackByUser(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	userId := request.PathParameter("user-id")
	feedback, err := s.DataClient.GetUserFeedback(userId)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, feedback)
}

// Item is the data structure for the item but stores the timestamp using string.
type Item struct {
	ItemId    string
	Timestamp string
	Labels    []string
	Comment   string
}

func (s *RestServer) insertItems(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Add ratings
	items := make([]Item, 0)
	if err := request.ReadEntity(&items); err != nil {
		BadRequest(response, err)
		return
	}
	// Insert items
	var count int
	for _, item := range items {
		// parse datetime
		timestamp, err := dateparse.ParseAny(item.Timestamp)
		if err != nil {
			BadRequest(response, err)
			return
		}
		err = s.DataClient.BatchInsertItems([]data.Item{{ItemId: item.ItemId, Timestamp: timestamp, Labels: item.Labels, Comment: item.Comment}})
		count++
		if err != nil {
			InternalServerError(response, err)
			return
		}
		// insert modify timestamp
		if err = s.CacheClient.SetTime(cache.LastModifyItemTime, item.ItemId, time.Now()); err != nil {
			return
		}
	}
	Ok(response, Success{RowAffected: count})
}

func (s *RestServer) insertItem(request *restful.Request, response *restful.Response) {
	// authorize
	if !s.auth(request, response) {
		return
	}
	item := new(Item)
	var err error
	if err = request.ReadEntity(item); err != nil {
		BadRequest(response, err)
		return
	}
	// parse datetime
	timestamp, err := dateparse.ParseAny(item.Timestamp)
	if err != nil {
		BadRequest(response, err)
		return
	}
	if err = s.DataClient.BatchInsertItems([]data.Item{{ItemId: item.ItemId, Timestamp: timestamp, Labels: item.Labels, Comment: item.Comment}}); err != nil {
		InternalServerError(response, err)
		return
	}
	// insert modify timestamp
	if err = s.CacheClient.SetTime(cache.LastModifyItemTime, item.ItemId, time.Now()); err != nil {
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
	// Authorize
	if !s.auth(request, response) {
		return
	}
	cursor := request.QueryParameter("cursor")
	n, err := ParseInt(request, "n", s.GorseConfig.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	cursor, items, err := s.DataClient.GetItems(cursor, n, nil)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, ItemIterator{Cursor: cursor, Items: items})
}

func (s *RestServer) getItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get item id
	itemId := request.PathParameter("item-id")
	// Get item
	item, err := s.DataClient.GetItem(itemId)
	if err != nil {
		if err == data.ErrItemNotExist {
			PageNotFound(response, err)
		} else {
			InternalServerError(response, err)
		}
		return
	}
	Ok(response, item)
}

func (s *RestServer) deleteItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	itemId := request.PathParameter("item-id")
	if err := s.DataClient.DeleteItem(itemId); err != nil {
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

func (s *RestServer) insertFeedback(request *restful.Request, response *restful.Response) {
	// authorize
	if !s.auth(request, response) {
		return
	}
	// add ratings
	feedbackLiterTime := new([]Feedback)
	if err := request.ReadEntity(feedbackLiterTime); err != nil {
		BadRequest(response, err)
		return
	}
	// parse datetime
	var err error
	feedback := make([]data.Feedback, len(*feedbackLiterTime))
	users := set.NewStringSet()
	items := set.NewStringSet()
	for i := range feedback {
		users.Add((*feedbackLiterTime)[i].UserId)
		items.Add((*feedbackLiterTime)[i].ItemId)
		feedback[i].FeedbackKey = (*feedbackLiterTime)[i].FeedbackKey
		feedback[i].Comment = (*feedbackLiterTime)[i].Comment
		feedback[i].Timestamp, err = dateparse.ParseAny((*feedbackLiterTime)[i].Timestamp)
		if err != nil {
			BadRequest(response, err)
			return
		}
	}
	// insert feedback to data store
	err = s.DataClient.BatchInsertFeedback(feedback,
		s.GorseConfig.Database.AutoInsertUser,
		s.GorseConfig.Database.AutoInsertItem)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	// insert feedback to cache store
	if err = s.InsertFeedbackToCache(feedback); err != nil {
		InternalServerError(response, err)
		return
	}

	for _, userId := range users.List() {
		err = s.CacheClient.SetTime(cache.LastModifyUserTime, userId, time.Now())
		if err != nil {
			InternalServerError(response, err)
			return
		}
	}
	for _, itemId := range items.List() {
		err = s.CacheClient.SetTime(cache.LastModifyItemTime, itemId, time.Now())
		if err != nil {
			InternalServerError(response, err)
			return
		}
	}
	Ok(response, Success{RowAffected: len(feedback)})
}

// FeedbackIterator is the iterator for feedback.
type FeedbackIterator struct {
	Cursor   string
	Feedback []data.Feedback
}

func (s *RestServer) getFeedback(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Parse parameters
	cursor := request.QueryParameter("cursor")
	n, err := ParseInt(request, "n", s.GorseConfig.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	cursor, feedback, err := s.DataClient.GetFeedback(cursor, n, nil)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, FeedbackIterator{Cursor: cursor, Feedback: feedback})
}

func (s *RestServer) getTypedFeedback(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Parse parameters
	feedbackType := request.PathParameter("feedback-type")
	cursor := request.QueryParameter("cursor")
	n, err := ParseInt(request, "n", s.GorseConfig.Server.DefaultN)
	if err != nil {
		BadRequest(response, err)
		return
	}
	cursor, feedback, err := s.DataClient.GetFeedback(cursor, n, nil, feedbackType)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, FeedbackIterator{Cursor: cursor, Feedback: feedback})
}

func (s *RestServer) getUserItemFeedback(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Parse parameters
	userId := request.PathParameter("user-id")
	itemId := request.PathParameter("item-id")
	if feedback, err := s.DataClient.GetUserItemFeedback(userId, itemId); err != nil {
		InternalServerError(response, err)
	} else {
		Ok(response, feedback)
	}
}

func (s *RestServer) deleteUserItemFeedback(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Parse parameters
	userId := request.PathParameter("user-id")
	itemId := request.PathParameter("item-id")
	if deleteCount, err := s.DataClient.DeleteUserItemFeedback(userId, itemId); err != nil {
		InternalServerError(response, err)
	} else {
		Ok(response, Success{RowAffected: deleteCount})
	}
}

func (s *RestServer) getTypedUserItemFeedback(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Parse parameters
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	itemId := request.PathParameter("item-id")
	if feedback, err := s.DataClient.GetUserItemFeedback(userId, itemId, feedbackType); err != nil {
		InternalServerError(response, err)
	} else if feedbackType == "" {
		Text(response, "{}")
	} else {
		Ok(response, feedback[0])
	}
}

func (s *RestServer) deleteTypedUserItemFeedback(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Parse parameters
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	itemId := request.PathParameter("item-id")
	if deleteCount, err := s.DataClient.DeleteUserItemFeedback(userId, itemId, feedbackType); err != nil {
		InternalServerError(response, err)
	} else {
		Ok(response, Success{deleteCount})
	}
}

func (s *RestServer) getMeasurements(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Parse parameters
	name := request.PathParameter("name")
	n, err := ParseInt(request, "n", 100)
	if err != nil {
		BadRequest(response, err)
		return
	}
	measurements, err := s.DataClient.GetMeasurements(name, n)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, measurements)
}

// BadRequest returns a bad request error.
func BadRequest(response *restful.Response, err error) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	base.Logger().Error("bad request", zap.Error(err))
	if err = response.WriteError(http.StatusBadRequest, err); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
}

// InternalServerError returns a internal server error.
func InternalServerError(response *restful.Response, err error) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	base.Logger().Error("internal server error", zap.Error(err))
	if err = response.WriteError(http.StatusInternalServerError, err); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
}

// PageNotFound returns a not found error.
func PageNotFound(response *restful.Response, err error) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	if err := response.WriteError(http.StatusNotFound, err); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
}

// Ok sends the content as JSON to the client.
func Ok(response *restful.Response, content interface{}) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	if err := response.WriteAsJson(content); err != nil {
		base.Logger().Error("failed to write json", zap.Error(err))
	}
}

// Text returns a plain text.
func Text(response *restful.Response, content string) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	if _, err := response.Write([]byte(content)); err != nil {
		base.Logger().Error("failed to write text", zap.Error(err))
	}
}

func (s *RestServer) auth(request *restful.Request, response *restful.Response) bool {
	if !s.EnableAuth || s.GorseConfig.Server.APIKey == "" {
		return true
	}
	apikey := request.HeaderParameter("X-API-Key")
	if apikey == s.GorseConfig.Server.APIKey {
		return true
	}
	base.Logger().Error("unauthorized",
		zap.String("api_key", s.GorseConfig.Server.APIKey),
		zap.String("X-API-Key", apikey))
	if err := response.WriteError(http.StatusUnauthorized, fmt.Errorf("unauthorized")); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
	return false
}

// InsertFeedbackToCache inserts feedback to cache.
func (s *RestServer) InsertFeedbackToCache(feedback []data.Feedback) error {
	for _, v := range feedback {
		err := s.CacheClient.AppendList(cache.IgnoreItems, v.UserId, v.ItemId)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}
