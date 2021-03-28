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
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/rank"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Server struct {
	CacheStore   cache.Database
	DataStore    data.Database
	Config       *config.Config
	MasterClient protocol.MasterClient

	fmModel        rank.FactorizationMachine
	RankModelMutex sync.RWMutex
	fmVersion      int64

	MasterHost string
	MasterPort int
	ServerHost string
	ServerPort int
}

func NewServer(masterHost string, masterPort int, serverHost string, serverPort int) *Server {
	return &Server{
		MasterHost: masterHost,
		MasterPort: masterPort,
		ServerHost: serverHost,
		ServerPort: serverPort,
	}
}

func (s *Server) Serve() {
	// connect to master
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", s.MasterHost, s.MasterPort), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("server: failed to connect master (%v)", err)
	}
	s.MasterClient = protocol.NewMasterClient(conn)

	// load master config
	masterCfgJson, err := s.MasterClient.GetConfig(context.Background(), &protocol.Void{})
	if err != nil {
		log.Fatalf("server: failed to load master config (%v)", err)
	}
	err = json.Unmarshal([]byte(masterCfgJson.Json), &s.Config)
	if err != nil {
		log.Fatalf("server: failed to parse master config (%v)", err)
	}

	// connect to data store
	if s.DataStore, err = data.Open(s.Config.Database.DataStore); err != nil {
		log.Fatalf("server: failed to connect data store (%v)", err)
	}

	// connect to cache store
	if s.CacheStore, err = cache.Open(s.Config.Database.CacheStore); err != nil {
		log.Fatalf("server: failed to connect cache store (%v)", err)
	}

	// register to master
	go s.Register()
	// pull model
	go s.Sync()

	// register restful APIs
	ws := s.CreateWebService()
	restful.DefaultContainer.Add(ws)

	// register swagger UI
	specConfig := restfulspec.Config{
		WebServices: restful.RegisteredWebServices(),
		APIPath:     "/apidocs.json",
	}
	restful.DefaultContainer.Add(restfulspec.NewOpenAPIService(specConfig))
	swaggerFile = fmt.Sprintf("http://%s:%d/apidocs.json", s.ServerHost, s.ServerPort)
	http.HandleFunc(apiDocsPath, handler)

	log.Printf("start gorse server at %v\n", fmt.Sprintf("%s:%d", s.ServerHost, s.ServerPort))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", s.ServerHost, s.ServerPort), nil))
}

// Sync factorization machine.
func (s *Server) Sync() {
	defer base.CheckPanic()
	for {
		ctx := context.Background()

		// pull factorization machine
		base.Logger().Debug("check factorization machine version")
		if fmVersionResponse, err := s.MasterClient.GetFactorizationMachineVersion(ctx, &protocol.Void{}); err != nil {
			base.Logger().Error("failed to check factorization machine version", zap.Error(err))
		} else if fmVersionResponse.Version == 0 {
			base.Logger().Debug("remote factorization machine doesn't exist")
		} else if fmVersionResponse.Version != s.fmVersion {
			if mfResponse, err := s.MasterClient.GetFactorizationMachine(context.Background(), &protocol.Void{}, grpc.MaxCallRecvMsgSize(10e9)); err != nil {
				base.Logger().Error("failed to pull factorization machine", zap.Error(err))
			} else {
				s.fmModel, err = rank.DecodeModel(mfResponse.Model)
				if err != nil {
					base.Logger().Error("failed to decode factorization machine", zap.Error(err))
				} else {
					s.fmVersion = mfResponse.Version
					base.Logger().Info("sync factorization machine", zap.Int64("version", s.fmVersion))
				}
			}
		}

		// sleep
		time.Sleep(time.Minute)
	}
}

// Register this server to the master.
func (s *Server) Register() {
	defer base.CheckPanic()
	for {
		if _, err := s.MasterClient.RegisterServer(context.Background(), &protocol.Void{}); err != nil {
			base.Logger().Error("failed to register", zap.Error(err))
		}
		time.Sleep(time.Duration(s.Config.Master.ClusterMetaTimeout/2) * time.Second)
	}
}

