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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/scylladb/go-set"
	"net/http"
	"strconv"
	"time"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
)

type RestServer struct {
	CacheStore  cache.Database
	DataStore   data.Database
	GorseConfig *config.Config
	HttpHost    string
	HttpPort    int
	EnableAuth  bool
	WebService  *restful.WebService
}

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

func (s *RestServer) CreateWebService() {
	// Create a server
	ws := s.WebService
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	ws.Path("/api/")

	/* Interactions with data store */

	// Insert a user
	ws.Route(ws.POST("/user").To(s.insertUser).
		Doc("Insert a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"user"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Reads(data.User{}))
	// Get a user
	ws.Route(ws.GET("/user/{user-id}").To(s.getUser).
		Doc("Get a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"user"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes(data.User{}))
	// Insert users
	ws.Route(ws.POST("/users").To(s.insertUsers).
		Doc("Insert users.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"user"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Reads([]data.User{}))
	// Get users
	ws.Route(ws.GET("/users").To(s.getUsers).
		Doc("Get users.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"user"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned users").DataType("int")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Writes(UserIterator{}))
	// Delete a user
	ws.Route(ws.DELETE("/user/{user-id}").To(s.deleteUser).
		Doc("Delete a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"user"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes(Success{}))

	// Insert an item
	ws.Route(ws.POST("/item").To(s.insertItem).
		Doc("Insert an item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"item"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Reads(data.Item{}))
	// Get items
	ws.Route(ws.GET("/items").To(s.getItems).
		Doc("Get items.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"item"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Writes(ItemIterator{}))
	// Get item
	ws.Route(ws.GET("/item/{item-id}").To(s.getItem).
		Doc("Get a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"item"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")).
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
		Writes(Success{}))

	// Insert feedback
	ws.Route(ws.POST("/feedback").To(s.insertFeedback).
		Doc("Insert multiple feedback.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Reads([]data.Feedback{}))
	// Get feedback
	ws.Route(ws.GET("/feedback").To(s.getFeedback).
		Doc("Get multiple feedback.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned feedback").DataType("integer")).
		Writes(FeedbackIterator{}))
	ws.Route(ws.GET("/feedback/{user-id}/{item-id}").To(s.getUserItemFeedback).
		Doc("Get feedback between a user and a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Writes([]data.Feedback{}))
	ws.Route(ws.DELETE("/feedback/{user-id}/{item-id}").To(s.deleteUserItemFeedback).
		Doc("Delete feedback between a user and a item.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Writes([]data.Feedback{}))
	ws.Route(ws.GET("/feedback/{feedback-type}").To(s.getTypedFeedback).
		Doc("Get multiple feedback with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned feedback").DataType("integer")).
		Writes(FeedbackIterator{}))
	ws.Route(ws.GET("/feedback/{feedback-type}/{user-id}/{item-id}").To(s.getTypedUserItemFeedback).
		Doc("Get feedback between a user and a item with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Writes(data.Feedback{}))
	ws.Route(ws.DELETE("/feedback/{feedback-type}/{user-id}/{item-id}").To(s.deleteTypedUserItemFeedback).
		Doc("Delete feedback between a user and a item with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Writes(data.Feedback{}))
	// Get feedback by user id
	ws.Route(ws.GET("/user/{user-id}/feedback/{feedback-type}").To(s.getTypedFeedbackByUser).
		Doc("Get feedback by user id with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Writes([]data.Feedback{}))
	ws.Route(ws.GET("/user/{user-id}/feedback/").To(s.getFeedbackByUser).
		Doc("Get feedback by user id.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes([]data.Feedback{}))
	// Get feedback by item-id
	ws.Route(ws.GET("/item/{item-id}/feedback/{feedback-type}").To(s.getTypedFeedbackByItem).
		Doc("Get feedback by item id with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("strung")).
		Writes([]string{}))
	ws.Route(ws.GET("/item/{item-id}/feedback/").To(s.getFeedbackByItem).
		Doc("Get feedback by item id.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Writes([]string{}))

	/* Interaction with intermediate result */

	// Get collaborative filtering recommendation by user id
	ws.Route(ws.GET("/intermediate/recommend/{user-id}").To(s.getCollaborative).
		Doc("get the collaborative filtering recommendation for a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{"intermediate"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	// Get subscribe items
	//ws.Route(ws.GET("/intermediate/subscribe/{user-id}").To(s.getSubscribe).
	//	Doc("get subscribe items for a user").
	//	Metadata(restfulspec.KeyOpenAPITags, []string{"intermediate"}).
	//	Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
	//	Param(ws.QueryParameter("user-id", "identifier of the user").DataType("string")).
	//	Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
	//	Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
	//	Param(ws.QueryParameter("return", "return type (id/detail)").DataType("string")).
	//	Writes([]string{}))

	/* Rank recommendation */

	// Get popular items
	ws.Route(ws.GET("/popular").To(s.getPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	ws.Route(ws.GET("/popular/{label}").To(s.getLabelPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	// Get latest items
	ws.Route(ws.GET("/latest").To(s.getLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	ws.Route(ws.GET("/latest/{label}").To(s.getLabelLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	// Get neighbors
	ws.Route(ws.GET("/neighbors/{item-id}").To(s.getNeighbors).
		Doc("get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	ws.Route(ws.GET("/recommend/{user-id}").To(s.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("write-back", "write recommendation back to feedback").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Writes([]string{}))

	/* Interaction with measurements */

	ws.Route(ws.GET("/measurements/{name}").To(s.getMeasurements).
		Doc("Get measurements").
		Metadata(restfulspec.KeyOpenAPITags, []string{"measurements"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Writes([]data.Measurement{}))
}

func ParseInt(request *restful.Request, name string, fallback int) (value int, err error) {
	valueString := request.QueryParameter(name)
	value, err = strconv.Atoi(valueString)
	if err != nil && valueString == "" {
		value = fallback
		err = nil
	}
	return
}

func (s *RestServer) getList(prefix string, name string, request *restful.Request, response *restful.Response) {
	var begin, end int
	var err error
	if begin, err = ParseInt(request, "begin", 0); err != nil {
		BadRequest(response, err)
		return
	}
	if end, err = ParseInt(request, "end", s.GorseConfig.Server.DefaultN-1); err != nil {
		BadRequest(response, err)
		return
	}
	// Get the popular list
	items, err := s.CacheStore.GetList(prefix, name, begin, end)
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

func (s *RestServer) getLabelPopular(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	label := request.PathParameter("label")
	base.Logger().Debug("get label popular items", zap.String("label", label))
	s.getList(cache.PopularItems, label, request, response)
}

func (s *RestServer) getLabelLatest(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	label := request.PathParameter("label")
	base.Logger().Debug("get label latest items", zap.String("label", label))
	s.getList(cache.LatestItems, label, request, response)
}

// get feedback by item-id with feedback type
func (s *RestServer) getTypedFeedbackByItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	feedbackType := request.PathParameter("feedback-type")
	itemId := request.PathParameter("item-id")
	feedback, err := s.DataStore.GetItemFeedback(itemId, &feedbackType)
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
	feedback, err := s.DataStore.GetItemFeedback(itemId, nil)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, feedback)
}

// getNeighbors gets neighbors of a item from database.
func (s *RestServer) getNeighbors(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get item id
	itemId := request.PathParameter("item-id")
	s.getList(cache.SimilarItems, itemId, request, response)
}

// getSubscribe gets subscribed items of a user from database.
func (s *RestServer) getSubscribe(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get user id
	userId := request.PathParameter("user-id")
	s.getList(cache.SubscribeItems, userId, request, response)
}

// getCollaborative gets cached recommended items from database.
func (s *RestServer) getCollaborative(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get user id
	userId := request.PathParameter("user-id")
	s.getList(cache.CollaborativeItems, userId, request, response)
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
	// load offline recommendation
	start := time.Now()
	itemsChan := make(chan []string, 1)
	errChan := make(chan error, 1)
	go func() {
		var collaborativeFilteringItems []string
		collaborativeFilteringItems, err = s.CacheStore.GetList(cache.CollaborativeItems, userId, 0, s.GorseConfig.Database.CacheSize)
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
	userFeedback, err := s.DataStore.GetUserFeedback(userId, nil)
	if err != nil {
		InternalServerError(response, err)
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
		InternalServerError(response, err)
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
	// write back
	if writeBackFeedback != "" {
		for _, itemId := range results {
			err = s.DataStore.InsertFeedback(data.Feedback{
				FeedbackKey: data.FeedbackKey{
					UserId:       userId,
					ItemId:       itemId,
					FeedbackType: writeBackFeedback,
				},
				Timestamp: time.Now(),
			}, false, false)
			if err != nil {
				InternalServerError(response, err)
				return
			}
		}
	}
	// Send result
	Ok(response, results)
}

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
	if err := s.DataStore.InsertUser(temp); err != nil {
		InternalServerError(response, err)
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
	user, err := s.DataStore.GetUser(userId)
	if err != nil {
		if err.Error() == data.ErrUserNotExist {
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
	for _, user := range *temp {
		if err := s.DataStore.InsertUser(user); err != nil {
			InternalServerError(response, err)
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
	cursor, users, err := s.DataStore.GetUsers(cursor, n)
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
	if err := s.DataStore.DeleteUser(userId); err != nil {
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
	feedback, err := s.DataStore.GetUserFeedback(userId, &feedbackType)
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
	feedback, err := s.DataStore.GetUserFeedback(userId, nil)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, feedback)
}

// putItems puts items into the database.
func (s *RestServer) insertItems(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Add ratings
	items := make([]data.Item, 0)
	if err := request.ReadEntity(&items); err != nil {
		BadRequest(response, err)
		return
	}
	// Insert items
	var count int
	for _, item := range items {
		err := s.DataStore.InsertItem(data.Item{ItemId: item.ItemId, Timestamp: item.Timestamp, Labels: item.Labels})
		count++
		if err != nil {
			InternalServerError(response, err)
			return
		}
	}
	Ok(response, Success{RowAffected: count})
}

func (s *RestServer) insertItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	item := new(data.Item)
	var err error
	if err = request.ReadEntity(item); err != nil {
		BadRequest(response, err)
		return
	}
	if err = s.DataStore.InsertItem(*item); err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, Success{RowAffected: 1})
}

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
	cursor, items, err := s.DataStore.GetItems(cursor, n)
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
	item, err := s.DataStore.GetItem(itemId)
	if err != nil {
		if err.Error() == data.ErrItemNotExist {
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
	if err := s.DataStore.DeleteItem(itemId); err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, Success{RowAffected: 1})
}

// putFeedback puts new ratings into the database.
func (s *RestServer) insertFeedback(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Add ratings
	ratings := new([]data.Feedback)
	if err := request.ReadEntity(ratings); err != nil {
		BadRequest(response, err)
		return
	}
	var err error
	// Insert feedback
	var count int
	users := set.NewStringSet()
	for _, feedback := range *ratings {
		users.Add(feedback.UserId)
		err = s.DataStore.InsertFeedback(feedback,
			s.GorseConfig.Database.AutoInsertUser,
			s.GorseConfig.Database.AutoInsertItem)
		count++
		if err != nil {
			InternalServerError(response, err)
			return
		}
	}
	for _, userId := range users.List() {
		err = s.CacheStore.SetString(cache.LastActiveTime, userId, base.Now())
		if err != nil {
			InternalServerError(response, err)
			return
		}
	}
	Ok(response, Success{RowAffected: count})
}

type FeedbackIterator struct {
	Cursor   string
	Feedback []data.Feedback
}

// Get feedback
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
	cursor, feedback, err := s.DataStore.GetFeedback(cursor, n, nil)
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
	cursor, feedback, err := s.DataStore.GetFeedback(cursor, n, &feedbackType)
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
	if feedback, err := s.DataStore.GetUserItemFeedback(userId, itemId, nil); err != nil {
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
	if deleteCount, err := s.DataStore.DeleteUserItemFeedback(userId, itemId, nil); err != nil {
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
	if feedback, err := s.DataStore.GetUserItemFeedback(userId, itemId, &feedbackType); err != nil {
		InternalServerError(response, err)
	} else if len(feedbackType) == 0 {
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
	if deleteCount, err := s.DataStore.DeleteUserItemFeedback(userId, itemId, &feedbackType); err != nil {
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
	measurements, err := s.DataStore.GetMeasurements(name, n)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Ok(response, measurements)
}

func BadRequest(response *restful.Response, err error) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	base.Logger().Error("bad request", zap.Error(err))
	if err = response.WriteError(400, err); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
}

func InternalServerError(response *restful.Response, err error) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	base.Logger().Error("internal server error", zap.Error(err))
	if err = response.WriteError(500, err); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
}

func PageNotFound(response *restful.Response, err error) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	if err := response.WriteError(400, err); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
}

// json sends the content as JSON to the client.
func Ok(response *restful.Response, content interface{}) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	if err := response.WriteAsJson(content); err != nil {
		base.Logger().Error("failed to write json", zap.Error(err))
	}
}

func Text(response *restful.Response, content string) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	if _, err := response.Write([]byte(content)); err != nil {
		base.Logger().Error("failed to write text", zap.Error(err))
	}
}

func (s *RestServer) auth(request *restful.Request, response *restful.Response) bool {
	if s.GorseConfig.Server.APIKey == "" {
		return true
	}
	apikey := request.HeaderParameter("X-API-Key")
	if apikey == s.GorseConfig.Server.APIKey {
		return true
	}
	base.Logger().Error("unauthorized",
		zap.String("api_key", s.GorseConfig.Server.APIKey),
		zap.String("X-API-Key", apikey))
	if err := response.WriteError(401, fmt.Errorf("unauthorized")); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
	return false
}
