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
	"github.com/araddon/dateparse"
	"github.com/emicklei/go-restful"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage"
	"log"
	"net/http"
	"strconv"
)

type Server struct {
	DB     storage.Database
	Config *config.ServerConfig
}

func NewServer(db storage.Database, config *config.ServerConfig) *Server {
	return &Server{
		DB:     db,
		Config: config.LoadDefaultIfNil(),
	}
}

func (s *Server) CreateWebService() *restful.WebService {
	// Create a server
	ws := new(restful.WebService)
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)

	// Post user
	ws.Route(ws.POST("/user").To(s.postUser).
		Doc("post user").
		Reads(storage.User{}))
	// Get a user
	ws.Route(ws.GET("/user/{user-id}").To(s.getUser).
		Doc("get a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes(storage.User{}))
	// Post users
	ws.Route(ws.POST("/users").To(s.postUsers).
		Doc("").
		Reads([]storage.User{}))
	// Get users
	ws.Route(ws.GET("/users").To(s.getUsers).
		Doc("get users").
		Writes(UserIterator{}))
	// Delete a user
	ws.Route(ws.DELETE("/user/{user-id}").To(s.deleteUser).
		Doc("delete a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes(Success{}))
	// Get feedback by user-id
	ws.Route(ws.GET("/user/{user-id}/feedback").To(s.getFeedbackByUser).
		Doc("get feedback by user-id").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes([]storage.Feedback{}))
	// Get ignorance by user-id
	ws.Route(ws.GET("/user/{user-id}/ignore").To(s.getIgnoreByUser).
		Doc("get ignorance by user-id").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes([]string{}))
	// Get recommends by user-id
	ws.Route(ws.GET("/user/{user-id}/recommend/cache").To(s.getRecommendCache).
		Doc("get the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.FormParameter("n", "the number of recommendations").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))

	ws.Route(ws.GET("/recommend/{user-id}").To(s.getRecommend).
		Doc("consume the top list for a user").
		Param(ws.FormParameter("n", "the number of recommendations").DataType("int")).
		Writes([]storage.RecommendedItem{}))

	// Get popular items
	ws.Route(ws.GET("/popular").To(s.getPopular).
		Doc("get popular items").
		Param(ws.FormParameter("n", "the number of popular items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))
	ws.Route(ws.GET("/popular/{label}").To(s.getPopularByLabel).
		Doc("get popular items by label").
		Param(ws.FormParameter("n", "the number of popular items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))

	// Get latest items
	ws.Route(ws.GET("/latest").To(s.getLatest).
		Doc("get latest items").
		Param(ws.FormParameter("n", "the number of latest items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))
	ws.Route(ws.GET("/latest/{label}").To(s.getLabelLatest).
		Doc("get latest items").
		Param(ws.FormParameter("n", "the number of latest items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))

	// Post item
	ws.Route(ws.POST("/item").To(s.postItem).
		Doc("post an item").
		Reads(Item{}))
	// Get items
	ws.Route(ws.GET("/items").To(s.getItems).
		Doc("get items").
		Param(ws.FormParameter("n", "the number of neighbors").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes(ItemIterator{}))
	// Get item
	ws.Route(ws.GET("/item/{item-id}").To(s.getItem).
		Doc("get a item").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")).
		Writes(storage.Item{}))
	// Post items
	ws.Route(ws.POST("/items").To(s.postItems).
		Doc("post items").
		Reads([]Item{}))
	// Delete item
	ws.Route(ws.DELETE("/item/{item-id}").To(s.deleteItem).
		Doc("delete a item").
		Param(ws.PathParameter("item-id", "identified of the item").DataType("string")).
		Writes(Success{}))

	// Get feedback by item-id
	ws.Route(ws.GET("/item/{item-id}/feedback").To(s.getFeedbackByItem).
		Doc("get feedback by item-id").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Writes([]storage.RecommendedItem{}))
	// Get neighbors
	ws.Route(ws.GET("/item/{item-id}/neighbors").To(s.getNeighbors).
		Doc("get neighbors of a item").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("string")).
		Param(ws.FormParameter("n", "the number of neighbors").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))

	// Get items by label
	ws.Route(ws.GET("/label/{label}/items").To(s.getItemsByLabel).
		Doc("get items by label").
		Param(ws.PathParameter("label", "label").DataType("string")).
		Writes([]storage.Item{}))

	// Get labels
	ws.Route(ws.GET("/labels").To(s.getLabels).
		Doc("get labels").
		Writes(LabelIterator{}))

	// Post feedback
	ws.Route(ws.POST("/feedback").To(s.postFeedback).
		Doc("put feedback").
		Reads(storage.Feedback{}))
	// Get feedback
	ws.Route(ws.GET("/feedback").To(s.getFeedback).
		Doc("get feedback").
		Writes(FeedbackIterator{}))

	// Insert Ignore
	ws.Route(ws.POST("/user/{user-id}/ignore").
		To(s.postIgnore).
		Doc("insert ignore").
		Reads(Success{}))
	return ws
}

func (s *Server) Serve() {
	ws := s.CreateWebService()
	restful.DefaultContainer.Add(ws)
	log.Printf("start gorse server at %v\n", fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port), nil))
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
	items, err := s.DB.GetPop("", n, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	ok(response, items)
}

func (s *Server) getPopularByLabel(request *restful.Request, response *restful.Response) {
	label := request.PathParameter("label")
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
	items, err := s.DB.GetPop(label, n, offset)
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
	items, err := s.DB.GetLatest("", n, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	ok(response, items)
}

func (s *Server) getLabelLatest(request *restful.Request, response *restful.Response) {
	label := request.PathParameter("label")
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
	items, err := s.DB.GetLatest(label, n, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	ok(response, items)
}

// get feedback by item-id
func (s *Server) getFeedbackByItem(request *restful.Request, response *restful.Response) {
	itemId := request.PathParameter("item-id")

	feedback, err := s.DB.GetItemFeedback(itemId)
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
	items, err := s.DB.GetNeighbors(itemId, n, offset)
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
	items, err := s.DB.GetRecommend(userId, n, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	ok(response, items)
}

func (s *Server) getRecommend(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")
	n, err := parseInt(request, "n", s.Config.DefaultN)
	if err != nil {
		badRequest(response, err)
		return
	}
	consume, err := parseInt(request, "consume", 0)
	if err != nil {
		badRequest(response, err)
		return
	}
	// load ignore and feedback
	ignoreSet := base.NewStringSet()
	ignore, err := s.DB.GetUserIgnore(userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ignoreSet.Add(ignore...)
	feedback, err := s.DB.GetUserFeedback(userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	for _, f := range feedback {
		ignoreSet.Add(f.ItemId)
	}
	// load recommend cache
	recommend, err := s.DB.GetRecommend(userId, 0, 0)
	if err != nil {
		internalServerError(response, err)
		return
	}
	result := make([]storage.RecommendedItem, 0)
	for _, item := range recommend {
		if !ignoreSet.Contain(item.ItemId) {
			result = append(result, item)
		}
	}
	if len(result) > n {
		result = result[:n]
	}
	// consume
	if consume != 0 {
		incIgnores := make([]string, 0, len(result))
		for _, item := range result {
			incIgnores = append(incIgnores, item.ItemId)
		}
		if err := s.DB.InsertUserIgnore(userId, incIgnores); err != nil {
			internalServerError(response, err)
			return
		}
	}
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

func (s *Server) postUser(request *restful.Request, response *restful.Response) {
	temp := storage.User{}
	// get userInfo from request and put into temp
	if err := request.ReadEntity(&temp); err != nil {
		badRequest(response, err)
		return
	}
	if err := s.DB.InsertUser(temp); err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, Success{RowAffected: 1})
}

func (s *Server) getUser(request *restful.Request, response *restful.Response) {
	// get user id
	userId := request.PathParameter("user-id")
	// get user
	user, err := s.DB.GetUser(userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, user)
}

func (s *Server) postUsers(request *restful.Request, response *restful.Response) {
	temp := new([]storage.User)
	// get param from request and put into temp
	if err := request.ReadEntity(temp); err != nil {
		badRequest(response, err)
		return
	}
	var count int
	// range temp and achieve user
	for _, user := range *temp {
		if err := s.DB.InsertUser(user); err != nil {
			internalServerError(response, err)
			return
		}
		count++
	}
	ok(response, Success{RowAffected: count})
}

type UserIterator struct {
	Cursor string
	Users  []storage.User
}

func (s *Server) getUsers(request *restful.Request, response *restful.Response) {
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.DefaultN)
	if err != nil {
		badRequest(response, err)
		return
	}
	// get all users
	cursor, users, err := s.DB.GetUsers(cursor, n)
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
	if err := s.DB.DeleteUser(userId); err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, Success{RowAffected: 1})
}

// get feedback by user-id
func (s *Server) getFeedbackByUser(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")

	feedback, err := s.DB.GetUserFeedback(userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, feedback)
}

// get ignorance by user-id
func (s *Server) getIgnoreByUser(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")
	ignore, err := s.DB.GetUserIgnore(userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, ignore)
}

// putItems puts items into the database.
func (s *Server) postItems(request *restful.Request, response *restful.Response) {
	// Add ratings
	temp := new([]Item)
	if err := request.ReadEntity(temp); err != nil {
		badRequest(response, err)
		return
	}
	// Parse timestamp
	var err error
	items := make([]storage.Item, len(*temp))
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
		err = s.DB.InsertItem(storage.Item{ItemId: item.ItemId, Timestamp: item.Timestamp, Labels: item.Labels})
		count++
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	ok(response, Success{RowAffected: count})
}
func (s *Server) postItem(request *restful.Request, response *restful.Response) {
	temp := new(storage.Item)
	if err := request.ReadEntity(temp); err != nil {
		badRequest(response, err)
		return
	}
	if err := s.DB.InsertItem(*temp); err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, Success{RowAffected: 1})
}

type ItemIterator struct {
	Cursor string
	Items  []storage.Item
}

func (s *Server) getItems(request *restful.Request, response *restful.Response) {
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.DefaultN)
	if err != nil {
		badRequest(response, err)
		return
	}
	cursor, items, err := s.DB.GetItems(cursor, n)
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
	item, err := s.DB.GetItem(itemId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, item)
}

func (s *Server) deleteItem(request *restful.Request, response *restful.Response) {
	itemId := request.PathParameter("item-id")

	if err := s.DB.DeleteItem(itemId); err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, Success{RowAffected: 1})
}

// putFeedback puts new ratings into the database.
func (s *Server) postFeedback(request *restful.Request, response *restful.Response) {
	// Add ratings
	ratings := new([]storage.Feedback)
	if err := request.ReadEntity(ratings); err != nil {
		badRequest(response, err)
		return
	}
	var err error
	// Insert feedback
	var count int
	for _, feedback := range *ratings {
		err = s.DB.InsertFeedback(feedback)
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
	Feedback []storage.Feedback
}

// Get feedback
func (s *Server) getFeedback(request *restful.Request, response *restful.Response) {
	// Parse parameters
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.DefaultN)
	if err != nil {
		badRequest(response, err)
		return
	}
	cursor, feedback, err := s.DB.GetFeedback(cursor, n)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, FeedbackIterator{Cursor: cursor, Feedback: feedback})
}

func (s *Server) getItemsByLabel(request *restful.Request, response *restful.Response) {
	// Get label
	label := request.PathParameter("label")
	// Get item
	items, err := s.DB.GetLabelItems(label)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, items)
}

type LabelIterator struct {
	Cursor string
	Labels []string
}

func (s *Server) getLabels(request *restful.Request, response *restful.Response) {
	cursor := request.QueryParameter("cursor")
	n, err := parseInt(request, "n", s.Config.DefaultN)
	if err != nil {
		badRequest(response, err)
		return
	}
	cursor, labels, err := s.DB.GetLabels(cursor, n)
	if err != nil {
		internalServerError(response, err)
		return
	}
	ok(response, LabelIterator{Cursor: cursor, Labels: labels})
}

func (s *Server) postIgnore(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")
	items := new([]string)
	if err := request.ReadEntity(items); err != nil {
		badRequest(response, err)
	}
	if err := s.DB.InsertUserIgnore(userId, *items); err != nil {
		internalServerError(response, err)
	}
	ok(response, Success{RowAffected: 1})
}

func badRequest(response *restful.Response, err error) {
	log.Println(err)
	if err = response.WriteError(400, err); err != nil {
		log.Println(err)
	}
}

func internalServerError(response *restful.Response, err error) {
	log.Println(err)
	if err = response.WriteError(500, err); err != nil {
		log.Println(err)
	}
}

// json sends the content as JSON to the client.
func ok(response *restful.Response, content interface{}) {
	if err := response.WriteAsJson(content); err != nil {
		log.Println(err)
	}
}
