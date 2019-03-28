package cmd_serve

import (
	"fmt"
	"github.com/emicklei/go-restful"
	"log"
	"net/http"
	"strconv"
)

// Server receives requests from clients and sent responses back.
func Server(config ServeConfig) {
	// Create a web service
	ws := new(restful.WebService)
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	// Get the recommendation list
	ws.Route(ws.GET("/recommends/{user-id}").
		To(GetRecommendations).
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
	// Add items
	ws.Route(ws.PUT("/items").To(PutItems)).
		Doc("put items")
	// Add ratings
	ws.Route(ws.PUT("/ratings").To(PutRatings).
		Doc("put ratings"))
	// Start web service
	restful.DefaultContainer.Add(ws)
	log.Printf("start a server at %v\n", fmt.Sprintf("%s:%d", config.Host, config.Port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", config.Host, config.Port), nil))
}

// QueryResponse capsules results for queries.
type QueryResponse struct {
	Failed bool  // `false` for success
	Items  []int // items in the response
}

// GetPopular gets popular items from database.
func GetPopular(request *restful.Request, response *restful.Response) {
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		Failed(response, err)
	}
	// Get the popular list
	items, _, err := db.GetPopular(number)
	if err != nil {
		Failed(response, err)
		return
	}
	// Send result
	Json(response, QueryResponse{Items: items})
}

// GetRandom gets random items from database.
func GetRandom(request *restful.Request, response *restful.Response) {
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		Failed(response, err)
	}
	// Get random items
	items, err := db.GetRandom(number)
	if err != nil {
		Failed(response, err)
		return
	}
	// Send result
	Json(response, QueryResponse{Items: items})
}

// GetRecommendations gets cached recommended items from database.
func GetRecommendations(request *restful.Request, response *restful.Response) {
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
		Failed(response, err)
	}
	// Get recommended items
	items, err := db.GetRecommends(userId, number)
	if err != nil {
		Failed(response, err)
		return
	}
	// Send result
	Json(response, QueryResponse{Items: items})
}

// ExecResponse capsules result of execution.
type ExecResponse struct {
	Failed bool // `false` for success
}

// PutItems puts items
func PutItems(request *restful.Request, response *restful.Response) {
	// Add ratings
	items := new([]int)
	if err := request.ReadEntity(items); err != nil {
		Failed(response, err)
	}
	err := db.PutItems(*items)
	if err != nil {
		Failed(response, err)
		return
	}
	Json(response, ExecResponse{})
}

// RatingTuple is the tuple of a rating record.
type RatingTuple struct {
	UserId int     // the identifier of the user
	ItemId int     // the identifier of the item
	Rating float64 // the rating
}

// PutRatings puts new ratings into database.
func PutRatings(request *restful.Request, response *restful.Response) {
	// Add ratings
	ratings := new([]RatingTuple)
	if err := request.ReadEntity(ratings); err != nil {
		Failed(response, err)
	}
	for _, v := range *ratings {
		err := db.PutRating(v.UserId, v.ItemId, v.Rating)
		if err != nil {
			Failed(response, err)
			return
		}
	}
	Json(response, ExecResponse{})
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
