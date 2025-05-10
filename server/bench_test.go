// Copyright 2022 gorse Project Authors
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
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/go-resty/resty/v2"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/proto"
	"math/rand"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	numInitUsers = 10000
	numInitItems = 10000
)

var (
	benchDataStore  string
	benchCacheStore string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	benchDataStore = env("BENCH_DATA_STORE", "clickhouse://127.0.0.1:8123/")
	benchCacheStore = env("BENCH_CACHE_STORE", "redis://127.0.0.1:6379/")
}

type benchServer struct {
	listener net.Listener
	address  string
	RestServer
}

func newBenchServer(b *testing.B) *benchServer {
	ctx := context.Background()
	// retrieve benchmark name
	var benchName string
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		splits := strings.Split(details.Name(), ".")
		benchName = splits[len(splits)-1]
	} else {
		b.Fatalf("failed to retrieve benchmark name")
	}

	// configuration
	s := &benchServer{}
	s.Settings = &config.Settings{}
	s.Config = config.GetDefaultConfig()
	s.DisableLog = true
	s.WebService = new(restful.WebService)
	cacheStoreURL := s.prepareCache(b, benchCacheStore, benchName)
	dataStoreURL := s.prepareData(b, benchDataStore, benchName)
	s.CreateWebService()
	container := restful.NewContainer()
	container.Add(s.WebService)

	// open database
	var err error
	s.DataClient, err = data.Open(dataStoreURL, "")
	require.NoError(b, err)
	err = s.DataClient.Init()
	require.NoError(b, err)
	s.CacheClient, err = cache.Open(cacheStoreURL, "")
	require.NoError(b, err)
	err = s.CacheClient.Init()
	require.NoError(b, err)

	// insert users
	users := make([]data.User, 0)
	for i := 0; i < numInitUsers; i++ {
		users = append(users, data.User{
			UserId: fmt.Sprintf("init_user_%d", i),
			Labels: []string{
				fmt.Sprintf("label_%d", i),
				fmt.Sprintf("label_%d", i*2),
			},
			Comment: fmt.Sprintf("comment_%d", i),
		})
	}
	err = s.DataClient.BatchInsertUsers(ctx, users)
	require.NoError(b, err)

	// insert items
	items := make([]data.Item, 0)
	for i := 0; i < numInitItems; i++ {
		items = append(items, data.Item{
			ItemId: fmt.Sprintf("init_item_%d", i),
			Labels: []string{
				fmt.Sprintf("label%d001", i),
				fmt.Sprintf("label%d002", i),
			},
			Comment: fmt.Sprintf("add label for user: demo%d", i),
			Categories: []string{
				fmt.Sprintf("category%d001", i),
				fmt.Sprintf("category%d001", i),
			},
			Timestamp: time.Now(),
			IsHidden:  false,
		})
	}
	err = s.DataClient.BatchInsertItems(ctx, items)
	require.NoError(b, err)

	// insert feedback
	feedbacks := make([]data.Feedback, 0)
	for userIndex := 0; userIndex < numInitUsers; userIndex++ {
		itemIndex := userIndex % numInitItems
		feedbacks = append(feedbacks, data.Feedback{
			FeedbackKey: data.FeedbackKey{
				FeedbackType: "feedback_type",
				ItemId:       fmt.Sprintf("init_item_%d", userIndex),
				UserId:       fmt.Sprintf("init_user_%d", itemIndex),
			},
			Timestamp: time.Now(),
		})
	}
	err = s.DataClient.BatchInsertFeedback(ctx, feedbacks, true, true, true)
	require.NoError(b, err)

	// start http server
	s.listener, err = net.Listen("tcp", ":0")
	require.NoError(b, err)
	s.address = fmt.Sprintf("http://127.0.0.1:%d", s.listener.Addr().(*net.TCPAddr).Port)
	go func() {
		err = http.Serve(s.listener, container)
		require.NoError(b, err)
	}()
	return s
}

