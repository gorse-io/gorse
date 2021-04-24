package master

import (
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/scylladb/go-set"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"time"
)

func (m *Master) StartHttpServer() {

	ws := m.WebService

	ws.Route(ws.GET("/dashboard/cluster").To(m.getCluster).
		Doc("Get nodes in the cluster.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Writes([]Node{}))
	ws.Route(ws.GET("/dashboard/stats").To(m.getStats).
		Doc("Get global statistics.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.HeaderParameter("X-API-Key", "secret key for RESTful API")).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes(Status{}))
	// Get a user
	ws.Route(ws.GET("/dashboard/user/{user-id}").To(m.getUser).
		Doc("Get a user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Writes(User{}))
	// Get users
	ws.Route(ws.GET("/dashboard/users").To(m.getUsers).
		Doc("Get users.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned users").DataType("int")).
		Param(ws.QueryParameter("cursor", "cursor for next page").DataType("string")).
		Writes(UserIterator{}))
	// Get popular items
	ws.Route(ws.GET("/dashboard/popular").To(m.getPopular).
		Doc("get popular items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	// Get latest items
	ws.Route(ws.GET("/dashboard/latest").To(m.getLatest).
		Doc("get latest items").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Param(ws.QueryParameter("offset", "offset of the list").DataType("int")).
		Writes([]data.Item{}))
	ws.Route(ws.GET("/dashboard/recommend/{user-id}").To(m.getRecommend).
		Doc("Get recommendation for user.").
		Metadata(restfulspec.KeyOpenAPITags, []string{"dashboard"}).
		Param(ws.PathParameter("user-id", "identifier of the user").DataType("string")).
		Param(ws.QueryParameter("n", "number of returned items").DataType("int")).
		Writes([]data.Item{}))

	m.RestServer.StartHttpServer()
}

func (m *Master) getCluster(request *restful.Request, response *restful.Response) {
	// collect nodes
	workers := make([]*Node, 0)
	servers := make([]*Node, 0)
	m.nodesInfoMutex.Lock()
	for _, info := range m.nodesInfo {
		switch info.Type {
		case WorkerNode:
			workers = append(workers, info)
		case ServerNode:
			servers = append(servers, info)
		}
	}
	m.nodesInfoMutex.Unlock()
	// return nodes
	nodes := make([]*Node, 0)
	nodes = append(nodes, workers...)
	nodes = append(nodes, servers...)
	server.Ok(response, nodes)
}

type Status struct {
	NumUsers       string
	NumItems       string
	NumPosFeedback string
	PRModel        string
	CTRModel       string
}

func (m *Master) getStats(request *restful.Request, response *restful.Response) {
	status := Status{}
	var err error
	// read number of users
	status.NumUsers, err = m.CacheStore.GetString(cache.GlobalMeta, cache.NumUsers)
	if err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	// read number of items
	status.NumItems, err = m.CacheStore.GetString(cache.GlobalMeta, cache.NumItems)
	if err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	// read number of positive feedback
	status.NumPosFeedback, err = m.CacheStore.GetString(cache.GlobalMeta, cache.NumPositiveFeedback)
	if err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	status.PRModel = m.prModelName
	server.Ok(response, status)
}

type UserIterator struct {
	Cursor string
	Users  []User
}

type User struct {
	data.User
	LastActiveTime string
	LastUpdateTime string
}

func (m *Master) getUsers(request *restful.Request, response *restful.Response) {
	// Authorize
	cursor := request.QueryParameter("cursor")
	n, err := server.ParseInt(request, "n", m.GorseConfig.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	// get all users
	cursor, users, err := m.DataStore.GetUsers(cursor, n)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	details := make([]User, len(users))
	for i, user := range users {
		details[i].User = user
		if details[i].LastActiveTime, err = m.CacheStore.GetString(cache.LastActiveTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
			server.InternalServerError(response, err)
			return
		}
		if details[i].LastUpdateTime, err = m.CacheStore.GetString(cache.LastUpdateRecommendTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, UserIterator{Cursor: cursor, Users: details})
}

func (m *Master) getUser(request *restful.Request, response *restful.Response) {
	// get user id
	userId := request.PathParameter("user-id")
	// get user
	user, err := m.DataStore.GetUser(userId)
	if err != nil {
		if err.Error() == data.ErrUserNotExist {
			server.PageNotFound(response, err)
		} else {
			server.InternalServerError(response, err)
		}
		return
	}
	detail := User{User: user}
	if detail.LastActiveTime, err = m.CacheStore.GetString(cache.LastActiveTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	if detail.LastUpdateTime, err = m.CacheStore.GetString(cache.LastUpdateRecommendTime, user.UserId); err != nil && err != cache.ErrObjectNotExist {
		server.InternalServerError(response, err)
		return
	}
	server.Ok(response, detail)
}

func (m *Master) getRecommend(request *restful.Request, response *restful.Response) {
	// parse arguments
	userId := request.PathParameter("user-id")
	n, err := server.ParseInt(request, "n", m.GorseConfig.Server.DefaultN)
	if err != nil {
		server.BadRequest(response, err)
		return
	}
	// load offline recommendation
	start := time.Now()
	itemsChan := make(chan []string, 1)
	errChan := make(chan error, 1)
	go func() {
		var collaborativeFilteringItems []string
		collaborativeFilteringItems, err = m.CacheStore.GetList(cache.CollaborativeItems, userId, 0, m.GorseConfig.Database.CacheSize)
		if err != nil {
			itemsChan <- nil
			errChan <- err
		} else {
			itemsChan <- collaborativeFilteringItems
			errChan <- nil
			if len(collaborativeFilteringItems) == 0 {
				base.Logger().Warn("empty collaborative filtering", zap.String("user_id", userId))
			}
		}
	}()
	// load historical feedback
	userFeedback, err := m.DataStore.GetUserFeedback(userId, nil)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	excludeSet := set.NewStringSet()
	for _, feedback := range userFeedback {
		excludeSet.Add(feedback.ItemId)
	}
	// remove historical items
	items := <-itemsChan
	err = <-errChan
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	results := make([]string, 0, len(items))
	for _, itemId := range items {
		if !excludeSet.Has(itemId) {
			results = append(results, itemId)
		}
	}
	if len(results) > n {
		results = results[:n]
	}
	spent := time.Since(start)
	base.Logger().Info("complete recommendation",
		zap.Duration("total_time", spent))
	// Send result
	details := make([]data.Item, len(results))
	for i := range results {
		details[i], err = m.DataStore.GetItem(results[i])
		if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, details)
}

type Feedback struct {
	FeedbackType string
	UserId       string
	Item         data.Item
	Timestamp    time.Time
	Comment      string
}

// get feedback by user-id
func (m *Master) getFeedbackByUser(request *restful.Request, response *restful.Response) {
	userId := request.PathParameter("user-id")
	feedback, err := m.DataStore.GetUserFeedback(userId, nil)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	details := make([]Feedback, len(feedback))
	for i := range feedback {
		details[i].FeedbackType = feedback[i].FeedbackType
		details[i].UserId = feedback[i].UserId
		details[i].Timestamp = feedback[i].Timestamp
		details[i].Comment = feedback[i].Comment
		details[i].Item, err = m.DataStore.GetItem(feedback[i].ItemId)
		if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, details)
}

func (m *Master) getList(prefix string, name string, request *restful.Request, response *restful.Response) {
	var begin, end int
	var err error
	if begin, err = server.ParseInt(request, "begin", 0); err != nil {
		server.BadRequest(response, err)
		return
	}
	if end, err = server.ParseInt(request, "end", m.GorseConfig.Server.DefaultN-1); err != nil {
		server.BadRequest(response, err)
		return
	}
	// Get the popular list
	items, err := m.CacheStore.GetList(prefix, name, begin, end)
	if err != nil {
		server.InternalServerError(response, err)
		return
	}
	// Send result
	itemDetails := make([]data.Item, len(items))
	for i := range items {
		itemDetails[i], err = m.DataStore.GetItem(items[i])
		if err != nil {
			server.InternalServerError(response, err)
			return
		}
	}
	server.Ok(response, itemDetails)
}

// getPopular gets popular items from database.
func (m *Master) getPopular(request *restful.Request, response *restful.Response) {
	m.getList(cache.PopularItems, "", request, response)
}

func (m *Master) getLatest(request *restful.Request, response *restful.Response) {
	m.getList(cache.LatestItems, "", request, response)
}