func (s *Server) CreateWebService() *restful.WebService {
	// Create a server
	ws := new(restful.WebService)
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)

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
		Param(ws.FormParameter("n", "number of returned users").DataType("int")).
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
		Param(ws.FormParameter("n", "number of returned items").DataType("int")).
		Param(ws.FormParameter("cursor", "cursor for next page").DataType("string")).
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
	ws.Route(ws.GET("/feedback/{feedback-type}").To(s.getTypedFeedback).
		Doc("Get multiple feedback with feedback type.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"feedback"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned feedback").DataType("integer")).
		Writes(FeedbackIterator{}))
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

	/* Interaction with cache store */

	// Get collaborative filtering recommendation by user id
	ws.Route(ws.GET("/collaborative/{user-id}").To(s.getCollaborative).
		Doc("get the collaborative filtering recommendation for a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.FormParameter("n", "number of returned items").DataType("int")).
		Param(ws.FormParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	// Get subscribe items
	ws.Route(ws.GET("/subscribe/{user-id}").To(s.getSubscribe).
		Doc("get subscribe items for a user").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.FormParameter("n", "number of returned items").DataType("int")).
		Param(ws.FormParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	// Get popular items
	ws.Route(ws.GET("/popular").To(s.getPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.FormParameter("n", "number of returned items").DataType("int")).
		Param(ws.FormParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	ws.Route(ws.GET("/popular/{label}").To(s.getLabelPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.FormParameter("n", "number of returned items").DataType("int")).
		Param(ws.FormParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	// Get latest items
	ws.Route(ws.GET("/latest").To(s.getLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.FormParameter("n", "number of returned items").DataType("int")).
		Param(ws.FormParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	ws.Route(ws.GET("/latest/{label}").To(s.getLabelLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.FormParameter("n", "number of returned items").DataType("int")).
		Param(ws.FormParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))
	// Get neighbors
	ws.Route(ws.GET("/neighbors/{item-id}").To(s.getNeighbors).
		Doc("get neighbors of a item").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.FormParameter("n", "number of returned items").DataType("int")).
		Param(ws.FormParameter("offset", "offset of the list").DataType("int")).
		Writes([]string{}))

	/* Rank recommendation */

	ws.Route(ws.GET("/recommend/{user-id}").To(s.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"recommendation"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("write-back", "write recommendation back to feedback").DataType("string")).
		Param(ws.FormParameter("n", "number of returned items").DataType("int")).
		Writes([]string{}))

	return ws
}

func parseInt(request *restful.Request, name string, fallback int) (value int, err error) {
	valueString := request.QueryParameter(name)
	value, err = strconv.Atoi(valueString)
	if err != nil && valueString == "" {
		value = fallback
		err = nil
	}
	return
}

func (s *Server) getList(prefix string, name string, request *restful.Request, response *restful.Response) {
	var n, offset int
	var err error
	if n, err = parseInt(request, "n", s.Config.Server.DefaultN); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get the popular list
	items, err := s.CacheStore.GetList(prefix, name, n, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	ok(response, items)
}

// getPopular gets popular items from database.
func (s *Server) getPopular(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	base.Logger().Debug("get popular items")
	s.getList(cache.PopularItems, "", request, response)
}

func (s *Server) getLatest(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	base.Logger().Debug("get latest items")
	s.getList(cache.LatestItems, "", request, response)
}

func (s *Server) getLabelPopular(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	label := request.PathParameter("label")
	base.Logger().Debug("get label popular items", zap.String("label", label))
	s.getList(cache.PopularItems, label, request, response)
}

func (s *Server) getLabelLatest(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	label := request.PathParameter("label")
	base.Logger().Debug("get label latest items", zap.String("label", label))
	s.getList(cache.LatestItems, label, request, response)
}

// get feedback by item-id with feedback type
func (s *Server) getTypedFeedbackByItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	feedbackType := request.PathParameter("feedback-type")
	itemId := request.PathParameter("item-id")
	feedback, err := s.DataStore.GetItemFeedback(itemId, &feedbackType)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, feedback)
}

// get feedback by item-id
func (s *Server) getFeedbackByItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	itemId := request.PathParameter("item-id")
	feedback, err := s.DataStore.GetItemFeedback(itemId, nil)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, feedback)
}

// getNeighbors gets neighbors of a item from database.
func (s *Server) getNeighbors(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get item id
	itemId := request.PathParameter("item-id")
	s.getList(cache.SimilarItems, itemId, request, response)
}

// getSubscribe gets subscribed items of a user from database.
func (s *Server) getSubscribe(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get user id
	userId := request.PathParameter("user-id")
	s.getList(cache.SubscribeItems, userId, request, response)
}

// getCollaborative gets cached recommended items from database.
func (s *Server) getCollaborative(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get user id
	userId := request.PathParameter("user-id")
	s.getList(cache.CollaborativeItems, userId, request, response)
}

func (s *Server) getRecommend(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	start := time.Now()
	userId := request.PathParameter("user-id")
	n, err := parseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		badRequest(response, err)
		return
	}
	writeBackFeedback := request.QueryParameter("write-back")
	candidateCollections := make(chan []string, 3)
	errors := make([]error, 3)
	// load populars
	go func() {
		popularItems, err := s.CacheStore.GetList(cache.PopularItems, "", s.Config.Popular.NumCache, 0)
		if err != nil {
			errors[0] = err
			candidateCollections <- nil
		} else {
			candidateCollections <- popularItems
		}
	}()
	// load latest
	go func() {
		latestItems, err := s.CacheStore.GetList(cache.LatestItems, "", s.Config.Latest.NumCache, 0)
		if err != nil {
			errors[1] = err
			candidateCollections <- nil
		} else {
			candidateCollections <- latestItems
		}
	}()
	// load matched
	go func() {
		matchedItems, err := s.CacheStore.GetList(cache.CollaborativeItems, userId, s.Config.Collaborative.NumCached, 0)
		if err != nil {
			errors[2] = err
			candidateCollections <- nil
		} else {
			candidateCollections <- matchedItems
		}
	}()
	// load feedback
	userFeedback, err := s.DataStore.GetUserFeedback(userId, nil)
	if err != nil {
		internalServerError(response, err)
		return
	}
	excludeSet := base.NewStringSet()
	for _, feedback := range userFeedback {
		excludeSet.Add(feedback.ItemId)
	}
	// merge collections
	candidateItems := make([]string, 0)
	for i := 0; i < 3; i++ {
		items := <-candidateCollections
		for _, itemId := range items {
			if !excludeSet.Contain(itemId) {
				candidateItems = append(candidateItems, itemId)
				excludeSet.Add(itemId)
			}
		}
	}
	for i := 0; i < 3; i++ {
		if errors[i] != nil {
			internalServerError(response, errors[i])
			return
		}
	}
	// collect item features
	candidateFeaturedItems := make([]data.Item, len(candidateItems))
	err = base.Parallel(len(candidateItems), 4, func(_, jobId int) error {
		candidateFeaturedItems[jobId], err = s.DataStore.GetItem(candidateItems[jobId])
		return err
	})
	if err != nil {
		internalServerError(response, err)
		return
	}
	// online predict
	startPredictTime := time.Now()
	recItems := base.NewTopKStringFilter(n)
	for _, item := range candidateFeaturedItems {
		s.RankModelMutex.RLock()
		m := s.fmModel
		s.RankModelMutex.RUnlock()
		recItems.Push(item.ItemId, m.Predict(userId, item.ItemId, item.Labels))
	}
	result, _ := recItems.PopAll()
	spent := time.Since(start)
	predictTime := time.Since(startPredictTime)
	base.Logger().Info("complete recommendation",
		zap.Int("n_candidate", len(candidateItems)),
		zap.Duration("total_time", spent),
		zap.Duration("predict_time", predictTime))
	// write back
	if writeBackFeedback != "" {
		for _, itemId := range result {
			err = s.DataStore.InsertFeedback(data.Feedback{
				FeedbackKey: data.FeedbackKey{
					UserId:       userId,
					ItemId:       itemId,
					FeedbackType: writeBackFeedback,
				},
				Timestamp: time.Now(),
			}, false, false)
			if err != nil {
				internalServerError(response, err)
				return
			}
		}
	}
	ok(response, result)
}

type Success struct {
	RowAffected int
}

func (s *Server) insertUser(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	temp := data.User{}
	// get userInfo from request and put into temp
	if err := request.ReadEntity(&temp); err != nil {
		badRequest(response, err)
		return
	}
	if err := s.DataStore.InsertUser(temp); err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, Success{RowAffected: 1})
}

func (s *Server) getUser(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// get user id
	userId := request.PathParameter("user-id")
	// get user
	user, err := s.DataStore.GetUser(userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, user)
}

func (s *Server) insertUsers(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	temp := new([]data.User)
	// get param from request and put into temp
	if err := request.ReadEntity(temp); err != nil {
		badRequest(response, err)
		return
	}
	var count int
	// range temp and achieve user
	for _, user := range *temp {
		if err := s.DataStore.InsertUser(user); err != nil {
			internalServerError(response, err)
			return
		}
		count++
	}
	ok(response, Success{RowAffected: count})
}

type UserIterator struct {
	Cursor string
	Users  []data.User
}

func (s *Server) getUsers(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		badRequest(response, err)
		return
	}
	// get all users
	cursor, users, err := s.DataStore.GetUsers(cursor, n)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, UserIterator{Cursor: cursor, Users: users})
}

// delete a user by user-id
func (s *Server) deleteUser(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// get user-id and put into temp
	userId := request.PathParameter("user-id")
	if err := s.DataStore.DeleteUser(userId); err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, Success{RowAffected: 1})
}

