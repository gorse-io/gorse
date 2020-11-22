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
	"github.com/BurntSushi/toml"
	"github.com/araddon/dateparse"
	"github.com/emicklei/go-restful"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage"
	"log"
	"net/http"
	"strconv"
)

type Server struct {
	DB       storage.Database
	Config   *config.Config
	MetaData *toml.MetaData
}

func (s *Server) SetupWebService() {
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
		Writes([]storage.User{}))
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
	ws.Route(ws.GET("/user/{user-id}/recommend").To(s.getRecommends).
		Doc("get the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.FormParameter("number", "the number of recommendations").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))

	ws.Route(ws.GET("/consume/{user-id}").To(s.consumeRecommends).
		Doc("consume the top list for a user").
		Param(ws.FormParameter("number", "the number of recommendations").DataType("int")).
		Writes([]storage.RecommendedItem{}))

	// Get popular items
	ws.Route(ws.GET("/popular").To(s.getPopular).
		Doc("get popular items").
		Param(ws.FormParameter("number", "the number of popular items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))
	ws.Route(ws.GET("/popular/{label}").To(s.getPopularByLabel).
		Doc("get popular items by label").
		Param(ws.FormParameter("number", "the number of popular items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))

	// Get latest items
	ws.Route(ws.GET("/latest").To(s.getLatest).
		Doc("get latest items").
		Param(ws.FormParameter("number", "the number of latest items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))
	ws.Route(ws.GET("/latest/{label}").To(s.getLabelLatest).
		Doc("get latest items").
		Param(ws.FormParameter("number", "the number of latest items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))

	// Post item
	ws.Route(ws.POST("/item").To(s.postItem).
		Doc("post an item").
		Reads(Item{}))
	// Get items
	ws.Route(ws.GET("/items").To(s.getItems).
		Doc("get items").
		Param(ws.FormParameter("number", "the number of neighbors").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.Item{}))
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
		Param(ws.FormParameter("number", "the number of neighbors").DataType("int")).
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
		Writes([]string{}))

	// Post feedback
	ws.Route(ws.POST("/feedback").To(s.postFeedback).
		Doc("put feedback").
		Reads(storage.Feedback{}))
	// Get feedback
	ws.Route(ws.GET("/feedback").To(s.getFeedback).
		Doc("get feedback").
		Writes([]storage.Feedback{}))

	// Insert Ignore
	ws.Route(ws.POST("/user/{user-id}/ignore").
		To(s.postIgnore).
		Doc("insert ignore").
		Reads(Success{}))
	// Count ignore
	//ws.Route(ws.GET("/user/{user-id}/countIgnore"))

	// Start web s
	restful.DefaultContainer.Add(ws)
}

func (s *Server) Serve() {
	s.SetupWebService()
	log.Printf("start gorse server at %v\n", fmt.Sprintf("%s:%d", s.Config.Server.Host, s.Config.Server.Port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", s.Config.Server.Host, s.Config.Server.Port), nil))
}

// Status contains information about engine.
type Status struct {
	FeedbackCount           int // number of feedback
	ItemCount               int // number of items
	UserCount               int // number of users
	IgnoreCount             int // nunber of ignored
	LastFitFeedbackCount    int // number of committed feedback
	LastUpdateFeedbackCount int
	LastUpdateIgnoreCount   int
	LastFitTime             string
	LastUpdateTime          string
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
	var number, offset int
	var err error
	if number, err = parseInt(request, "number", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get the popular list
	items, err := s.DB.GetPop("", number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

func (s *Server) getPopularByLabel(request *restful.Request, response *restful.Response) {
	label := request.PathParameter("label")
	var number, offset int
	var err error
	if number, err = parseInt(request, "number", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get the popular list
	items, err := s.DB.GetPop(label, number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

func (s *Server) getLatest(request *restful.Request, response *restful.Response) {
	var number, offset int
	var err error
	if number, err = parseInt(request, "number", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get the popular list
	items, err := s.DB.GetLatest("", number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

func (s *Server) getLabelLatest(request *restful.Request, response *restful.Response) {
	label := request.PathParameter("label")
	var number, offset int
	var err error
	if number, err = parseInt(request, "number", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get the popular list
	items, err := s.DB.GetLatest(label, number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

// get feedback by item-id
func (s *Server) getFeedbackByItem(request *restful.Request, response *restful.Response) {
	itemId := request.PathParameter("item-id")

	feedback, err := s.DB.GetItemFeedback(itemId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, feedback)
}

// getNeighbors gets neighbors of a item from database.
func (s *Server) getNeighbors(request *restful.Request, response *restful.Response) {
	// Get item id
	itemId := request.PathParameter("item-id")
	// Get the number and offset
	var number, offset int
	var err error
	if number, err = parseInt(request, "number", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get recommended items
	items, err := s.DB.GetNeighbors(itemId, number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

// getRecommends gets cached recommended items from database.
func (s *Server) getRecommends(request *restful.Request, response *restful.Response) {
	// Get user id
	userId := request.PathParameter("user-id")
	// Get the number and offset
	var number, offset int
	var err error
	if number, err = parseInt(request, "number", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	// Get recommended items
	items, err := s.DB.GetRecommend(userId, number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

func (s *Server) consumeRecommends(request *restful.Request, response *restful.Response) {
	// TODO
	badRequest(response, fmt.Errorf("not implemented"))
}

// Change contains information of changes after insert.
type Change struct {
	ItemsBefore    int // number of items before change
	ItemsAfter     int // number of items after change
	UsersBefore    int // number of users before change
	UsersAfter     int // number of user after change
	FeedbackBefore int // number of feedback before change
	FeedbackAfter  int // number of feedback after change
}

type Item struct {
	ItemId     string
	Popularity float64
	Timestamp  string
	Labels     []string
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
	json(response, Success{RowAffected: 1})
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
	json(response, user)
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
	json(response, Success{RowAffected: count})
}

func (s *Server) getUsers(request *restful.Request, response *restful.Response) {
	// get all users
	users, err := s.DB.GetUsers()
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, users)
}

// delete a user by user-id
func (s *Server) deleteUser(request *restful.Request, response *restful.Response) {
	// get user-id and put into temp
	userId := request.PathParameter("user-id")
	if err := s.DB.DeleteUser(userId); err != nil {
		internalServerError(response, err)
		return
	}
	json(response, Success{RowAffected: 1})
}

// get feedback by user-id
func (s *Server) getFeedbackByUser(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")

	feedback, err := s.DB.GetUserFeedback(userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, feedback)
}

// get ignorance by user-id
func (s *Server) getIgnoreByUser(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")
	ignore, err := s.DB.GetUserIgnore(userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, ignore)
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
	json(response, Success{RowAffected: count})
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
	json(response, Success{RowAffected: 1})
}

func (s *Server) getItems(request *restful.Request, response *restful.Response) {
	var number, offset int
	var err error
	if number, err = parseInt(request, "number", 10); err != nil {
		badRequest(response, err)
		return
	}
	if offset, err = parseInt(request, "offset", 0); err != nil {
		badRequest(response, err)
		return
	}
	items, err := s.DB.GetItems(number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, items)
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
	json(response, item)
}

func (s *Server) deleteItem(request *restful.Request, response *restful.Response) {
	itemId := request.PathParameter("item-id")

	if err := s.DB.DeleteItem(itemId); err != nil {
		internalServerError(response, err)
		return
	}
	json(response, Success{RowAffected: 1})
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
	json(response, Success{RowAffected: count})
}

// Get feedback
func (s *Server) getFeedback(request *restful.Request, response *restful.Response) {
	feedback, err := s.DB.GetFeedback()
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, feedback)
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
	json(response, items)
}
func (s *Server) getLabels(request *restful.Request, response *restful.Response) {
	// Get all labels
	labels, err := s.DB.GetLabels()
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, labels)
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
	json(response, Success{RowAffected: 1})
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
func json(response *restful.Response, content interface{}) {
	if err := response.WriteAsJson(content); err != nil {
		log.Println(err)
	}
}
