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
	"github.com/araddon/dateparse"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	restful "github.com/emicklei/go-restful/v3"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/rank"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
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

	RankModel        rank.FactorizationMachine
	RankModelMutex   sync.RWMutex
	RankModelVersion int64

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

	s.Sync()

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

func (s *Server) Sync() {
	ctx := context.Background()

	// pull model version
	log.Infof("server: check model version")
	modelVersion, err := s.MasterClient.GetRankModelVersion(ctx, &protocol.Void{})
	if err != nil {
		log.Fatal("server: failed to check model version (%v)", err)
	}

	// pull model
	if modelVersion.Version != s.RankModelVersion {
		log.Infof("server: sync model")
		modelData, err := s.MasterClient.GetRankModel(ctx, &protocol.Void{}, grpc.MaxCallRecvMsgSize(10e9))
		if err != nil {
			log.Fatal("server: failed to sync model (%v)", err)
		}
		nextModel, err := rank.DecodeModel(modelData.Model)
		if err != nil {
			log.Fatal("server: failed to decode model (%v)", err)
		}
		s.RankModelMutex.Lock()
		defer s.RankModelMutex.Unlock()
		s.RankModel = nextModel
		s.RankModelVersion = modelData.Version
	}
}

func (s *Server) Register() {
	for {
		if _, err := s.MasterClient.RegisterServer(context.Background(), &protocol.Void{}); err != nil {
			log.Fatal("server:", err)
		}
		time.Sleep(time.Duration(s.Config.Common.ClusterMetaTimeout/2) * time.Second)
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
		Reads(data.User{}))
	// Get a user
	ws.Route(ws.GET("/user/{user-id}").To(s.getUser).
		Doc("Get a user.").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes(data.User{}))
	// Insert users
	ws.Route(ws.POST("/users").To(s.insertUsers).
		Doc("Insert users.").
		Reads([]data.User{}))
	// Get users
	ws.Route(ws.GET("/users").To(s.getUsers).
		Doc("Get users.").
		Param(ws.QueryParameter("cursor", "cursor of iteration").DataType("string")).
		Writes(UserIterator{}))
	// Delete a user
	ws.Route(ws.DELETE("/user/{user-id}").To(s.deleteUser).
		Doc("Delete a user.").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes(Success{}))

	// Insert an item
	ws.Route(ws.POST("/item").To(s.insertItem).
		Doc("Insert an item.").
		Reads(Item{}))
	// Get items
	ws.Route(ws.GET("/items").To(s.getItems).
		Doc("Get items.").
		Param(ws.FormParameter("n", "the number of neighbors").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes(ItemIterator{}))
	// Get item
	ws.Route(ws.GET("/item/{item-id}").To(s.getItem).
		Doc("Get a item.").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")).
		Writes(data.Item{}))
	// Insert items
	ws.Route(ws.POST("/items").To(s.insertItems).
		Doc("Insert items.").
		Reads([]Item{}))
	// Delete item
	ws.Route(ws.DELETE("/item/{item-id}").To(s.deleteItem).
		Doc("Delete a item.").
		Param(ws.PathParameter("item-id", "identified of the item").DataType("string")).
		Writes(Success{}))

	// Insert feedback
	ws.Route(ws.POST("/feedback").To(s.insertFeedback).
		Doc("Insert feedback.").
		Reads(data.Feedback{}))
	// Get feedback
	ws.Route(ws.GET("/feedback").To(s.getFeedback).
		Doc("Get feedback.").
		Writes(FeedbackIterator{}))
	// Get feedback by user id
	ws.Route(ws.GET("/user/{user-id}/feedback/{feedback-type}").To(s.getFeedbackByUser).
		Doc("Get feedback by user id.").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("string")).
		Writes([]data.Feedback{}))
	// Get feedback by item-id
	ws.Route(ws.GET("/item/{item-id}/feedback/{feedback-type}").To(s.getFeedbackByItem).
		Doc("Get feedback by item id").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.PathParameter("feedback-type", "feedback type").DataType("strung")).
		Writes([]string{}))

	/* Interaction with cache store */

	// Get matched items by user id
	ws.Route(ws.GET("/user/{user-id}/match").To(s.getRecommendCache).
		Doc("get the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.FormParameter("n", "the number of recommendations").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]string{}))
	// Get popular items
	ws.Route(ws.GET("/popular").To(s.getPopular).
		Doc("get popular items").
		Param(ws.FormParameter("n", "the number of popular items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]string{}))
	// Get latest items
	ws.Route(ws.GET("/latest").To(s.getLatest).
		Doc("get latest items").
		Param(ws.FormParameter("n", "the number of latest items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]string{}))
	// Get neighbors
	ws.Route(ws.GET("/item/{item-id}/neighbors").To(s.getNeighbors).
		Doc("get neighbors of a item").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.FormParameter("n", "the number of neighbors").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]string{}))

	/* Online recommendation */

	ws.Route(ws.GET("/recommend/{user-id}").To(s.getRecommend)).
		Doc("Get recommendation for user.").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.FormParameter("n", "the number of neighbors").DataType("int"))

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

// getPopular gets popular items from database.
func (s *Server) getPopular(request *restful.Request, response *restful.Response) {
	var n, offset int
	var err error
	if n, err = parseInt(request, "n", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get the popular list
	items, err := s.CacheStore.GetList(cache.PopularItems, "", n, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	ok(response, items)
}

func (s *Server) getLatest(request *restful.Request, response *restful.Response) {
	var n, offset int
	var err error
	if n, err = parseInt(request, "n", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get the popular list
	items, err := s.CacheStore.GetList(cache.LatestItems, "", n, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	ok(response, items)
}

// get feedback by item-id
func (s *Server) getFeedbackByItem(request *restful.Request, response *restful.Response) {
	feedbackType := request.PathParameter("feedback-type")
	itemId := request.PathParameter("item-id")
	feedback, err := s.DataStore.GetItemFeedback(feedbackType, itemId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, feedback)
}

// getNeighbors gets neighbors of a item from database.
func (s *Server) getNeighbors(request *restful.Request, response *restful.Response) {
	// Get item id
	itemId := request.PathParameter("item-id")
	// Get the number and offset
	var n, offset int
	var err error
	if n, err = parseInt(request, "n", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get recommended items
	items, err := s.CacheStore.GetList(cache.SimilarItems, itemId, n, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	ok(response, items)
}

// getRecommendCache gets cached recommended items from database.
func (s *Server) getRecommendCache(request *restful.Request, response *restful.Response) {
	// Get user id
	userId := request.PathParameter("user-id")
	// Get the number and offset
	var n, offset int
	var err error
	if n, err = parseInt(request, "n", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get recommended items
	items, err := s.CacheStore.GetList(cache.MatchedItems, userId, n, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	ok(response, items)
}

func (s *Server) getRecommend(request *restful.Request, response *restful.Response) {
	start := time.Now()
	userId := request.PathParameter("user-id")
	n, err := parseInt(request, "n", s.Config.Server.DefaultReturnNumber)
	if err != nil {
		badRequest(response, err)
		return
	}
	// load feedback
	userFeedback, err := s.DataStore.GetUserFeedback("", userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	excludeSet := base.NewStringSet()
	for _, feedback := range userFeedback {
		excludeSet.Add(feedback.ItemId)
	}
	// load popular
	candidateItems := make([]string, 0)
	popularItems, err := s.CacheStore.GetList(cache.PopularItems, "", s.Config.Online.NumPopular, 0)
	for _, itemId := range popularItems {
		if !excludeSet.Contain(itemId) {
			candidateItems = append(candidateItems, itemId)
			excludeSet.Add(itemId)
		}
	}
	// load latest
	latestItems, err := s.CacheStore.GetList(cache.LatestItems, "", s.Config.Online.NumLatest, 0)
	for _, itemId := range latestItems {
		if !excludeSet.Contain(itemId) {
			candidateItems = append(candidateItems, itemId)
			excludeSet.Add(itemId)
		}
	}
	// load matched
	matchedItems, err := s.CacheStore.GetList(cache.MatchedItems, userId, s.Config.Online.NumMatch, 0)
	for _, itemId := range matchedItems {
		if !excludeSet.Contain(itemId) {
			candidateItems = append(candidateItems, itemId)
			excludeSet.Add(itemId)
		}
	}
	log.Infof("server: recommend from (#candidate = %v)", len(candidateItems))
	// collect item features
	candidateFeaturedItems := make([]data.Item, len(candidateItems))
	for i, itemId := range candidateItems {
		candidateFeaturedItems[i], err = s.DataStore.GetItem(itemId)
		if err != nil {
			internalServerError(response, err)
		}
	}
	// online predict
	recItems := base.NewTopKStringFilter(n)
	for _, item := range candidateFeaturedItems {
		s.RankModelMutex.RLock()
		m := s.RankModel
		s.RankModelMutex.RUnlock()
		recItems.Push(item.ItemId, m.Predict(userId, item.ItemId, item.Labels))
	}
	result, _ := recItems.PopAll()
	spent := time.Since(start)
	log.Infof("server: complete recommendation (time = %v)", spent)
	ok(response, result)
}

type Item struct {
	ItemId    string
	Timestamp string
	Labels    []string
}

type Success struct {
	RowAffected int
}

func (s *Server) insertUser(request *restful.Request, response *restful.Response) {
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
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.Server.DefaultReturnNumber)
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
	// get user-id and put into temp
	userId := request.PathParameter("user-id")
	if err := s.DataStore.DeleteUser(userId); err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, Success{RowAffected: 1})
}

// get feedback by user-id
func (s *Server) getFeedbackByUser(request *restful.Request, response *restful.Response) {
	feedbackType := request.PathParameter("feedback-type")
	userId := request.PathParameter("user-id")
	feedback, err := s.DataStore.GetUserFeedback(feedbackType, userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, feedback)
}

// putItems puts items into the database.
func (s *Server) insertItems(request *restful.Request, response *restful.Response) {
	// Add ratings
	temp := new([]Item)
	if err := request.ReadEntity(temp); err != nil {
		badRequest(response, err)
		return
	}
	// Parse timestamp
	var err error
	items := make([]data.Item, len(*temp))
	for i, v := range *temp {
		items[i].ItemId = v.ItemId
		items[i].Timestamp, err = dateparse.ParseAny(v.Timestamp)
		items[i].Labels = v.Labels
		if err != nil {
			badRequest(response, err)
		}
	}
	// Get status before change
	if err != nil {
		internalServerError(response, err)
		return
	}

	// Insert items
	var count int
	for _, item := range items {
		err = s.DataStore.InsertItem(data.Item{ItemId: item.ItemId, Timestamp: item.Timestamp, Labels: item.Labels})
		count++
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	ok(response, Success{RowAffected: count})
}
func (s *Server) insertItem(request *restful.Request, response *restful.Response) {
	temp := new(data.Item)
	if err := request.ReadEntity(temp); err != nil {
		badRequest(response, err)
		return
	}
	if err := s.DataStore.InsertItem(*temp); err != nil {
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
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.Server.DefaultReturnNumber)
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
	itemId := request.PathParameter("item-id")
	if err := s.DataStore.DeleteItem(itemId); err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, Success{RowAffected: 1})
}

// putFeedback puts new ratings into the database.
func (s *Server) insertFeedback(request *restful.Request, response *restful.Response) {
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
			s.Config.Common.AutoInsertUser,
			s.Config.Common.AutoInsertItem)
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
	// Parse parameters
	feedbackType := request.QueryParameter("feedback-type")
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.Server.DefaultReturnNumber)
	if err != nil {
		badRequest(response, err)
		return
	}
	cursor, feedback, err := s.DataStore.GetFeedback(feedbackType, cursor, n)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, FeedbackIterator{Cursor: cursor, Feedback: feedback})
}

func badRequest(response *restful.Response, err error) {
	log.Error("server:", err)
	if err = response.WriteError(400, err); err != nil {
		log.Error("server:", err)
	}
}

func internalServerError(response *restful.Response, err error) {
	log.Error("server:", err)
	if err = response.WriteError(500, err); err != nil {
		log.Error("server:", err)
	}
}

// json sends the content as JSON to the client.
func ok(response *restful.Response, content interface{}) {
	if err := response.WriteAsJson(content); err != nil {
		log.Error("server:", err)
	}
}
