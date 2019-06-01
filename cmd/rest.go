package cmd

import (
	"fmt"
	"github.com/emicklei/go-restful"
	"github.com/zhenghaoz/gorse/engine"
	"log"
	"net/http"
	"strconv"
)

func serve(config engine.ServerConfig) {
	// Create a web service
	ws := new(restful.WebService)
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	// Get the recommendation list
	ws.Route(ws.GET("/recommends/{user-id}").
		To(getRecommends).
		Doc("get the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")).
		Param(ws.FormParameter("number", "the number of recommendations").DataType("int")))
	// Get user's feedback
	ws.Route(ws.GET("/user/{user-id}").
		To(getUser).
		Doc("get a user's feedback").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")))
	// Popular items
	ws.Route(ws.GET("/popular").To(getPopular).
		Doc("get popular items").
		Param(ws.FormParameter("number", "the number of popular items").DataType("int")))
	// Random items
	ws.Route(ws.GET("/random").To(getRandom).
		Doc("get random items").
		Param(ws.FormParameter("number", "the number of random items").DataType("int")))
	// Neighbors
	ws.Route(ws.GET("/neighbors/{item-id}").To(getNeighbors).
		Doc("get neighbors of a item").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")).
		Param(ws.FormParameter("number", "the number of neighbors").DataType("int")))
	// Add items
	ws.Route(ws.PUT("/items").To(putItems)).
		Doc("put items")
	// Add ratings
	ws.Route(ws.PUT("/feedback").To(putFeedback).
		Doc("put feedback"))
	ws.Route(ws.GET("/status").To(getStatus))
	// Start web service
	restful.DefaultContainer.Add(ws)
	log.Printf("start a server at %v\n", fmt.Sprintf("%s:%d", config.Host, config.Port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", config.Host, config.Port), nil))
}

// Status contains information about engine.
type Status struct {
	FeedbackCount int // number of feedback
	ItemCount     int // number of items
	CommitCount   int // number of committed feedback
}

func getStatus(request *restful.Request, response *restful.Response) {
	status := Status{}
	var err error
	// Get feedback count
	if status.FeedbackCount, err = db.CountFeedback(); err != nil {
		internalServerError(response, err)
	}
	// Get item count
	if status.ItemCount, err = db.CountItems(); err != nil {
		internalServerError(response, err)
	}
	// Get commit count
	var commit string
	if commit, err = db.GetMeta("commit"); err != nil {
		internalServerError(response, err)
	}
	if status.CommitCount, err = strconv.Atoi(commit); len(commit) > 0 && err != nil {
		internalServerError(response, err)
	}
	json(response, status)
}

func getUser(request *restful.Request, response *restful.Response) {
	// Get user id
	paramUserId := request.PathParameter("user-id")
	userId, err := strconv.Atoi(paramUserId)
	if err != nil {
		badRequest(response, err)
	}
	// Get the user's feedback
	items, err := db.GetUserFeedback(userId)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

// getPopular gets popular items from database.
func getPopular(request *restful.Request, response *restful.Response) {
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		if len(paramNumber) == 0 {
			number = 10
		} else {
			badRequest(response, err)
			return
		}
	}
	// Get the popular list
	items, err := db.GetPopular(number)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

// getRandom gets random items from database.
func getRandom(request *restful.Request, response *restful.Response) {
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		if len(paramNumber) == 0 {
			number = 10
		} else {
			badRequest(response, err)
			return
		}
	}
	// Get random items
	items, err := db.GetRandom(number)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

// getNeighbors gets neighbors of a item from database.
func getNeighbors(request *restful.Request, response *restful.Response) {
	// Get item id
	paramUserId := request.PathParameter("item-id")
	itemId, err := strconv.Atoi(paramUserId)
	if err != nil {
		badRequest(response, err)
	}
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		if len(paramNumber) == 0 {
			number = 10
		} else {
			badRequest(response, err)
			return
		}
	}
	// Get recommended items
	items, err := db.GetNeighbors(itemId, number)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

// getRecommends gets cached recommended items from database.
func getRecommends(request *restful.Request, response *restful.Response) {
	// Get user id
	paramUserId := request.PathParameter("user-id")
	userId, err := strconv.Atoi(paramUserId)
	if err != nil {
		badRequest(response, err)
	}
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		if len(paramNumber) == 0 {
			number = 10
		} else {
			badRequest(response, err)
			return
		}
	}
	// Get recommended items
	items, err := db.GetRecommends(userId, number)
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
	FeedbackBefore int // number of feedback before change
	FeedbackAfter  int // number of feedback after change
}

// putItems puts items into the database.
func putItems(request *restful.Request, response *restful.Response) {
	// Add ratings
	items := new([]int)
	if err := request.ReadEntity(items); err != nil {
		badRequest(response, err)
		return
	}
	var err error
	change := Change{}
	change.FeedbackBefore, err = db.CountFeedback()
	if err != nil {
		internalServerError(response, err)
		return
	}
	change.FeedbackAfter = change.FeedbackBefore
	change.ItemsBefore, err = db.CountItems()
	if err != nil {
		internalServerError(response, err)
		return
	}
	for _, itemId := range *items {
		err = db.InsertItem(itemId)
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	change.ItemsAfter, err = db.CountItems()
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, change)
}

// Feedback is the feedback from a user to an item.
type Feedback struct {
	UserId   int     // identifier of the user
	ItemId   int     // identifier of the item
	Feedback float64 // rating, confidence or indicator
}

// putFeedback puts new ratings into the database.
func putFeedback(request *restful.Request, response *restful.Response) {
	// Add ratings
	ratings := new([]Feedback)
	if err := request.ReadEntity(ratings); err != nil {
		badRequest(response, err)
		return
	}
	var err error
	status := Change{}
	status.FeedbackBefore, err = db.CountFeedback()
	if err != nil {
		internalServerError(response, err)
		return
	}
	status.FeedbackAfter = status.FeedbackBefore
	status.ItemsBefore, err = db.CountItems()
	if err != nil {
		internalServerError(response, err)
		return
	}
	for _, feedback := range *ratings {
		err = db.InsertFeedback(feedback.UserId, feedback.ItemId, feedback.Feedback)
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	status.FeedbackAfter, err = db.CountFeedback()
	if err != nil {
		internalServerError(response, err)
		return
	}
	status.ItemsAfter, err = db.CountItems()
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, status)
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
