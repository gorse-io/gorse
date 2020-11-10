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

func (s *Server) Serve() {
	// Create a server
	ws := new(restful.WebService)
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)

	// Get recommends
	ws.Route(ws.GET("/recommends/{user-id}").To(s.getRecommends).
		Doc("get the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")).
		Param(ws.FormParameter("number", "the number of recommendations").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))
	ws.Route(ws.GET("/consume/{user-id}").To(s.consumeRecommends).
		Doc("consume the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")).
		Param(ws.FormParameter("number", "the number of recommendations").DataType("int")).
		Writes([]storage.RecommendedItem{}))
	// Get popular items
	ws.Route(ws.GET("/popular").To(s.getPopular).
		Doc("get popular items").
		Param(ws.FormParameter("number", "the number of popular items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))
	ws.Route(ws.GET("/popular/{label}").To(s.getLabelPopular).
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
	// Get neighbors
	ws.Route(ws.GET("/neighbors/{item-id}").To(s.getNeighbors).
		Doc("get neighbors of a item").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")).
		Param(ws.FormParameter("number", "the number of neighbors").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]storage.RecommendedItem{}))

	// Put items
	ws.Route(ws.PUT("/items").To(s.putItems).
		Doc("put items").
		Reads([]Item{}))
	// Put feedback
	ws.Route(ws.PUT("/feedback").To(s.putFeedback).
		Doc("put feedback").
		Reads(storage.Feedback{}))
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
	// Get items by label
	ws.Route(ws.GET("/item/label/{label}").To(s.getItemsByLabel).
		Doc(("get items by label")).
		Param(ws.PathParameter("label", "label").DataType("string")).
		Writes([]storage.Item{}))

	ws.Route(ws.GET("/status").To(s.getStatus).Writes(Status{}))

	// Start web s
	restful.DefaultContainer.Add(ws)

	log.Printf("start a cmd at %v\n", fmt.Sprintf("%s:%d", s.Config.Server.Host, s.Config.Server.Port))
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

func (s *Server) status() (Status, error) {
	status := Status{}
	var err error
	// Get number of feedback
	if status.FeedbackCount, err = s.DB.CountFeedback(); err != nil {
		return Status{}, err
	}
	// Get number of items
	if status.ItemCount, err = s.DB.CountItems(); err != nil {
		return Status{}, err
	}
	// Get number of users
	if status.UserCount, err = s.DB.CountUsers(); err != nil {
		return Status{}, err
	}
	// Get number of ignored
	if status.LastFitTime, err = s.DB.GetString(storage.LastFitTime); err != nil {
		return Status{}, err
	}
	if status.LastUpdateTime, err = s.DB.GetString(storage.LastUpdateTime); err != nil {
		return Status{}, err
	}
	if status.LastUpdateFeedbackCount, err = s.DB.GetInt(storage.LastUpdateFeedbackCount); err != nil {
		return Status{}, err
	}
	if status.LastUpdateIgnoreCount, err = s.DB.GetInt(storage.LastUpdateIgnoreCount); err != nil {
		return Status{}, err
	}
	return status, nil
}

func (s *Server) getStatus(request *restful.Request, response *restful.Response) {
	status, err := s.status()
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, status)
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

func (s *Server) getLabelPopular(request *restful.Request, response *restful.Response) {
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
	// Get user id
	userId := request.PathParameter("user-id")
	// Get the number and offset
	var number int
	var err error
	if number, err = parseInt(request, "number", 10); err != nil {
		badRequest(response, err)
		return
	}
	// Get recommended items
	items, err := s.DB.ConsumeRecommends(userId, number)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
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

// putItems puts items into the database.
func (s *Server) putItems(request *restful.Request, response *restful.Response) {
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
		if err != nil {
			badRequest(response, err)
		}
	}
	change := Change{}
	// Get status before change
	if err != nil {
		internalServerError(response, err)
		return
	}
	change.FeedbackBefore, err = s.DB.CountFeedback()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.ItemsBefore, err = s.DB.CountItems()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.UsersBefore, err = s.DB.CountUsers()
	if err != nil {
		badRequest(response, err)
		return
	}
	// Insert items
	for _, item := range items {
		err = s.DB.InsertItem(storage.Item{ItemId: item.ItemId, Timestamp: item.Timestamp, Labels: item.Labels})
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	change.FeedbackAfter, err = s.DB.CountFeedback()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.ItemsAfter, err = s.DB.CountItems()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.UsersAfter, err = s.DB.CountUsers()
	if err != nil {
		badRequest(response, err)
		return
	}
	json(response, change)
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

// putFeedback puts new ratings into the database.
func (s *Server) putFeedback(request *restful.Request, response *restful.Response) {
	// Add ratings
	ratings := new([]storage.Feedback)
	if err := request.ReadEntity(ratings); err != nil {
		badRequest(response, err)
		return
	}
	var err error
	change := Change{}
	// Get status before change
	change.FeedbackBefore, err = s.DB.CountFeedback()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.ItemsBefore, err = s.DB.CountItems()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.UsersBefore, err = s.DB.CountUsers()
	if err != nil {
		badRequest(response, err)
		return
	}
	// Insert feedback
	for _, feedback := range *ratings {
		err = s.DB.InsertFeedback(feedback)
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	// Get status after change
	change.FeedbackAfter, err = s.DB.CountFeedback()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.ItemsAfter, err = s.DB.CountItems()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.UsersAfter, err = s.DB.CountUsers()
	if err != nil {
		badRequest(response, err)
		return
	}
	json(response, change)
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