// get feedback by user-id with feedback type
func (s *Server) getTypedFeedbackByUser(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	feedback, err := s.DataStore.GetUserFeedback(userId, &feedbackType)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, feedback)
}

// get feedback by user-id
func (s *Server) getFeedbackByUser(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	userId := request.PathParameter("user-id")
	feedback, err := s.DataStore.GetUserFeedback(userId, nil)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, feedback)
}

// putItems puts items into the database.
func (s *Server) insertItems(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Add ratings
	items := make([]data.Item, 0)
	if err := request.ReadEntity(&items); err != nil {
		badRequest(response, err)
		return
	}
	// Insert items
	var count int
	for _, item := range items {
		err := s.DataStore.InsertItem(data.Item{ItemId: item.ItemId, Timestamp: item.Timestamp, Labels: item.Labels})
		count++
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	ok(response, Success{RowAffected: count})
}

func (s *Server) insertItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	item := new(data.Item)
	var err error
	if err = request.ReadEntity(item); err != nil {
		badRequest(response, err)
		return
	}
	if err = s.DataStore.InsertItem(*item); err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, Success{RowAffected: 1})
}

type ItemIterator struct {
	Cursor string
	Items  []data.Item
}

func (s *Server) getItems(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		badRequest(response, err)
		return
	}
	cursor, items, err := s.DataStore.GetItems(cursor, n)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, ItemIterator{Cursor: cursor, Items: items})
}