func (s *benchServer) prepareData(b *testing.B, url, benchName string) string {
	dbName := "gorse_data_" + benchName
	if strings.HasPrefix(url, "mysql://") {
		db, err := sql.Open("mysql", url[len("mysql://"):])
		require.NoError(b, err)
		_, err = db.Exec("DROP DATABASE IF EXISTS " + dbName)
		require.NoError(b, err)
		_, err = db.Exec("CREATE DATABASE " + dbName)
		require.NoError(b, err)
		err = db.Close()
		require.NoError(b, err)
		return url + dbName + "?timeout=30s&parseTime=true"
	} else if strings.HasPrefix(url, "postgres://") {
		db, err := sql.Open("postgres", url+"?sslmode=disable&TimeZone=UTC")
		require.NoError(b, err)
		_, err = db.Exec("DROP DATABASE IF EXISTS " + dbName)
		require.NoError(b, err)
		_, err = db.Exec("CREATE DATABASE " + dbName)
		require.NoError(b, err)
		err = db.Close()
		require.NoError(b, err)
		return url + strings.ToLower(dbName) + "?sslmode=disable&TimeZone=UTC"
	} else if strings.HasPrefix(url, "clickhouse://") {
		uri := "http://" + url[len("clickhouse://"):]
		db, err := sql.Open("clickhouse", uri)
		require.NoError(b, err)
		_, err = db.Exec("DROP DATABASE IF EXISTS " + dbName)
		require.NoError(b, err)
		_, err = db.Exec("CREATE DATABASE " + dbName)
		require.NoError(b, err)
		err = db.Close()
		require.NoError(b, err)
		return url + dbName + "?mutations_sync=2"
	} else if strings.HasPrefix(url, "mongodb://") {
		ctx := context.Background()
		cli, err := mongo.Connect(ctx, options.Client().ApplyURI(url+"?authSource=admin&connect=direct"))
		require.NoError(b, err)
		err = cli.Database(dbName).Drop(ctx)
		require.NoError(b, err)
		err = cli.Disconnect(ctx)
		require.NoError(b, err)
		return url + dbName + "?authSource=admin&connect=direct"
	} else {
		b.Fatal("unsupported data store type")
		return ""
	}
}

func (s *benchServer) prepareCache(b *testing.B, url, benchName string) string {
	dbName := "gorse_cache_" + benchName
	if strings.HasPrefix(url, "redis://") {
		opt, err := redis.ParseURL(url)
		require.NoError(b, err)
		cli := redis.NewClient(opt)
		require.NoError(b, cli.FlushDB(context.Background()).Err())
		return url
	} else if strings.HasPrefix(url, "mongodb://") {
		ctx := context.Background()
		cli, err := mongo.Connect(ctx, options.Client().ApplyURI(url+"?authSource=admin&connect=direct"))
		require.NoError(b, err)
		err = cli.Database(dbName).Drop(ctx)
		require.NoError(b, err)
		err = cli.Disconnect(ctx)
		require.NoError(b, err)
		return url + dbName + "?authSource=admin&connect=direct"
	} else if strings.HasPrefix(url, "postgres://") {
		db, err := sql.Open("postgres", url+"?sslmode=disable&TimeZone=UTC")
		require.NoError(b, err)
		_, err = db.Exec("DROP DATABASE IF EXISTS " + dbName)
		require.NoError(b, err)
		_, err = db.Exec("CREATE DATABASE " + dbName)
		require.NoError(b, err)
		err = db.Close()
		require.NoError(b, err)
		return url + strings.ToLower(dbName) + "?sslmode=disable&TimeZone=UTC"
	} else if strings.HasPrefix(url, "mysql://") {
		db, err := sql.Open("mysql", url[len("mysql://"):]+"?timeout=30s&parseTime=true")
		require.NoError(b, err)
		_, err = db.Exec("DROP DATABASE IF EXISTS " + dbName)
		require.NoError(b, err)
		_, err = db.Exec("CREATE DATABASE " + dbName)
		require.NoError(b, err)
		err = db.Close()
		require.NoError(b, err)
		return url + dbName + "?timeout=30s&parseTime=true"
	} else {
		b.Fatal("unsupported cache store type")
		return ""
	}
}

func (s *benchServer) Close(b *testing.B) {
	err := s.DataClient.Close()
	require.NoError(b, err)
	err = s.CacheClient.Close()
	require.NoError(b, err)
}

func BenchmarkInsertUser(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := client.R().
			SetBody(&data.User{
				UserId: fmt.Sprintf("user_%d", i),
				Labels: []string{
					fmt.Sprintf("label_%d", i*2+0),
					fmt.Sprintf("label_%d", i*2+1),
				},
				Comment: fmt.Sprintf("comment_%d", i),
			}).
			Post(s.address + "/api/user")
		require.NoError(b, err)
		require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
	}
	b.StopTimer()
}

