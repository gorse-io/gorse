package cmd_serve

import (
	"fmt"
	"github.com/emicklei/go-restful"
	"log"
	"net/http"
	"strconv"
)

const (
	BadRequest          = 400
	InternalServerError = 500
)

func Server(config ServeConfig) {
	// Create a web service
	ws := new(restful.WebService)
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	// Get the recommendation list
	ws.Route(ws.GET("/top/{user-id}").
		To(GetRecommendations).
		Doc("get the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")))
	// Popular items
	ws.Route(ws.GET("/popular").To(GetPopular).Doc("get popular items"))
	// Random items
	ws.Route(ws.GET("/random").To(GetRandom).Doc("get random items"))
	// Add ratings
	ws.Route(ws.PUT("/ratings/").
		To(PutRatings).
		Doc("put ratings"))
	// Add ratings for a user
	ws.Route(ws.PUT("/ratings/user/{user-id}").
		To(PutRatings).
		Doc("put ratings"))
	// Start web service
	restful.DefaultContainer.Add(ws)
	log.Printf("start a server at %v\n", fmt.Sprintf("%s:%d", config.Host, config.Port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", config.Host, config.Port), nil))
}

func GetPopular(request *restful.Request, response *restful.Response) {
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		HttpError(response, BadRequest, err)
	}
	// Get the popular list
	items, _, err := db.GetPopular(number)
	if err != nil {
		HttpError(response, InternalServerError, err)
		return
	}
	// Send result
	Json(response, items)
}

func GetRandom(request *restful.Request, response *restful.Response) {
	// Get the number
	paramNumber := request.QueryParameter("number")
	number, err := strconv.Atoi(paramNumber)
	if err != nil {
		HttpError(response, BadRequest, err)
	}
	// Get random items
	items, err := db.GetRandom(number)
	if err != nil {
		HttpError(response, InternalServerError, err)
		return
	}
	// Send result
	Json(response, items)
}

func GetRecommendations(request *restful.Request, response *restful.Response) {

}

func PutRatings(request *restful.Request, response *restful.Response) {
	fmt.Println(request)
}

func HttpError(response *restful.Response, code int, err error) {
	log.Println(err)
	if err = response.WriteError(code, err); err != nil {
		log.Println(err)
	}
}

func Json(response *restful.Response, content interface{}) {
	if err := response.WriteAsJson(content); err != nil {
		log.Println(err)
	}
}
