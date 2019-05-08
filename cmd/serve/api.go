package serve

import (
	"fmt"
	"github.com/emicklei/go-restful"
	"github.com/zhenghaoz/gorse/engine"
	"log"
	"net/http"
	"strconv"
)

// Server receives requests from clients and sent responses back.
func Server(config engine.ServerConfig) {
	// Create a web service
	ws := new(restful.WebService)
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	// Get the recommendation list
	ws.Route(ws.GET("/recommends/{user-id}").
		To(GetRecommends).
		Doc("get the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")).
		Param(ws.FormParameter("number", "the number of recommendations").DataType("int")))
	// Popular items
	ws.Route(ws.GET("/popular").To(GetPopular).
		Doc("get popular items").
		Param(ws.FormParameter("number", "the number of popular items").DataType("int")))
	// Random items
	ws.Route(ws.GET("/random").To(GetRandom).
		Doc("get random items").
		Param(ws.FormParameter("number", "the number of random items").DataType("int")))
	// Neighbors
	ws.Route(ws.GET("/neighbors/{item-id}").To(GetNeighbors).
		Doc("get neighbors of a item").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")).
		Param(ws.FormParameter("number", "the number of neighbors").DataType("int")))
	// Add items
	ws.Route(ws.PUT("/items").To(PutItems)).
		Doc("put items")
	// Add ratings
	ws.Route(ws.PUT("/feedback").To(PutFeedback).
		Doc("put feedback"))
	ws.Route(ws.GET("/status").To(GetStatus))
	// Start web service
	restful.DefaultContainer.Add(ws)
	log.Printf("start a server at %v\n", fmt.Sprintf("%s:%d", config.Host, config.Port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", config.Host, config.Port), nil))
}

type Status struct {
	FeedbackCount int
	ItemCount     int
	CommitCount   int
}

func GetStatus(request *restful.Request, response *restful.Response) {
	status := Status{}
	var err error
	// Get feedback count
	if status.FeedbackCount, err = db.CountFeedback(); err != nil {
		InternalServerError(response, err)
	}
	// Get item count
	if status.ItemCount, err = db.CountItems(); err != nil {
		InternalServerError(response, err)
	}
	// Get commit count
	var commit string
	if commit, err = db.GetMeta("commit"); err != nil {
		InternalServerError(response, err)
	}
	if status.CommitCount, err = strconv.Atoi(commit); len(commit) > 0 && err != nil {
		InternalServerError(response, err)
	}
	Json(response, status)
}

// GetPopular gets popular items from database.
func GetPopular(request *restful.Request, response *restful.Response) {
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		if len(paramNumber) == 0 {
			number = 10
		} else {
			BadRequest(response, err)
			return
		}
	}
	// Get the popular list
	items, err := db.GetPopular(number)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	// Send result
	Json(response, items)
}

// GetRandom gets random items from database.
func GetRandom(request *restful.Request, response *restful.Response) {
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		if len(paramNumber) == 0 {
			number = 10
		} else {
			BadRequest(response, err)
			return
		}
	}
	// Get random items
	items, err := db.GetRandom(number)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	// Send result
	Json(response, items)
}

// GetNeighbors gets neighbors of a item from database.
func GetNeighbors(request *restful.Request, response *restful.Response) {
	// Get item id
	paramUserId := request.PathParameter("item-id")
	itemId, err := strconv.Atoi(paramUserId)
	if err != nil {
		BadRequest(response, err)
	}
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		if len(paramNumber) == 0 {
			number = 10
		} else {
			BadRequest(response, err)
			return
		}
	}
	// Get recommended items
	items, err := db.GetNeighbors(itemId, number)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	// Send result
	Json(response, items)
}

// GetRecommends gets cached recommended items from database.
func GetRecommends(request *restful.Request, response *restful.Response) {
	// Get user id
	paramUserId := request.PathParameter("user-id")
	userId, err := strconv.Atoi(paramUserId)
	if err != nil {
		BadRequest(response, err)
	}
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		if len(paramNumber) == 0 {
			number = 10
		} else {
			BadRequest(response, err)
			return
		}
	}
	// Get recommended items
	items, err := db.GetRecommends(userId, number)
	if err != nil {
		InternalServerError(response, err)
		return
	}
	// Send result
	Json(response, items)
}

type Change struct {
	ItemsBefore    int
	ItemsAfter     int
	FeedbackBefore int
	FeedbackAfter  int
}

// PutItems puts items
func PutItems(request *restful.Request, response *restful.Response) {
	// Add ratings
	items := new([]int)
	if err := request.ReadEntity(items); err != nil {
		BadRequest(response, err)
		return
	}
	var err error
	change := Change{}
	change.FeedbackBefore, err = db.CountFeedback()
	if err != nil {
		InternalServerError(response, err)
		return
	}
	change.FeedbackAfter = change.FeedbackBefore
	change.ItemsBefore, err = db.CountItems()
	if err != nil {
		InternalServerError(response, err)
		return
	}
	for _, itemId := range *items {
		err = db.InsertItem(itemId)
		if err != nil {
			InternalServerError(response, err)
			return
		}
	}
	change.ItemsAfter, err = db.CountItems()
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Json(response, change)
}

// PutFeedback puts new ratings into database.
func PutFeedback(request *restful.Request, response *restful.Response) {
	// Add ratings
	ratings := new([]engine.Feedback)
	if err := request.ReadEntity(ratings); err != nil {
		BadRequest(response, err)
		return
	}
	var err error
	status := Change{}
	status.FeedbackBefore, err = db.CountFeedback()
	if err != nil {
		InternalServerError(response, err)
		return
	}
	status.FeedbackAfter = status.FeedbackBefore
	status.ItemsBefore, err = db.CountItems()
	if err != nil {
		InternalServerError(response, err)
		return
	}
	for _, feedback := range *ratings {
		err = db.InsertFeedback(feedback.UserId, feedback.ItemId, feedback.Feedback)
		if err != nil {
			InternalServerError(response, err)
			return
		}
	}
	status.FeedbackAfter, err = db.CountFeedback()
	if err != nil {
		InternalServerError(response, err)
		return
	}
	status.ItemsAfter, err = db.CountItems()
	if err != nil {
		InternalServerError(response, err)
		return
	}
	Json(response, status)
}

func BadRequest(response *restful.Response, err error) {
	log.Println(err)
	if err = response.WriteError(400, err); err != nil {
		log.Println(err)
	}
}

func InternalServerError(response *restful.Response, err error) {
	log.Println(err)
	if err = response.WriteError(500, err); err != nil {
		log.Println(err)
	}
}

// Json sends the content as JSON to the client.
func Json(response *restful.Response, content interface{}) {
	if err := response.WriteAsJson(content); err != nil {
		log.Println(err)
	}
}