func BenchmarkPatchUser(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userIndex := i % numInitUsers
		times := i / numInitUsers
		r, err := client.R().
			SetBody(&data.UserPatch{
				Labels: []string{
					fmt.Sprintf("label_%d_%d", userIndex*2+0, times),
					fmt.Sprintf("label_%d_%d", userIndex*2+1, times),
				},
				Comment: proto.String(fmt.Sprintf("comment_%d_%d", userIndex, times)),
			}).
			Patch(s.address + "/api/user/init_user_" + strconv.Itoa(userIndex))
		require.NoError(b, err)
		require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
	}
	b.StopTimer()
}

func BenchmarkGetUser(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	response := make([]*resty.Response, b.N)
	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		response[i], err = client.R().
			Get(s.address + "/api/user/init_user_" + strconv.Itoa(i%numInitUsers))
		require.NoError(b, err)
	}
	b.StopTimer()

	for i, r := range response {
		require.Equal(b, http.StatusOK, r.StatusCode())
		var ret data.User
		err := json.Unmarshal(r.Body(), &ret)
		require.NoError(b, err)
		require.Equal(b, "init_user_"+strconv.Itoa(i%numInitUsers), ret.UserId)
	}
}

func BenchmarkInsertUsers(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			users := make([][]data.User, b.N)
			for i := 0; i < b.N; i++ {
				users[i] = make([]data.User, batchSize)
				for j := range users[i] {
					userId := fmt.Sprintf("batch_%d_user_%d", i, j)
					users[i][j] = data.User{
						UserId: fmt.Sprintf("batch_%d_user_%d", i, j),
						Labels: []string{
							fmt.Sprintf("label_%s_0", userId),
							fmt.Sprintf("label_%s_1", userId),
						},
						Comment: fmt.Sprintf("comment_%s", userId),
					}
				}
			}

			client := resty.New()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r, err := client.R().
					SetBody(users[i]).
					Post(s.address + "/api/users")
				require.NoError(b, err)
				require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
			}
			b.StopTimer()
		})
	}
}

func BenchmarkGetUsers(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			response := make([]*resty.Response, b.N)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				response[i], err = client.R().
					Get(s.address + fmt.Sprintf("/api/users?n=%d", batchSize))
				require.NoError(b, err)
			}
			b.StopTimer()
			for _, r := range response {
				require.Equal(b, http.StatusOK, r.StatusCode())
				var ret UserIterator
				err := json.Unmarshal(r.Body(), &ret)
				require.NoError(b, err)
				require.Equal(b, batchSize, len(ret.Users))
			}
		})
	}
}

func BenchmarkDeleteUser(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := client.R().
			Delete(s.address + "/api/user/init_user_" + strconv.Itoa(i%numInitUsers))
		require.NoError(b, err)
		require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
	}
	b.StopTimer()
}

func BenchmarkInsertItem(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := client.R().
			SetBody(&data.Item{
				ItemId: fmt.Sprintf("item_%v", i),
				Labels: []string{
					fmt.Sprintf("label_%d", i*2+0),
					fmt.Sprintf("label_%d", i*2+1),
				},
				Categories: []string{
					fmt.Sprintf("category_%d", i*2+0),
					fmt.Sprintf("category_%d", i*2+1),
				},
				IsHidden:  i%2 == 0,
				Timestamp: time.Now(),
				Comment:   fmt.Sprintf("comment_%d", i),
			}).
			Post(s.address + "/api/item")
		require.NoError(b, err)
		require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
	}
	b.StopTimer()
}

func BenchmarkPatchItem(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		itemIndex := i % numInitItems
		times := i / numInitItems
		isHidden := false
		if i%2 == 0 {
			isHidden = true
		}
		now := time.Now()
		r, err := client.R().
			SetBody(&data.ItemPatch{
				Labels: []string{
					fmt.Sprintf("label_%d_%d", itemIndex*2+0, times),
					fmt.Sprintf("label_%d_%d", itemIndex*2+1, times),
				},
				Comment: proto.String(fmt.Sprintf("modified_comment_%d", i)),
				Categories: []string{
					fmt.Sprintf("category_%d_%d", i*2+0, times),
					fmt.Sprintf("category_%d_%d", i*2+1, times),
				},
				IsHidden:  &isHidden,
				Timestamp: &now,
			}).
			Patch(s.address + "/api/item/init_item_" + strconv.Itoa(itemIndex))
		require.NoError(b, err)
		require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
	}
	b.StopTimer()
}

