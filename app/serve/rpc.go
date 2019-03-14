package serve

import (
	"fmt"
	"github.com/emicklei/go-restful"
	"log"
	"net/http"
	"strconv"
)

func serveRPC() {
	// Create a web service
	ws := new(restful.WebService)
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	// Get the recommendation list
	ws.Route(ws.GET("/top/{user-id}").
		To(getTop).
		Doc("get the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")))
	// Add ratings
	ws.Route(ws.PUT("/ratings/").
		To(putRatings).
		Doc("put ratings"))
	// Add ratings for a user
	ws.Route(ws.PUT("/ratings/user/{user-id}").
		To(putRatings).
		Doc("put ratings"))
	// Start web service
	restful.DefaultContainer.Add(ws)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getTop(request *restful.Request, response *restful.Response) {
	// Get user ID
	userId, err := strconv.Atoi(request.PathParameter("user-id"))
	if err != nil {
		logError(response.WriteError(400, err))
		return
	}
	// Get the top list
	topList, err := GetTop(db, userId)
	if err != nil {
		logError(response.WriteError(500, err))
		return
	}
	// Send result
	logError(response.WriteAsJson(topList))
}

func putRatings(request *restful.Request, response *restful.Response) {
	fmt.Println(request)
}

func logError(err error) {
	if err != nil {
		log.Println(err)
	}
}