func (s *Server) getItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Get item id
	itemId := request.PathParameter("item-id")
	// Get item
	item, err := s.DataStore.GetItem(itemId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, item)
}

func (s *Server) deleteItem(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	itemId := request.PathParameter("item-id")
	if err := s.DataStore.DeleteItem(itemId); err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, Success{RowAffected: 1})
}

// putFeedback puts new ratings into the database.
func (s *Server) insertFeedback(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Add ratings
	ratings := new([]data.Feedback)
	if err := request.ReadEntity(ratings); err != nil {
		badRequest(response, err)
		return
	}
	var err error
	// Insert feedback
	var count int
	for _, feedback := range *ratings {
		err = s.DataStore.InsertFeedback(feedback,
			s.Config.Database.AutoInsertUser,
			s.Config.Database.AutoInsertItem)
		count++
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	ok(response, Success{RowAffected: count})
}

type FeedbackIterator struct {
	Cursor   string
	Feedback []data.Feedback
}

// Get feedback
func (s *Server) getFeedback(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Parse parameters
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		badRequest(response, err)
		return
	}
	cursor, feedback, err := s.DataStore.GetFeedback(cursor, n, nil)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, FeedbackIterator{Cursor: cursor, Feedback: feedback})
}

func (s *Server) getTypedFeedback(request *restful.Request, response *restful.Response) {
	// Authorize
	if !s.auth(request, response) {
		return
	}
	// Parse parameters
	feedbackType := request.PathParameter("feedback-type")
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.Server.DefaultN)
	if err != nil {
		badRequest(response, err)
		return
	}
	cursor, feedback, err := s.DataStore.GetFeedback(cursor, n, &feedbackType)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, FeedbackIterator{Cursor: cursor, Feedback: feedback})
}

func badRequest(response *restful.Response, err error) {
	base.Logger().Error("bad request", zap.Error(err))
	if err = response.WriteError(400, err); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
}

func internalServerError(response *restful.Response, err error) {
	base.Logger().Error("internal server error", zap.Error(err))
	if err = response.WriteError(500, err); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
}

// json sends the content as JSON to the client.
func ok(response *restful.Response, content interface{}) {
	if err := response.WriteAsJson(content); err != nil {
		base.Logger().Error("failed to write json", zap.Error(err))
	}
}

func (s *Server) auth(request *restful.Request, response *restful.Response) bool {
	if s.Config.Server.APIKey == "" {
		return true
	}
	apikey := request.HeaderParameter("X-API-Key")
	if apikey == s.Config.Server.APIKey {
		return true
	}
	base.Logger().Error("unauthorized",
		zap.String("api_key", s.Config.Server.APIKey),
		zap.String("X-API-Key", apikey))
	if err := response.WriteError(401, fmt.Errorf("unauthorized")); err != nil {
		base.Logger().Error("failed to write error", zap.Error(err))
	}
	return false
}