func BenchmarkGetItem(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	response := make([]*resty.Response, b.N)
	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		response[i], err = client.R().
			Get(s.address + "/api/item/init_item_" + strconv.Itoa(i%numInitItems))
		require.NoError(b, err)
	}
	b.StopTimer()

	for i, r := range response {
		require.Equal(b, http.StatusOK, r.StatusCode())
		var ret data.Item
		err := json.Unmarshal(r.Body(), &ret)
		require.NoError(b, err)
		require.Equal(b, "init_item_"+strconv.Itoa(i%numInitItems), ret.ItemId)
	}
}

func BenchmarkInsertItems(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				items := make([]data.Item, 0)
				for j := 0; j < batchSize; j++ {
					itemId := fmt.Sprintf("batch_%d_item_%d", i, j)
					items = append(items, data.Item{
						ItemId: itemId,
						Labels: []string{
							fmt.Sprintf("label_%d", j*2+0),
							fmt.Sprintf("label_%d", j*2+1),
						},
						Comment: fmt.Sprintf("comment_%d", j),
					})
				}
				r, err := client.R().
					SetBody(items).
					Post(s.address + "/api/items")
				require.NoError(b, err)
				require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
			}
			b.StopTimer()
		})
	}
}

func BenchmarkGetItems(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			response := make([]*resty.Response, b.N)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				response[i], err = client.R().
					Get(s.address + fmt.Sprintf("/api/items?n=%d", batchSize))
				require.NoError(b, err)
			}
			b.StopTimer()
			for _, r := range response {
				require.Equal(b, http.StatusOK, r.StatusCode())
				var ret ItemIterator
				err := json.Unmarshal(r.Body(), &ret)
				require.NoError(b, err)
				require.Equal(b, batchSize, len(ret.Items))
			}
		})
	}
}

func BenchmarkDeleteItem(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := client.R().
			Delete(s.address + "/api/item/init_item_" + strconv.Itoa(i%numInitItems))
		require.NoError(b, err)
		require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
	}
	b.StopTimer()
}

func BenchmarkInsertCategory(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		itemIndex := i % numInitItems
		r, err := client.R().
			Put(fmt.Sprintf("%s/api/item/init_item_%d/category/category%d", s.address, itemIndex, i))
		require.NoError(b, err)
		require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
	}
	b.StopTimer()
}

func BenchmarkDeleteCategory(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		itemIndex := i % numInitItems
		r, err := client.R().
			Delete(fmt.Sprintf("%s/api/item/init_item_%d/category/category%d", s.address, itemIndex, i))
		require.NoError(b, err)
		require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
	}
	b.StopTimer()
}

func BenchmarkPutFeedback(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				feedbacks := make([]data.Feedback, 0, batchSize)
				for j := 0; j < batchSize; j++ {
					feedbacks = append(feedbacks, data.Feedback{
						FeedbackKey: data.FeedbackKey{
							FeedbackType: fmt.Sprintf("feedback_type_%d", batchSize),
							ItemId:       fmt.Sprintf("init_item_%d", rand.Intn(numInitItems)),
							UserId:       fmt.Sprintf("init_user_%d", rand.Intn(numInitUsers)),
						},
						Timestamp: time.Now(),
					})
				}
				r, err := client.R().
					SetBody(feedbacks).
					Put(s.address + "/api/feedback")
				require.NoError(b, err)
				require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
			}
			b.StopTimer()
		})
	}
}

func BenchmarkInsertFeedback(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				feedbacks := make([]data.Feedback, 0, batchSize)
				for j := 0; j < batchSize; j++ {
					feedbacks = append(feedbacks, data.Feedback{
						FeedbackKey: data.FeedbackKey{
							FeedbackType: fmt.Sprintf("feedback_type_%d", batchSize),
							ItemId:       fmt.Sprintf("init_item_%d", rand.Intn(numInitItems)),
							UserId:       fmt.Sprintf("init_user_%d", rand.Intn(numInitUsers)),
						},
						Timestamp: time.Now(),
					})
				}
				r, err := client.R().
					SetBody(feedbacks).
					Post(s.address + "/api/feedback")
				require.NoError(b, err)
				require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
			}
			b.StopTimer()
		})
	}
}

func BenchmarkGetFeedback(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			response := make([]*resty.Response, b.N)
			client := resty.New()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				response[i], err = client.R().
					Get(s.address + fmt.Sprintf("/api/feedback?n=%d", batchSize))
				require.NoError(b, err)
			}
			b.StopTimer()
			for _, r := range response {
				require.Equal(b, http.StatusOK, r.StatusCode())
				var it FeedbackIterator
				err := json.Unmarshal(r.Body(), &it)
				require.NoError(b, err)
				require.Equal(b, batchSize, len(it.Feedback))
			}
		})
	}
}

