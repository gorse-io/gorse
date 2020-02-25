package cmd

import (
	"fmt"
	"github.com/araddon/dateparse"
	"github.com/emicklei/go-restful"
	"github.com/zhenghaoz/gorse/engine"
	"log"
	"net/http"
	"strconv"
)
var engineConfig engine.TomlConfig
func serve(config engine.TomlConfig) {
	engineConfig = config
	// Create a web service
	ws := new(restful.WebService)
	ws.Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)

	// Get the recommendation list
	ws.Route(ws.GET("/recommends/{user-id}").
		To(getRecommends).
		Doc("get the top list for a user").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")).
		Param(ws.FormParameter("number", "the number of recommendations").DataType("int")).
		Param(ws.FormParameter("p", "weight of popularity").DataType("float")).
		Param(ws.FormParameter("t", "weight of time").DataType("float")).
		Param(ws.PathParameter("c", "weight of collaborative filtering").DataType("float")))
	// Get popular items
	ws.Route(ws.GET("/popular").To(getPopular).
		Doc("get popular items").
		Param(ws.FormParameter("number", "the number of popular items").DataType("int")))
	// Get latest items
	ws.Route(ws.GET("/latest").To(getLatest).
		Doc("get latest items").
		Param(ws.FormParameter("number", "the number of latest items").DataType("int")))
	// Get random items
	ws.Route(ws.GET("/random").To(getRandom).
		Doc("get random items").
		Param(ws.FormParameter("number", "the number of random items").DataType("int")))
	// Get neighbors
	ws.Route(ws.GET("/neighbors/{item-id}").To(getNeighbors).
		Doc("get neighbors of a item").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")).
		Param(ws.FormParameter("number", "the number of neighbors").DataType("int")))

	// Put items
	ws.Route(ws.PUT("/items").To(putItems)).
		Doc("put items")
	// Get items
	ws.Route(ws.GET("/items").To(getItems)).
		Doc("get items")
	// Get item
	ws.Route(ws.GET("/item/{item-id}").To(getItem).
		Doc("get a item").
		Param(ws.PathParameter("item-id", "identifier of the item").DataType("int")))

	// Put feedback
	ws.Route(ws.PUT("/feedback").To(putFeedback).
		Doc("put feedback"))
	// Get users
	ws.Route(ws.GET("/users").
		To(getUsers).
		Doc("get the list of users"))
	// Get user feedback
	ws.Route(ws.GET("/user/{user-id}/feedback").To(getUserFeedback).
		Doc("get a user's feedback").
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("int")))

	ws.Route(ws.GET("/status").To(getStatus))
	// Start web service
	restful.DefaultContainer.Add(ws)
	log.Printf("start a server at %v\n", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port), nil))
}

// Status contains information about engine.
type Status struct {
	FeedbackCount int    // number of feedback
	ItemCount     int    // number of items
	UserCount     int    // number of users
	CommitCount   int    // number of committed feedback
	CommitTime    string // time for commit
}

func status() (Status, error) {
	status := Status{}
	var err error
	// Get feedback count
	if status.FeedbackCount, err = db.CountFeedback(); err != nil {
		return status, err
	}
	// Get item count
	if status.ItemCount, err = db.CountItems(); err != nil {
		return status, err
	}
	// Get user count
	if status.UserCount, err = db.CountUsers(); err != nil {
		return status, err
	}
	// Get commit count
	var commit string
	if commit, err = db.GetMeta("commit"); err != nil {
		return status, err
	}
	if status.CommitCount, err = strconv.Atoi(commit); len(commit) > 0 && err != nil {
		return status, err
	}
	// Get commit time
	if status.CommitTime, err = db.GetMeta("commit_time"); err != nil {
		return status, err
	}
	return status, nil
}

func getStatus(request *restful.Request, response *restful.Response) {
	status, err := status()
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, status)
}

func getUsers(request *restful.Request, response *restful.Response) {
	users, err := db.GetUsers()
	if err != nil {
		internalServerError(response, err)
		return
	}
	json(response, users)
}

func getUserFeedback(request *restful.Request, response *restful.Response) {
	// Get user id
	userId := request.PathParameter("user-id")
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
	items, err := db.GetList(engine.ListPop, number)
	if err != nil {
		internalServerError(response, err)
		return
	}
	// Send result
	json(response, items)
}

