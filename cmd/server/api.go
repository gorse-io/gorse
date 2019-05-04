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
	// Start web service
	restful.DefaultContainer.Add(ws)
	log.Printf("start a server at %v\n", fmt.Sprintf("%s:%d", config.Host, config.Port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", config.Host, config.Port), nil))
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
			Failed(response, err)
			return
		}
	}
	// Get the popular list
	items, err := db.GetPopular(number)
	if err != nil {
		Failed(response, err)
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
			Failed(response, err)
			return
		}
	}
	// Get random items
	items, err := db.GetRandom(number)
	if err != nil {
		Failed(response, err)
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
		Failed(response, err)
	}
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		if len(paramNumber) == 0 {
			number = 10
		} else {
			Failed(response, err)
			return
		}
	}
	// Get recommended items
	items, err := db.GetNeighbors(itemId, number)
	if err != nil {
		Failed(response, err)
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
		Failed(response, err)
	}
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		if len(paramNumber) == 0 {
			number = 10
		} else {
			Failed(response, err)
			return
		}
	}
	// Get recommended items
	items, err := db.GetRecommends(userId, number)
	if err != nil {
		Failed(response, err)
		return
	}
	// Send result
	Json(response, items)
}

type Status struct {
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
		Failed(response, err)
		return
	}
	var err error
	status := Status{}
	status.FeedbackBefore, err = db.CountFeedback()
	if err != nil {
		Failed(response, err)
		return
	}
	status.FeedbackAfter = status.FeedbackBefore
	status.ItemsBefore, err = db.CountItems()
	if err != nil {
		Failed(response, err)
		return
	}
	for _, itemId := range *items {
		err = db.InsertItem(itemId)
		if err != nil {
			Failed(response, err)
			return
		}
	}
	status.ItemsAfter, err = db.CountItems()
	if err != nil {
		Failed(response, err)
		return
	}
	Json(response, status)
}

// PutFeedback puts new ratings into database.
func PutFeedback(request *restful.Request, response *restful.Response) {
	// Add ratings
	ratings := new([]engine.Feedback)
	if err := request.ReadEntity(ratings); err != nil {
		Failed(response, err)
		return
	}
	var err error
	status := Status{}
	status.FeedbackBefore, err = db.CountFeedback()
	if err != nil {
		Failed(response, err)
		return
	}
	status.FeedbackAfter = status.FeedbackBefore
	status.ItemsBefore, err = db.CountItems()
	if err != nil {
		Failed(response, err)
		return
	}
	for _, feedback := range *ratings {
		err = db.InsertFeedback(feedback.UserId, feedback.ItemId, feedback.Feedback)
		if err != nil {
			Failed(response, err)
			return
		}
	}
	status.FeedbackAfter, err = db.CountFeedback()
	if err != nil {
		Failed(response, err)
		return
	}
	status.ItemsAfter, err = db.CountItems()
	if err != nil {
		Failed(response, err)
		return
	}
	Json(response, status)
}

// ErrorResponse capsules the error message.
type ErrorResponse struct {
	Failed bool   // `true` for error message
	Error  string // error message
}

// Failed sends the error message to the client.
func Failed(response *restful.Response, err error) {
	log.Println(err)
	if err = response.WriteAsJson(ErrorResponse{true, err.Error()}); err != nil {
		log.Println(err)
	}
}

// Json sends the content as JSON to the client.
func Json(response *restful.Response, content interface{}) {
	if err := response.WriteAsJson(content); err != nil {
		log.Println(err)
	}
}