func BenchmarkGetUserItemFeedback(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	response := make([]*resty.Response, b.N)
	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userIndex := i % numInitUsers
		itemIndex := i % numInitItems
		var err error
		response[i], err = client.R().
			Get(s.address + fmt.Sprintf("/api/feedback/init_user_%d/init_item_%d", userIndex, itemIndex))
		require.NoError(b, err)
	}
	b.StopTimer()

	for _, r := range response {
		require.Equal(b, http.StatusOK, r.StatusCode())
		var feedback []data.Feedback
		err := json.Unmarshal(r.Body(), &feedback)
		require.NoError(b, err)
		require.Equal(b, 1, len(feedback))
	}
}

func BenchmarkDeleteUserItemFeedback(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userIndex := i % numInitUsers
		itemIndex := i % numInitItems
		r, err := client.R().
			Delete(s.address + fmt.Sprintf("/api/feedback/init_user_%d/init_item_%d", userIndex, itemIndex))
		require.NoError(b, err)
		require.Equal(b, http.StatusOK, r.StatusCode())
	}
	b.StopTimer()
}

func BenchmarkGetUserFeedback(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	response := make([]*resty.Response, b.N)
	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		response[i], err = client.R().
			Get(s.address + fmt.Sprintf("/api/user/init_user_%d/feedback", i%numInitUsers))
		require.NoError(b, err)
	}
	b.StopTimer()

	for _, r := range response {
		require.Equal(b, http.StatusOK, r.StatusCode())
		var feedback []data.Feedback
		err := json.Unmarshal(r.Body(), &feedback)
		require.NoError(b, err)
		require.Equal(b, 1, len(feedback))
	}
}

func BenchmarkGetItemFeedback(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	response := make([]*resty.Response, b.N)
	client := resty.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		response[i], err = client.R().
			Get(s.address + fmt.Sprintf("/api/item/init_item_%d/feedback", i%numInitItems))
		require.NoError(b, err)
	}
	b.StopTimer()

	for _, r := range response {
		require.Equal(b, http.StatusOK, r.StatusCode())
		var feedback []data.Feedback
		err := json.Unmarshal(r.Body(), &feedback)
		require.NoError(b, err)
		require.GreaterOrEqual(b, 1, len(feedback))
	}
}

func BenchmarkGetRecommendCache(b *testing.B) {
	s := newBenchServer(b)
	defer s.Close(b)

	ctx := context.Background()
	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			documents := make([]cache.Score, batchSize)
			for i := range documents {
				documents[i].Id = strconv.Itoa(i)
				documents[i].Score = float64(i)
				documents[i].Categories = []string{""}
			}
			lo.Reverse(documents)
			err := s.CacheClient.AddScores(ctx, cache.NonPersonalized, cache.Popular, documents)
			require.NoError(b, err)
			s.Config.Recommend.CacheSize = len(documents)

			response := make([]*resty.Response, b.N)
			client := resty.New()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response[i], err = client.R().Get(s.address + fmt.Sprintf("/api/popular?n=%d", batchSize))
				require.NoError(b, err)
			}
			b.StopTimer()

			for _, r := range response {
				require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
				expected, err := json.Marshal(documents[:batchSize])
				require.NoError(b, err)
				require.JSONEq(b, string(expected), r.String())
			}
		})
	}
}

func BenchmarkRecommendFromOfflineCache(b *testing.B) {
	log.CloseLogger()
	s := newBenchServer(b)
	defer s.Close(b)

	ctx := context.Background()
	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			documents := make([]cache.Score, batchSize*2)
			expects := make([]string, batchSize)
			feedbacks := make([]data.Feedback, batchSize)
			for i := range documents {
				documents[i].Id = strconv.Itoa(i)
				documents[i].Score = float64(i)
				documents[i].Categories = []string{""}
				if i%2 == 0 {
					expects[i/2] = documents[i].Id
				} else {
					feedbacks[i/2].FeedbackType = "read"
					feedbacks[i/2].UserId = "init_user_1"
					feedbacks[i/2].ItemId = documents[i].Id
				}
			}
			lo.Reverse(documents)
			lo.Reverse(expects)
			err := s.CacheClient.AddScores(ctx, cache.OfflineRecommend, "init_user_1", documents)
			require.NoError(b, err)
			err = s.DataClient.BatchInsertFeedback(ctx, feedbacks, true, true, true)
			require.NoError(b, err)
			s.Config.Recommend.CacheSize = len(documents)

			response := make([]*resty.Response, b.N)
			client := resty.New()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response[i], err = client.R().Get(s.address + fmt.Sprintf("/api/recommend/init_user_1?n=%d", batchSize))
				require.NoError(b, err)
			}
			b.StopTimer()

			for _, r := range response {
				require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
				var ret []string
				err = json.Unmarshal(r.Body(), &ret)
				require.NoError(b, err)
				require.Equal(b, expects[:batchSize], ret)
			}
		})
	}
}