func getLatest(request *restful.Request, response *restful.Response) {
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
	items, err := db.GetList(engine.ListLatest, number)
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
	itemId := request.PathParameter("item-id")
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
	items, err := db.GetIdentList(engine.BucketNeighbors, itemId, number)
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
	userId := request.PathParameter("user-id")
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
	// Get weights
	weights := []float64{0.0, 0.0, 1.0}
	params := []string{
		request.QueryParameter("p"),
		request.QueryParameter("t"),
		request.QueryParameter("c"),
	}
	for i := range params {
		if len(params[i]) > 0 {
			weights[i], err = strconv.ParseFloat(params[i], 64)
			if err != nil {
				badRequest(response, err)
			}
		}
	}
	p, t, c := weights[0], weights[1], weights[2]
	// Get recommended items
	items, err := db.GetIdentList(engine.BucketRecommends, userId, 0)
	if err != nil {
		internalServerError(response, err)
		return
	}
	if engineConfig.Recommend.Once {
		// Get read recommended items
		reads,err := db.GetIdentMap(engine.BucketReads, userId)
		if reads == nil {
			reads = make(map[string]bool)
		}
		var subItems []engine.RecommendedItem
		change := false
		notRecommended := false
		if err != nil {
			change = false
		} else {
			for i := range items {
				exist := reads[items[i].ItemId]
				if !exist {
					subItems = append(subItems,items[i])
					change = true
				}
			}
			if !change{
				notRecommended = true
			}
		}
		if notRecommended {
			var empty []engine.RecommendedItem
			json(response, empty)
		}else if change {
			subItems = engine.Ranking(subItems, number, p, t, c)
			for i := range subItems {
				reads[subItems[i].ItemId] = true
			}
			if err := db.PutIdentMap(engine.BucketReads, userId, reads); err != nil {
				badRequest(response, err)
			}
			// Send result
			json(response, subItems)
		} else {
			// Send result
			items = engine.Ranking(items, number, p, t, c)
			for i := range items {
				reads[items[i].ItemId] = true
			}
			if err := db.PutIdentMap(engine.BucketReads, userId, reads); err != nil {
				badRequest(response, err)
			}
			json(response, items)
		}
	}else{
		items = engine.Ranking(items, number, p, t, c)
		json(response, items)
	}

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

type ItemStringTime struct {
	ItemId    string
	Timestamp string
}

// putItems puts items into the database.
func putItems(request *restful.Request, response *restful.Response) {
	// Add ratings
	temp := new([]ItemStringTime)
	if err := request.ReadEntity(temp); err != nil {
		badRequest(response, err)
		return
	}
	// Parse timestamp
	var err error
	items := make([]engine.Item, len(*temp))
	for i, v := range *temp {
		items[i].ItemId = v.ItemId
		items[i].Timestamp, err = dateparse.ParseAny(v.Timestamp)
		if err != nil {
			badRequest(response, err)
		}
	}
	change := Change{}
	// Get status before change
	stat, err := status()
	if err != nil {
		internalServerError(response, err)
		return
	}
	change.FeedbackBefore = stat.FeedbackCount
	change.ItemsBefore = stat.ItemCount
	change.UsersBefore = stat.UserCount
	// Insert items
	for _, item := range items {
		err = db.InsertItem(item.ItemId, &item.Timestamp)
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	// Get status after change
	stat, err = status()
	if err != nil {
		internalServerError(response, err)
		return
	}
	change.FeedbackAfter = stat.FeedbackCount
	change.ItemsAfter = stat.ItemCount
	change.UsersAfter = stat.UserCount
	json(response, change)
}

func getItems(request *restful.Request, response *restful.Response) {
	items, err := db.GetItems()
	if err != nil {
		internalServerError(response, err)
	}
	json(response, items)
}

func getItem(request *restful.Request, response *restful.Response) {
	// Get item id
	itemId := request.PathParameter("item-id")
	// Get item
	item, err := db.GetItem(itemId)
	if err != nil {
		internalServerError(response, err)
	}
	json(response, item)
}

// Feedback is the feedback from a user to an item.
type Feedback struct {
	UserId   string     // identifier of the user
	ItemId   string     // identifier of the item
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
	change := Change{}
	// Get status before change
	stat, err := status()
	if err != nil {
		internalServerError(response, err)
		return
	}
	change.FeedbackBefore = stat.FeedbackCount
	change.ItemsBefore = stat.ItemCount
	change.UsersBefore = stat.UserCount
	// Insert feedback
	for _, feedback := range *ratings {
		err = db.InsertFeedback(feedback.UserId, feedback.ItemId, feedback.Feedback)
		if err != nil {
			internalServerError(response, err)
			return
		}
	}
	// Get status after change
	stat, err = status()
	if err != nil {
		internalServerError(response, err)
		return
	}
	change.FeedbackAfter = stat.FeedbackCount
	change.ItemsAfter = stat.ItemCount
	change.UsersAfter = stat.UserCount
	json(response, change)
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
