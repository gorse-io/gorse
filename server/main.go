// Copyright 2020 Zhenghao Zhang
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
	"github.com/zhenghaoz/gorse/database"
	"log"
	"net/http"
	"strconv"
)

type Instance struct {
	DB       *database.Database
	Config   *config.Config
	MetaData *toml.MetaData
}

func Main(instance *Instance) {
	// Create a web instance
	ws := new(restful.WebService)
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)

	// Get recommends
	ws.Route(ws.GET("/recommends/{user-id}").To(instance.getRecommends).
		Doc("get the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")).
		Param(ws.FormParameter("number", "the number of recommendations").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]database.RecommendedItem{}))
	ws.Route(ws.GET("/consume/{user-id}").To(instance.consumeRecommends).
		Doc("consume the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")).
		Param(ws.FormParameter("number", "the number of recommendations").DataType("int")).
		Writes([]database.RecommendedItem{}))
	// Get popular items
	ws.Route(ws.GET("/popular").To(instance.getPopular).
		Doc("get popular items").
		Param(ws.FormParameter("number", "the number of popular items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]database.RecommendedItem{}))
	ws.Route(ws.GET("/popular/{label}").To(instance.getLabelPopular).
		Doc("get popular items by label").
		Param(ws.FormParameter("number", "the number of popular items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]database.RecommendedItem{}))
	// Get latest items
	ws.Route(ws.GET("/latest").To(instance.getLatest).
		Doc("get latest items").
		Param(ws.FormParameter("number", "the number of latest items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]database.RecommendedItem{}))
	ws.Route(ws.GET("/latest/{label}").To(instance.getLabelLatest).
		Doc("get latest items").
		Param(ws.FormParameter("number", "the number of latest items").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]database.RecommendedItem{}))
	// Get neighbors
	ws.Route(ws.GET("/neighbors/{item-id}").To(instance.getNeighbors).
		Doc("get neighbors of a item").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")).
		Param(ws.FormParameter("number", "the number of neighbors").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]database.RecommendedItem{}))

	// Put items
	ws.Route(ws.PUT("/items").To(instance.putItems).
		Doc("put items").
		Reads([]Item{}))
	// Put feedback
	ws.Route(ws.PUT("/feedback").To(instance.putFeedback).
		Doc("put feedback").
		Reads(database.Feedback{}))
	// Get items
	ws.Route(ws.GET("/items").To(instance.getItems).
		Doc("get items").
		Param(ws.FormParameter("number", "the number of neighbors").DataType("int")).
		Param(ws.FormParameter("offset", "the offset of list").DataType("int")).
		Writes([]database.Item{}))
	// Get item
	ws.Route(ws.GET("/item/{item-id}").To(instance.getItem).
		Doc("get a item").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")).
		Writes(database.Item{}))
	// Get items by label
	ws.Route(ws.GET("/item/label/{label}").To(instance.getItemsByLabel).
		Doc(("get items by label")).
		Param(ws.PathParameter("label", "label").DataType("string")).
		Writes([]database.Item{}))

	ws.Route(ws.GET("/status").To(instance.getStatus).Writes(Status{}))

	// Start web instance
	restful.DefaultContainer.Add(ws)

	log.Printf("start a cmd at %v\n", fmt.Sprintf("%s:%d", instance.Config.Server.Host, instance.Config.Server.Port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", instance.Config.Server.Host, instance.Config.Server.Port), nil))
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

func (instance *Instance) status() (Status, error) {
	status := Status{}
	var err error
	// Get number of feedback
	if status.FeedbackCount, err = instance.DB.CountFeedback(); err != nil {
		return Status{}, err
	}
	// Get number of items
	if status.ItemCount, err = instance.DB.CountItems(); err != nil {
		return Status{}, err
	}
	// Get number of users
	if status.UserCount, err = instance.DB.CountUsers(); err != nil {
		return Status{}, err
	}
	// Get number of ignored
	if status.IgnoreCount, err = instance.DB.CountIgnore(); err != nil {
		return Status{}, err
	}
	if status.LastFitTime, err = instance.DB.GetString(database.LastFitTime); err != nil {
		return Status{}, err
	}
	if status.LastUpdateTime, err = instance.DB.GetString(database.LastUpdateTime); err != nil {
		return Status{}, err
	}
	if status.LastUpdateFeedbackCount, err = instance.DB.GetInt(database.LastUpdateFeedbackCount); err != nil {
		return Status{}, err
	}
	if status.LastUpdateIgnoreCount, err = instance.DB.GetInt(database.LastUpdateIgnoreCount); err != nil {
		return Status{}, err
	}
	return status, nil
}

func (instance *Instance) getStatus(request *restful.Request, response *restful.Response) {
	status, err := instance.status()
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
func (instance *Instance) getPopular(request *restful.Request, response *restful.Response) {
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
	items, err := instance.DB.GetPop("", number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

func (instance *Instance) getLabelPopular(request *restful.Request, response *restful.Response) {
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
	items, err := instance.DB.GetPop(label, number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

func (instance *Instance) getLatest(request *restful.Request, response *restful.Response) {
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
	items, err := instance.DB.GetLatest("", number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

func (instance *Instance) getLabelLatest(request *restful.Request, response *restful.Response) {
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
	items, err := instance.DB.GetLatest(label, number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

// getNeighbors gets neighbors of a item from database.
func (instance *Instance) getNeighbors(request *restful.Request, response *restful.Response) {
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
	items, err := instance.DB.GetNeighbors(itemId, number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

// getRecommends gets cached recommended items from database.
func (instance *Instance) getRecommends(request *restful.Request, response *restful.Response) {
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
	items, err := instance.DB.GetRecommend(userId, number, offset)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

func (instance *Instance) consumeRecommends(request *restful.Request, response *restful.Response) {
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
	items, err := instance.DB.ConsumeRecommends(userId, number)
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
func (instance *Instance) putItems(request *restful.Request, response *restful.Response) {
	// Add ratings
	temp := new([]Item)
	if err := request.ReadEntity(temp); err != nil {
		badRequest(response, err)
		return
	}
	// Parse timestamp
	var err error
	items := make([]database.Item, len(*temp))
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
	change.FeedbackBefore, err = instance.DB.CountFeedback()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.ItemsBefore, err = instance.DB.CountItems()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.UsersBefore, err = instance.DB.CountUsers()
	if err != nil {
		badRequest(response, err)
		return
	}
	// Insert items
	for _, item := range items {
		err = instance.DB.InsertItem(database.Item{ItemId: item.ItemId, Timestamp: item.Timestamp, Labels: item.Labels}, true)
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	change.FeedbackAfter, err = instance.DB.CountFeedback()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.ItemsAfter, err = instance.DB.CountItems()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.UsersAfter, err = instance.DB.CountUsers()
	if err != nil {
		badRequest(response, err)
		return
	}
	json(response, change)
}

func (instance *Instance) getItems(request *restful.Request, response *restful.Response) {
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
	items, err := instance.DB.GetItems(number, offset)
	if err != nil {
		internalServerError(response, err)
	}
	json(response, items)
}

func (instance *Instance) getItem(request *restful.Request, response *restful.Response) {
	// Get item id
	itemId := request.PathParameter("item-id")
	// Get item
	item, err := instance.DB.GetItem(itemId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, item)
}

// putFeedback puts new ratings into the database.
func (instance *Instance) putFeedback(request *restful.Request, response *restful.Response) {
	// Add ratings
	ratings := new([]database.Feedback)
	if err := request.ReadEntity(ratings); err != nil {
		badRequest(response, err)
		return
	}
	var err error
	change := Change{}
	// Get status before change
	change.FeedbackBefore, err = instance.DB.CountFeedback()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.ItemsBefore, err = instance.DB.CountItems()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.UsersBefore, err = instance.DB.CountUsers()
	if err != nil {
		badRequest(response, err)
		return
	}
	// Insert feedback
	for _, feedback := range *ratings {
		err = instance.DB.InsertFeedback(feedback)
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	// Get status after change
	change.FeedbackAfter, err = instance.DB.CountFeedback()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.ItemsAfter, err = instance.DB.CountItems()
	if err != nil {
		badRequest(response, err)
		return
	}
	change.UsersAfter, err = instance.DB.CountUsers()
	if err != nil {
		badRequest(response, err)
		return
	}
	json(response, change)
}

func (instance *Instance) getItemsByLabel(request *restful.Request, response *restful.Response) {
	// Get label
	label := request.PathParameter("label")
	// Get item
	items, err := instance.DB.GetItemsByLabel(label)
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