func BenchmarkRecommendFromLatest(b *testing.B) {
	ctx := context.Background()
	log.CloseLogger()
	s := newBenchServer(b)
	defer s.Close(b)

	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			documents := make([]cache.Score, batchSize*2)
			expects := make([]string, batchSize)
			feedbacks := make([]data.Feedback, batchSize)
			for i := range documents {
				documents[i].Id = strconv.Itoa(i)
				documents[i].Score = float64(i)
				documents[i].Categories = []string{""}
				if i%2 == 0 {
					expects[i/2] = documents[i].Id
				} else {
					feedbacks[i/2].FeedbackType = "feedback_type"
					feedbacks[i/2].UserId = "init_user_1"
					feedbacks[i/2].ItemId = documents[i].Id
				}
			}
			lo.Reverse(documents)
			lo.Reverse(expects)
			err := s.CacheClient.AddScores(ctx, cache.NonPersonalized, cache.Latest, documents)
			require.NoError(b, err)
			err = s.DataClient.BatchInsertFeedback(ctx, feedbacks, true, true, true)
			require.NoError(b, err)
			s.Config.Recommend.CacheSize = len(documents)

			response := make([]*resty.Response, b.N)
			client := resty.New()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response[i], err = client.R().Get(s.address + fmt.Sprintf("/api/recommend/init_user_1?n=%d", batchSize))
				require.NoError(b, err)
			}
			b.StopTimer()

			for _, r := range response {
				require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
				var ret []string
				err = json.Unmarshal(r.Body(), &ret)
				require.NoError(b, err)
				require.Equal(b, expects[:batchSize], ret)
			}
		})
	}
}

func BenchmarkRecommendFromItemBased(b *testing.B) {
	ctx := context.Background()
	log.CloseLogger()
	s := newBenchServer(b)
	s.Config.Recommend.ItemToItem = []config.ItemToItemConfig{{Name: "default"}}
	defer s.Close(b)

	for batchSize := 10; batchSize <= 1000; batchSize *= 10 {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			// insert user feedbacks
			documents := make([]cache.Score, batchSize*2)
			for i := range documents {
				documents[i].Id = fmt.Sprintf("init_item_%d", i)
				documents[i].Score = float64(i)
				documents[i].Categories = []string{""}
				if i < s.Config.Recommend.Online.NumFeedbackFallbackItemBased {
					err := s.DataClient.BatchInsertFeedback(ctx, []data.Feedback{{
						FeedbackKey: data.FeedbackKey{
							FeedbackType: "feedback_type_positive",
							UserId:       "init_user_1",
							ItemId:       fmt.Sprintf("init_item_%d", i),
						},
						Timestamp: time.Now().Add(-time.Minute),
					}}, true, true, true)
					require.NoError(b, err)
				}
			}

			// insert user neighbors
			for i := 0; i < s.Config.Recommend.Online.NumFeedbackFallbackItemBased; i++ {
				err := s.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", fmt.Sprintf("init_item_%d", i)), documents)
				require.NoError(b, err)
			}

			s.Config.Recommend.CacheSize = len(documents)
			s.Config.Recommend.DataSource.PositiveFeedbackTypes = []string{"feedback_type_positive"}
			s.Config.Recommend.Online.FallbackRecommend = []string{"item_based"}

			response := make([]*resty.Response, b.N)
			client := resty.New()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				response[i], err = client.R().Get(s.address + fmt.Sprintf("/api/recommend/init_user_1?n=%d", batchSize))
				require.NoError(b, err)
			}
			b.StopTimer()

			for _, r := range response {
				require.Equal(b, http.StatusOK, r.StatusCode(), r.String())
				var ret []string
				err := json.Unmarshal(r.Body(), &ret)
				require.NoError(b, err)
				require.Equal(b, batchSize, len(ret))
			}
		})
	}
}
