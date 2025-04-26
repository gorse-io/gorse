// Copyright 2020 gorse Project Authors
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

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/google/uuid"
	"github.com/juju/errors"
	"github.com/lafikl/consistent"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/lo"
	"github.com/thoas/go-funk"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/heap"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/cmd/version"
	encoding2 "github.com/zhenghaoz/gorse/common/encoding"
	"github.com/zhenghaoz/gorse/common/parallel"
	"github.com/zhenghaoz/gorse/common/sizeof"
	"github.com/zhenghaoz/gorse/common/util"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/logics"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

const (
	batchSize                 = 10000
	recommendComplexityFactor = 100
)

type ScheduleState struct {
	IsRunning bool      `json:"is_running"`
	StartTime time.Time `json:"start_time"`
}

// Worker manages states of a worker node.
type Worker struct {
	tracer      *progress.Tracer
	oneMode     bool
	testMode    bool
	managedMode bool
	*config.Settings

	// spawned rankers
	rankers []click.FactorizationMachine

	// worker config
	jobs       int
	workerName string
	httpHost   string
	httpPort   int
	masterHost string
	masterPort int
	tlsConfig  *util.TLSConfig
	cacheFile  string

	// database connection path
	cachePath   string
	cachePrefix string
	dataPath    string
	dataPrefix  string

	// master connection
	conn         *grpc.ClientConn
	masterClient protocol.MasterClient

	latestRankingModelVersion int64
	latestClickModelVersion   int64
	matrixFactorization       *logics.MatrixFactorization
	randGenerator             *rand.Rand

	// peers
	peers []string
	me    string

	// scheduler state
	scheduleState ScheduleState

	// events
	tickDuration time.Duration
	ticker       *time.Ticker
	syncedChan   *parallel.ConditionChannel // meta synced events
	pulledChan   *parallel.ConditionChannel // model pulled events
	triggerChan  *parallel.ConditionChannel // manually triggered events
}

// NewWorker creates a new worker node.
func NewWorker(
	masterHost string,
	masterPort int,
	httpHost string,
	httpPort int,
	jobs int,
	cacheFile string,
	managedMode bool,
	tlsConfig *util.TLSConfig,
) *Worker {
	return &Worker{
		rankers:       make([]click.FactorizationMachine, jobs),
		managedMode:   managedMode,
		Settings:      config.NewSettings(),
		randGenerator: base.NewRand(time.Now().UTC().UnixNano()),
		// config
		cacheFile:  cacheFile,
		masterHost: masterHost,
		masterPort: masterPort,
		tlsConfig:  tlsConfig,
		httpHost:   httpHost,
		httpPort:   httpPort,
		jobs:       jobs,
		// events
		tickDuration: time.Minute,
		ticker:       time.NewTicker(time.Minute),
		syncedChan:   parallel.NewConditionChannel(),
		pulledChan:   parallel.NewConditionChannel(),
		triggerChan:  parallel.NewConditionChannel(),
	}
}

func (w *Worker) SetOneMode(settings *config.Settings) {
	w.oneMode = true
	w.Settings = settings
}

// Sync this worker to the master.
func (w *Worker) Sync() {
	defer base.CheckPanic()
	log.Logger().Info("start meta sync", zap.Duration("meta_timeout", w.Config.Master.MetaTimeout))
	for {
		var meta *protocol.Meta
		var err error
		if meta, err = w.masterClient.GetMeta(context.Background(),
			&protocol.NodeInfo{
				NodeType:      protocol.NodeType_Worker,
				Uuid:          w.workerName,
				BinaryVersion: version.Version,
				Hostname:      lo.Must(os.Hostname()),
			}); err != nil {
			log.Logger().Error("failed to get meta", zap.Error(err))
			goto sleep
		}

		// load master config
		w.Config.Recommend.Offline.Lock()
		err = json.Unmarshal([]byte(meta.Config), &w.Config)
		if err != nil {
			w.Config.Recommend.Offline.UnLock()
			log.Logger().Error("failed to parse master config", zap.Error(err))
			goto sleep
		}
		w.Config.Recommend.Offline.UnLock()

		// reset ticker
		if w.tickDuration != w.Config.Recommend.Offline.CheckRecommendPeriod {
			w.tickDuration = w.Config.Recommend.Offline.CheckRecommendPeriod
			w.ticker.Reset(w.Config.Recommend.Offline.CheckRecommendPeriod)
		}

		// connect to data store
		if w.dataPath != w.Config.Database.DataStore || w.dataPrefix != w.Config.Database.DataTablePrefix {
			if strings.HasPrefix(w.Config.Database.DataStore, storage.SQLitePrefix) {
				log.Logger().Info("connect data store via master")
				w.DataClient = data.NewProxyClient(w.conn)
			} else {
				log.Logger().Info("connect data store",
					zap.String("database", log.RedactDBURL(w.Config.Database.DataStore)))
				if w.DataClient, err = data.Open(w.Config.Database.DataStore, w.Config.Database.DataTablePrefix); err != nil {
					log.Logger().Error("failed to connect data store", zap.Error(err))
					goto sleep
				}
			}
			w.dataPath = w.Config.Database.DataStore
			w.dataPrefix = w.Config.Database.DataTablePrefix
		}

		// connect to cache store
		if w.cachePath != w.Config.Database.CacheStore || w.cachePrefix != w.Config.Database.CacheTablePrefix {
			if strings.HasPrefix(w.Config.Database.CacheStore, storage.SQLitePrefix) {
				log.Logger().Info("connect cache store via master")
				w.CacheClient = cache.NewProxyClient(w.conn)
			} else {
				log.Logger().Info("connect cache store",
					zap.String("database", log.RedactDBURL(w.Config.Database.CacheStore)))
				if w.CacheClient, err = cache.Open(w.Config.Database.CacheStore, w.Config.Database.CacheTablePrefix); err != nil {
					log.Logger().Error("failed to connect cache store", zap.Error(err))
					goto sleep
				}
			}
			w.cachePath = w.Config.Database.CacheStore
			w.cachePrefix = w.Config.Database.CacheTablePrefix
		}

		// check ranking model version
		w.latestRankingModelVersion = meta.RankingModelVersion
		if w.latestRankingModelVersion != w.RankingModelVersion {
			log.Logger().Info("new ranking model found",
				zap.String("old_version", encoding.Hex(w.RankingModelVersion)),
				zap.String("new_version", encoding.Hex(w.latestRankingModelVersion)))
			w.syncedChan.Signal()
		}

		// check click model version
		w.latestClickModelVersion = meta.ClickModelVersion
		if w.latestClickModelVersion != w.ClickModelVersion {
			log.Logger().Info("new click model found",
				zap.String("old_version", encoding.Hex(w.ClickModelVersion)),
				zap.String("new_version", encoding.Hex(w.latestClickModelVersion)))
			w.syncedChan.Signal()
		}

		w.peers = meta.Workers
		w.me = meta.Me
	sleep:
		if w.testMode {
			return
		}
		time.Sleep(w.Config.Master.MetaTimeout)
	}
}

// Pull user index and ranking model from master.
func (w *Worker) Pull() {
	defer base.CheckPanic()
	for range w.syncedChan.C {
		pulled := false

		// pull ranking model
		if w.latestRankingModelVersion != w.RankingModelVersion {
			log.Logger().Info("start pull ranking model")
			if rankingModelReceiver, err := w.masterClient.GetRankingModel(context.Background(),
				&protocol.VersionInfo{Version: w.latestRankingModelVersion},
				grpc.MaxCallRecvMsgSize(math.MaxInt)); err != nil {
				log.Logger().Error("failed to pull ranking model", zap.Error(err))
			} else {
				var rankingModel cf.MatrixFactorization
				rankingModel, err = encoding2.UnmarshalRankingModel(rankingModelReceiver)
				if err != nil {
					log.Logger().Error("failed to unmarshal ranking model", zap.Error(err))
				} else {
					w.RankingModel = rankingModel
					w.matrixFactorization = nil
					w.RankingModelVersion = w.latestRankingModelVersion
					log.Logger().Info("synced ranking model",
						zap.String("version", encoding.Hex(w.RankingModelVersion)))
					MemoryInuseBytesVec.WithLabelValues("collaborative_filtering_model").Set(float64(sizeof.DeepSize(w.RankingModel)))
					pulled = true
				}
			}
		}

		// pull click model
		if w.latestClickModelVersion != w.ClickModelVersion {
			log.Logger().Info("start pull click model")
			if clickModelReceiver, err := w.masterClient.GetClickModel(context.Background(),
				&protocol.VersionInfo{Version: w.latestClickModelVersion},
				grpc.MaxCallRecvMsgSize(math.MaxInt)); err != nil {
				log.Logger().Error("failed to pull click model", zap.Error(err))
			} else {
				var clickModel click.FactorizationMachine
				clickModel, err = encoding2.UnmarshalClickModel(clickModelReceiver)
				if err != nil {
					log.Logger().Error("failed to unmarshal click model", zap.Error(err))
				} else {
					w.ClickModel = clickModel
					w.ClickModelVersion = w.latestClickModelVersion
					log.Logger().Info("synced click model",
						zap.String("version", encoding.Hex(w.ClickModelVersion)))
					MemoryInuseBytesVec.WithLabelValues("ranking_model").Set(float64(sizeof.DeepSize(w.ClickModel)))

					// spawn rankers
					for i := 0; i < w.jobs; i++ {
						if i == 0 {
							w.rankers[i] = w.ClickModel
						} else {
							w.rankers[i] = click.Spawn(w.ClickModel)
						}
					}

					pulled = true
				}
			}
		}

		if w.testMode {
			return
		}
		if pulled {
			w.pulledChan.Signal()
		}
	}
}

// ServeHTTP serves Prometheus metrics and API.
func (w *Worker) ServeHTTP() {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/api/health/live", w.checkLive)
	http.HandleFunc("/api/admin/schedule", w.ScheduleAPIHandler)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", w.httpHost, w.httpPort), nil)
	if err != nil {
		log.Logger().Fatal("failed to start http server", zap.Error(err))
	}
}

func (w *Worker) ScheduleAPIHandler(writer http.ResponseWriter, request *http.Request) {
	if !w.checkAdmin(request) {
		writeError(writer, "unauthorized", http.StatusMethodNotAllowed)
		return
	}
	switch request.Method {
	case http.MethodGet:
		writer.WriteHeader(http.StatusOK)
		bytes, err := json.Marshal(w.scheduleState)
		if err != nil {
			writeError(writer, err.Error(), http.StatusInternalServerError)
		}
		if _, err = writer.Write(bytes); err != nil {
			writeError(writer, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		w.triggerChan.Signal()
	default:
		writeError(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (w *Worker) checkAdmin(request *http.Request) bool {
	if w.Config.Master.AdminAPIKey == "" {
		return true
	}
	if request.FormValue("X-API-Key") == w.Config.Master.AdminAPIKey {
		return true
	}
	return false
}

func writeJSON(w http.ResponseWriter, content any) {
	w.WriteHeader(http.StatusOK)
	bytes, err := json.Marshal(content)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
	}
	if _, err = w.Write(bytes); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, error string, code int) {
	log.Logger().Error(strings.ToLower(http.StatusText(code)), zap.String("error", error))
	http.Error(w, error, code)
}

// Serve as a worker node.
func (w *Worker) Serve() {
	// open local store
	if !w.oneMode {
		state, err := LoadLocalCache(w.cacheFile)
		if err != nil {
			if errors.Is(err, errors.NotFound) {
				log.Logger().Info("no cache file found, create a new one", zap.String("path", state.path))
			} else {
				log.Logger().Error("failed to load persist state", zap.Error(err),
					zap.String("path", w.cacheFile))
			}
		}
		if state.WorkerName == "" {
			state.WorkerName = uuid.New().String()
			err = state.WriteLocalCache()
			if err != nil {
				log.Logger().Fatal("failed to write meta", zap.Error(err))
			}
		}
		w.workerName = state.WorkerName
		log.Logger().Info("start worker",
			zap.Bool("managed", w.managedMode),
			zap.Int("n_jobs", w.jobs),
			zap.String("worker_name", w.workerName))
	}

	// create progress tracer
	w.tracer = progress.NewTracer(w.workerName)

	// connect to master
	var opts []grpc.DialOption
	if w.tlsConfig != nil {
		c, err := util.NewClientCreds(w.tlsConfig)
		if err != nil {
			log.Logger().Fatal("failed to create credentials", zap.Error(err))
		}
		opts = append(opts, grpc.WithTransportCredentials(c))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	var err error
	w.conn, err = grpc.Dial(fmt.Sprintf("%v:%v", w.masterHost, w.masterPort), opts...)
	if err != nil {
		log.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	w.masterClient = protocol.NewMasterClient(w.conn)

	if w.oneMode {
		w.peers = []string{w.workerName}
		w.me = w.workerName
	} else {
		go w.Sync()
		go w.Pull()
		go w.ServeHTTP()
	}

	loop := func() {
		w.scheduleState.IsRunning = true
		w.scheduleState.StartTime = time.Now()
		defer func() {
			w.scheduleState.IsRunning = false
			w.scheduleState.StartTime = time.Time{}
		}()

		// pull users
		workingUsers, err := w.pullUsers(w.peers, w.me)
		if err != nil {
			log.Logger().Error("failed to split users", zap.Error(err),
				zap.String("me", w.me),
				zap.Strings("workers", w.peers))
			return
		}

		// recommendation
		w.Recommend(workingUsers)
	}

	if w.managedMode {
		for range w.triggerChan.C {
			loop()
		}
	} else {
		for {
			select {
			case tick := <-w.ticker.C:
				if time.Since(tick) < w.Config.Recommend.Offline.CheckRecommendPeriod {
					loop()
				}
			case <-w.pulledChan.C:
				loop()
			}
		}
	}
}

// Recommend items to users. The workflow of recommendation is:
// 1. Skip inactive users.
// 2. Load historical items.
// 3. Load positive items if KNN used.
// 4. Generate recommendation.
// 5. Save result.
// 6. Insert cold-start items into results.
// 7. Rank items in results by click-through-rate.
// 8. Refresh cache.
func (w *Worker) Recommend(users []data.User) {
	ctx := context.Background()
	startRecommendTime := time.Now()
	log.Logger().Info("ranking recommendation",
		zap.Int("n_working_users", len(users)),
		zap.Int("n_jobs", w.jobs),
		zap.Int("cache_size", w.Config.Recommend.CacheSize))

	// pull items from database
	itemCache, itemCategories, err := w.pullItems(ctx)
	if err != nil {
		log.Logger().Error("failed to pull items", zap.Error(err))
		return
	}
	MemoryInuseBytesVec.WithLabelValues("item_cache").Set(float64(sizeof.DeepSize(itemCache)))
	defer MemoryInuseBytesVec.WithLabelValues("item_cache").Set(0)

	// progress tracker
	completed := make(chan struct{}, 1000)
	_, span := w.tracer.Start(context.Background(), "Recommend", len(users))
	defer span.End()

	go func() {
		defer base.CheckPanic()
		completedCount, previousCount := 0, 0
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case _, ok := <-completed:
				if !ok {
					return
				}
				completedCount++
			case <-ticker.C:
				throughput := completedCount - previousCount
				previousCount = completedCount
				if throughput > 0 {
					if w.masterClient != nil {
						span.Add(throughput)
					}
					log.Logger().Info("ranking recommendation",
						zap.Int("n_complete_users", completedCount),
						zap.Int("n_working_users", len(users)),
						zap.Int("throughput", throughput))
				}
				if _, err := w.masterClient.PushProgress(context.Background(), protocol.EncodeProgress(w.tracer.List())); err != nil {
					log.Logger().Error("failed to report update task", zap.Error(err))
				}
			}
		}
	}()

	// build ranking index
	if w.RankingModel != nil && !w.RankingModel.Invalid() && w.matrixFactorization == nil {
		startTime := time.Now()
		log.Logger().Info("start building ranking index")
		itemIndex := w.RankingModel.GetItemIndex()
		matrixFactorization := logics.NewMatrixFactorization(time.Now())
		for i := int32(0); i < itemIndex.Count(); i++ {
			if itemId, ok := itemIndex.String(i); ok && itemCache.IsAvailable(itemId) {
				item, _ := itemCache.Get(itemId)
				matrixFactorization.Add(item, w.RankingModel.GetItemFactor(int32(i)))
			}
		}
		w.matrixFactorization = matrixFactorization
		log.Logger().Info("complete building ranking index",
			zap.Duration("build_time", time.Since(startTime)))
	}

	// recommendation
	startTime := time.Now()
	var (
		updateUserCount               atomic.Float64
		collaborativeRecommendSeconds atomic.Float64
		userBasedRecommendSeconds     atomic.Float64
		itemBasedRecommendSeconds     atomic.Float64
		latestRecommendSeconds        atomic.Float64
		popularRecommendSeconds       atomic.Float64
	)

	userFeedbackCache := NewFeedbackCache(w, w.Config.Recommend.DataSource.PositiveFeedbackTypes...)
	defer MemoryInuseBytesVec.WithLabelValues("user_feedback_cache").Set(0)
	err = parallel.Parallel(len(users), w.jobs, func(workerId, jobId int) error {
		defer func() {
			completed <- struct{}{}
		}()
		user := users[jobId]
		userId := user.UserId
		// skip inactive users before max recommend period
		if !w.checkUserActiveTime(ctx, userId) || !w.checkRecommendCacheTimeout(ctx, userId, itemCategories) {
			return nil
		}
		updateUserCount.Add(1)

		// load historical items
		historyItems, feedbacks, err := w.loadUserHistoricalItems(w.DataClient, userId)
		excludeSet := mapset.NewSet(historyItems...)
		if err != nil {
			log.Logger().Error("failed to pull user feedback",
				zap.String("user_id", userId), zap.Error(err))
			return errors.Trace(err)
		}

		// load positive items
		var positiveItems []string
		if w.Config.Recommend.Offline.EnableItemBasedRecommend {
			positiveItems, err = userFeedbackCache.GetUserFeedback(ctx, userId)
			if err != nil {
				log.Logger().Error("failed to pull user feedback",
					zap.String("user_id", userId), zap.Error(err))
				return errors.Trace(err)
			}
			MemoryInuseBytesVec.WithLabelValues("user_feedback_cache").Set(float64(sizeof.DeepSize(userFeedbackCache)))
		}

		// create candidates container
		candidates := make(map[string][][]string)
		candidates[""] = make([][]string, 0)
		for _, category := range itemCategories {
			candidates[category] = make([][]string, 0)
		}

		// Recommender #1: collaborative filtering.
		collaborativeUsed := false
		if w.Config.Recommend.Offline.EnableColRecommend && w.RankingModel != nil && !w.RankingModel.Invalid() {
			if userIndex := w.RankingModel.GetUserIndex().Id(userId); w.RankingModel.IsUserPredictable(int32(userIndex)) {
				var recommend map[string][]string
				var usedTime time.Duration
				recommend, usedTime, err = w.collaborativeRecommendHNSW(w.matrixFactorization, userId, excludeSet, itemCache)
				if err != nil {
					log.Logger().Error("failed to recommend by collaborative filtering",
						zap.String("user_id", userId), zap.Error(err))
					return errors.Trace(err)
				}
				for category, items := range recommend {
					candidates[category] = append(candidates[category], items)
				}
				collaborativeUsed = true
				collaborativeRecommendSeconds.Add(usedTime.Seconds())
			} else if !w.RankingModel.IsUserPredictable(int32(userIndex)) {
				log.Logger().Debug("user is unpredictable", zap.String("user_id", userId))
			}
		} else if w.RankingModel == nil || w.RankingModel.Invalid() {
			log.Logger().Debug("no collaborative filtering model")
		}

		// Recommender #2: item-based.
		itemNeighborDigests := mapset.NewSet[string]()
		if w.Config.Recommend.Offline.EnableItemBasedRecommend {
			localStartTime := time.Now()
			// collect candidates
			scores := make(map[string]float64)
			for _, itemId := range positiveItems {
				// load similar items
				similarItems, err := w.CacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key(cache.Neighbors, itemId), nil, 0, w.Config.Recommend.CacheSize)
				if err != nil {
					log.Logger().Error("failed to load similar items", zap.Error(err))
					return errors.Trace(err)
				}
				// add unseen items
				for _, item := range similarItems {
					if !excludeSet.Contains(item.Id) && itemCache.IsAvailable(item.Id) {
						scores[item.Id] += item.Score
					}
				}
				// load item neighbors digest
				digest, err := w.CacheClient.Get(ctx, cache.Key(cache.ItemToItemDigest, cache.Neighbors, itemId)).String()
				if err != nil {
					if !errors.Is(err, errors.NotFound) {
						log.Logger().Error("failed to load item neighbors digest", zap.Error(err))
						return errors.Trace(err)
					}
				}
				itemNeighborDigests.Add(digest)
			}
			// collect top k
			filter := heap.NewTopKFilter[string, float64](w.Config.Recommend.CacheSize)
			for id, score := range scores {
				filter.Push(id, score)
			}
			ids, _ := filter.PopAll()
			recommend := make(map[string][]string)
			for _, id := range ids {
				recommend[""] = append(recommend[""], id)
				for _, category := range itemCache.GetCategory(id) {
					recommend[category] = append(recommend[category], id)
				}
			}
			for category, items := range recommend {
				candidates[category] = append(candidates[category], items)
			}
			itemBasedRecommendSeconds.Add(time.Since(localStartTime).Seconds())
		}

		// Recommender #3: insert user-based items
		userNeighborDigests := mapset.NewSet[string]()
		if w.Config.Recommend.Offline.EnableUserBasedRecommend {
			localStartTime := time.Now()
			scores := make(map[string]float64)
			// load similar users
			similarUsers, err := w.CacheClient.SearchScores(ctx, cache.UserToUser, cache.Key(cache.Neighbors, userId), nil, 0, w.Config.Recommend.CacheSize)
			if err != nil {
				log.Logger().Error("failed to load similar users", zap.Error(err))
				return errors.Trace(err)
			}
			for _, user := range similarUsers {
				// load historical feedback
				similarUserPositiveItems, err := userFeedbackCache.GetUserFeedback(ctx, user.Id)
				if err != nil {
					log.Logger().Error("failed to pull user feedback",
						zap.String("user_id", userId), zap.Error(err))
					return errors.Trace(err)
				}
				MemoryInuseBytesVec.WithLabelValues("user_feedback_cache").Set(float64(sizeof.DeepSize(userFeedbackCache)))
				// add unseen items
				for _, itemId := range similarUserPositiveItems {
					if !excludeSet.Contains(itemId) && itemCache.IsAvailable(itemId) {
						scores[itemId] += user.Score
					}
				}
				// load user neighbors digest
				digest, err := w.CacheClient.Get(ctx, cache.Key(cache.UserToUserDigest, cache.Neighbors, user.Id)).String()
				if err != nil {
					if !errors.Is(err, errors.NotFound) {
						log.Logger().Error("failed to load user neighbors digest", zap.Error(err))
						return errors.Trace(err)
					}
				}
				userNeighborDigests.Add(digest)
			}
			// collect top k
			filters := make(map[string]*heap.TopKFilter[string, float64])
			filters[""] = heap.NewTopKFilter[string, float64](w.Config.Recommend.CacheSize)
			for _, category := range itemCategories {
				filters[category] = heap.NewTopKFilter[string, float64](w.Config.Recommend.CacheSize)
			}
			for id, score := range scores {
				filters[""].Push(id, score)
				for _, category := range itemCache.GetCategory(id) {
					filters[category].Push(id, score)
				}
			}
			for category, filter := range filters {
				ids, _ := filter.PopAll()
				candidates[category] = append(candidates[category], ids)
			}
			userBasedRecommendSeconds.Add(time.Since(localStartTime).Seconds())
		}

		// Recommender #4: latest items.
		if w.Config.Recommend.Offline.EnableLatestRecommend {
			localStartTime := time.Now()
			for _, category := range append([]string{""}, itemCategories...) {
				latestItems, err := w.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Latest, []string{category}, 0, w.Config.Recommend.CacheSize)
				if err != nil {
					log.Logger().Error("failed to load latest items", zap.Error(err))
					return errors.Trace(err)
				}
				var recommend []string
				for _, latestItem := range latestItems {
					if !excludeSet.Contains(latestItem.Id) && itemCache.IsAvailable(latestItem.Id) {
						recommend = append(recommend, latestItem.Id)
					}
				}
				candidates[category] = append(candidates[category], recommend)
			}
			latestRecommendSeconds.Add(time.Since(localStartTime).Seconds())
		}

		// Recommender #5: popular items.
		if w.Config.Recommend.Offline.EnablePopularRecommend {
			localStartTime := time.Now()
			for _, category := range append([]string{""}, itemCategories...) {
				popularItems, err := w.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Popular, []string{category}, 0, w.Config.Recommend.CacheSize)
				if err != nil {
					log.Logger().Error("failed to load popular items", zap.Error(err))
					return errors.Trace(err)
				}
				var recommend []string
				for _, popularItem := range popularItems {
					if !excludeSet.Contains(popularItem.Id) && itemCache.IsAvailable(popularItem.Id) {
						recommend = append(recommend, popularItem.Id)
					}
				}
				candidates[category] = append(candidates[category], recommend)
			}
			popularRecommendSeconds.Add(time.Since(localStartTime).Seconds())

			// update latest popular items timestamp
			if err = w.CacheClient.Set(ctx, cache.Time(cache.Key(cache.GlobalMeta, cache.LastUpdatePopularItemsTime), localStartTime)); err != nil {
				log.Logger().Error("failed to cache popular time", zap.Error(err))
			}
		}

		// rank items from different recommenders
		// 1. If click-through rate prediction model is available, use it to rank items.
		// 2. If collaborative filtering model is available, use it to rank items.
		// 3. Otherwise, merge all recommenders' results randomly.
		ctrUsed := false
		results := make(map[string][]cache.Score)
		for category, catCandidates := range candidates {
			if w.Config.Recommend.Offline.EnableClickThroughPrediction && w.rankers[workerId] != nil && !w.rankers[workerId].Invalid() {
				results[category], err = w.rankByClickTroughRate(&user, catCandidates, itemCache, w.rankers[workerId])
				if err != nil {
					log.Logger().Error("failed to rank items", zap.Error(err))
					return errors.Trace(err)
				}
				ctrUsed = true
			} else if w.RankingModel != nil && !w.RankingModel.Invalid() &&
				w.RankingModel.IsUserPredictable(w.RankingModel.GetUserIndex().Id(userId)) {
				results[category], err = w.rankByCollaborativeFiltering(userId, catCandidates)
				if err != nil {
					log.Logger().Error("failed to rank items", zap.Error(err))
					return errors.Trace(err)
				}
			} else {
				results[category] = w.mergeAndShuffle(catCandidates)
			}
		}

		// replacement
		if w.Config.Recommend.Replacement.EnableReplacement {
			if results, err = w.replacement(results, &user, feedbacks, itemCache); err != nil {
				log.Logger().Error("failed to replace items", zap.Error(err))
				return errors.Trace(err)
			}
		}

		// explore latest and popular
		recommendTime := time.Now()
		aggregator := cache.NewDocumentAggregator(recommendTime)
		for category, result := range results {
			scores, err := w.exploreRecommend(result, excludeSet, category)
			if err != nil {
				log.Logger().Error("failed to explore latest and popular items", zap.Error(err))
				return errors.Trace(err)
			}
			aggregator.Add(category, lo.Map(scores, func(document cache.Score, _ int) string {
				return document.Id
			}), lo.Map(scores, func(document cache.Score, _ int) float64 {
				return document.Score
			}))
		}
		if err = w.CacheClient.AddScores(ctx, cache.OfflineRecommend, userId, aggregator.ToSlice()); err != nil {
			log.Logger().Error("failed to cache recommendation", zap.Error(err))
			return errors.Trace(err)
		}
		if err = w.CacheClient.Set(
			ctx,
			cache.Time(cache.Key(cache.LastUpdateUserRecommendTime, userId), recommendTime),
			cache.String(cache.Key(cache.OfflineRecommendDigest, userId), w.Config.OfflineRecommendDigest(
				config.WithCollaborative(collaborativeUsed),
				config.WithRanking(ctrUsed),
				config.WithItemNeighborDigest(strings.Join(itemNeighborDigests.ToSlice(), "-")),
				config.WithUserNeighborDigest(strings.Join(userNeighborDigests.ToSlice(), "-")),
			))); err != nil {
			log.Logger().Error("failed to cache recommendation time", zap.Error(err))
		}
		return nil
	})
	close(completed)
	if err != nil {
		log.Logger().Error("failed to continue offline recommendation", zap.Error(err))
		return
	}
	log.Logger().Info("complete ranking recommendation",
		zap.String("used_time", time.Since(startTime).String()))
	UpdateUserRecommendTotal.Set(updateUserCount.Load())
	OfflineRecommendTotalSeconds.Set(time.Since(startRecommendTime).Seconds())
	OfflineRecommendStepSecondsVec.WithLabelValues("collaborative_recommend").Set(collaborativeRecommendSeconds.Load())
	OfflineRecommendStepSecondsVec.WithLabelValues("item_based_recommend").Set(itemBasedRecommendSeconds.Load())
	OfflineRecommendStepSecondsVec.WithLabelValues("user_based_recommend").Set(userBasedRecommendSeconds.Load())
	OfflineRecommendStepSecondsVec.WithLabelValues("latest_recommend").Set(latestRecommendSeconds.Load())
	OfflineRecommendStepSecondsVec.WithLabelValues("popular_recommend").Set(popularRecommendSeconds.Load())
}

func (w *Worker) collaborativeRecommendHNSW(rankingIndex *logics.MatrixFactorization, userId string, excludeSet mapset.Set[string], itemCache *ItemCache) (map[string][]string, time.Duration, error) {
	ctx := context.Background()
	userIndex := w.RankingModel.GetUserIndex().Id(userId)
	localStartTime := time.Now()
	scores := rankingIndex.Search(w.RankingModel.GetUserFactor(userIndex), w.Config.Recommend.CacheSize+excludeSet.Cardinality())
	// save result
	recommend := make(map[string][]string)
	for i := range scores {
		// the scores use the timestamp of the ranking index, which is only refreshed every so often.
		// if we don't overwrite the timestamp here, the code below will delete all scores that were
		// just written.
		scores[i].Timestamp = localStartTime
		if !excludeSet.Contains(scores[i].Id) && itemCache.IsAvailable(scores[i].Id) {
			for _, category := range scores[i].Categories {
				recommend[category] = append(recommend[category], scores[i].Id)
			}
		}
	}
	recommend[""] = lo.Map(scores, func(score cache.Score, _ int) string {
		return score.Id
	})
	if err := w.CacheClient.AddScores(ctx, cache.CollaborativeRecommend, userId, scores); err != nil {
		log.Logger().Error("failed to cache collaborative filtering recommendation result", zap.String("user_id", userId), zap.Error(err))
		return nil, 0, errors.Trace(err)
	}
	if err := w.CacheClient.DeleteScores(ctx, []string{cache.CollaborativeRecommend}, cache.ScoreCondition{Before: &localStartTime, Subset: proto.String(userId)}); err != nil {
		log.Logger().Error("failed to delete stale collaborative filtering recommendation result", zap.String("user_id", userId), zap.Error(err))
		return nil, 0, errors.Trace(err)
	}
	return recommend, time.Since(localStartTime), nil
}

func (w *Worker) rankByCollaborativeFiltering(userId string, candidates [][]string) ([]cache.Score, error) {
	// concat candidates
	memo := mapset.NewSet[string]()
	var itemIds []string
	for _, v := range candidates {
		for _, itemId := range v {
			if !memo.Contains(itemId) {
				memo.Add(itemId)
				itemIds = append(itemIds, itemId)
			}
		}
	}
	// rank by collaborative filtering
	topItems := make([]cache.Score, 0, len(candidates))
	for _, itemId := range itemIds {
		topItems = append(topItems, cache.Score{
			Id:    itemId,
			Score: float64(w.RankingModel.Predict(userId, itemId)),
		})
	}
	cache.SortDocuments(topItems)
	return topItems, nil
}

// rankByClickTroughRate ranks items by predicted click-through-rate.
func (w *Worker) rankByClickTroughRate(user *data.User, candidates [][]string, itemCache *ItemCache, predictor click.FactorizationMachine) ([]cache.Score, error) {
	// concat candidates
	memo := mapset.NewSet[string]()
	var itemIds []string
	for _, v := range candidates {
		for _, itemId := range v {
			if !memo.Contains(itemId) {
				memo.Add(itemId)
				itemIds = append(itemIds, itemId)
			}
		}
	}
	// download items
	items := make([]*data.Item, 0, len(itemIds))
	for _, itemId := range itemIds {
		if item, exist := itemCache.Get(itemId); exist {
			items = append(items, item)
		} else {
			log.Logger().Warn("item doesn't exists in database", zap.String("item_id", itemId))
		}
	}
	// rank by CTR
	topItems := make([]cache.Score, 0, len(items))
	if batchPredictor, ok := predictor.(click.BatchInference); ok {
		inputs := make([]lo.Tuple4[string, string, []click.Feature, []click.Feature], len(items))
		for i, item := range items {
			inputs[i].A = user.UserId
			inputs[i].B = item.ItemId
			inputs[i].C = click.ConvertLabelsToFeatures(user.Labels)
			inputs[i].D = click.ConvertLabelsToFeatures(item.Labels)
		}
		output := batchPredictor.BatchPredict(inputs)
		for i, score := range output {
			topItems = append(topItems, cache.Score{
				Id:    items[i].ItemId,
				Score: float64(score),
			})
		}
	} else {
		for _, item := range items {
			topItems = append(topItems, cache.Score{
				Id:    item.ItemId,
				Score: float64(predictor.Predict(user.UserId, item.ItemId, click.ConvertLabelsToFeatures(user.Labels), click.ConvertLabelsToFeatures(item.Labels))),
			})
		}
	}
	cache.SortDocuments(topItems)
	return topItems, nil
}

func (w *Worker) mergeAndShuffle(candidates [][]string) []cache.Score {
	memo := mapset.NewSet[string]()
	pos := make([]int, len(candidates))
	var recommend []cache.Score
	for {
		// filter out ended slice
		var src []int
		for i := range candidates {
			if pos[i] < len(candidates[i]) {
				src = append(src, i)
			}
		}
		if len(src) == 0 {
			break
		}
		// select a slice randomly
		j := src[w.randGenerator.Intn(len(src))]
		candidateId := candidates[j][pos[j]]
		pos[j]++
		if !memo.Contains(candidateId) {
			memo.Add(candidateId)
			recommend = append(recommend, cache.Score{Score: math.Exp(float64(-len(recommend))), Id: candidateId})
		}
	}
	return recommend
}

func (w *Worker) exploreRecommend(exploitRecommend []cache.Score, excludeSet mapset.Set[string], category string) ([]cache.Score, error) {
	var localExcludeSet mapset.Set[string]
	ctx := context.Background()
	if w.Config.Recommend.Replacement.EnableReplacement {
		localExcludeSet = mapset.NewSet[string]()
	} else {
		localExcludeSet = excludeSet.Clone()
	}
	// create thresholds
	explorePopularThreshold := 0.0
	if threshold, exist := w.Config.Recommend.Offline.GetExploreRecommend("popular"); exist {
		explorePopularThreshold = threshold
	}
	exploreLatestThreshold := explorePopularThreshold
	if threshold, exist := w.Config.Recommend.Offline.GetExploreRecommend("latest"); exist {
		exploreLatestThreshold += threshold
	}
	// load popular items
	popularItems, err := w.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Popular, []string{category}, 0, w.Config.Recommend.CacheSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// load the latest items
	latestItems, err := w.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Latest, []string{category}, 0, w.Config.Recommend.CacheSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// explore recommendation
	var exploreRecommend []cache.Score
	score := 1.0
	if len(exploitRecommend) > 0 {
		score += exploitRecommend[0].Score
	}
	for range exploitRecommend {
		dice := w.randGenerator.Float64()
		var recommendItem cache.Score
		if dice < explorePopularThreshold && len(popularItems) > 0 {
			score -= 1e-5
			recommendItem.Id = popularItems[0].Id
			recommendItem.Score = score
			popularItems = popularItems[1:]
		} else if dice < exploreLatestThreshold && len(latestItems) > 0 {
			score -= 1e-5
			recommendItem.Id = latestItems[0].Id
			recommendItem.Score = score
			latestItems = latestItems[1:]
		} else if len(exploitRecommend) > 0 {
			recommendItem = exploitRecommend[0]
			exploitRecommend = exploitRecommend[1:]
			score = recommendItem.Score
		} else {
			break
		}
		if !localExcludeSet.Contains(recommendItem.Id) {
			localExcludeSet.Add(recommendItem.Id)
			exploreRecommend = append(exploreRecommend, recommendItem)
		}
	}
	return exploreRecommend, nil
}

func (w *Worker) checkUserActiveTime(ctx context.Context, userId string) bool {
	if w.Config.Recommend.ActiveUserTTL == 0 {
		return true
	}
	// read active time
	activeTime, err := w.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read last modify user time", zap.Error(err))
		}
		return true
	}
	// check active time
	if time.Since(activeTime) < time.Duration(w.Config.Recommend.ActiveUserTTL*24)*time.Hour {
		return true
	}
	// remove recommend cache for inactive users
	if err := w.CacheClient.DeleteScores(ctx, []string{cache.OfflineRecommend, cache.CollaborativeRecommend},
		cache.ScoreCondition{Subset: proto.String(userId)}); err != nil {
		log.Logger().Error("failed to delete recommend cache", zap.String("user_id", userId), zap.Error(err))
	}
	return false
}

// checkRecommendCacheTimeout checks if recommend cache stale.
// 1. if cache is empty, stale.
// 2. if active time > recommend time, stale.
// 3. if recommend time + timeout < now, stale.
func (w *Worker) checkRecommendCacheTimeout(ctx context.Context, userId string, categories []string) bool {
	var (
		activeTime    time.Time
		recommendTime time.Time
		cacheDigest   string
		err           error
	)
	// check cache
	for _, category := range append([]string{""}, categories...) {
		items, err := w.CacheClient.SearchScores(ctx, cache.OfflineRecommend, userId, []string{category}, 0, -1)
		if err != nil {
			log.Logger().Error("failed to load offline recommendation", zap.String("user_id", userId), zap.Error(err))
			return true
		} else if len(items) == 0 {
			return true
		}
	}
	// read digest
	cacheDigest, err = w.CacheClient.Get(ctx, cache.Key(cache.OfflineRecommendDigest, userId)).String()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to load offline recommendation digest", zap.String("user_id", userId), zap.Error(err))
		}
		return true
	}
	if cacheDigest != w.Config.OfflineRecommendDigest() {
		return true
	}
	// read active time
	activeTime, err = w.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read last modify user time", zap.Error(err))
		}
		return true
	}
	// read recommend time
	recommendTime, err = w.CacheClient.Get(ctx, cache.Key(cache.LastUpdateUserRecommendTime, userId)).Time()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read last update user recommend time", zap.Error(err))
		}
		return true
	}
	// check cache expire
	if recommendTime.Before(time.Now().Add(-w.Config.Recommend.CacheExpire)) {
		return true
	}
	// check time
	if activeTime.Before(recommendTime) {
		timeoutTime := recommendTime.Add(w.Config.Recommend.Offline.RefreshRecommendPeriod)
		return timeoutTime.Before(time.Now())
	}
	return true
}

func (w *Worker) loadUserHistoricalItems(database data.Database, userId string) ([]string, []data.Feedback, error) {
	items := make([]string, 0)
	ctx := context.Background()
	feedbacks, err := database.GetUserFeedback(ctx, userId, w.Config.Now())
	if err != nil {
		return nil, nil, err
	}
	for _, feedback := range feedbacks {
		items = append(items, feedback.ItemId)
	}
	return items, feedbacks, nil
}

func (w *Worker) pullItems(ctx context.Context) (*ItemCache, []string, error) {
	// pull items from database
	itemCache := NewItemCache()
	itemCategories := mapset.NewSet[string]()
	itemChan, errChan := w.DataClient.GetItemStream(ctx, batchSize, nil)
	for batchItems := range itemChan {
		for _, item := range batchItems {
			itemCache.Set(item.ItemId, item)
			itemCategories.Append(item.Categories...)
		}
	}
	if err := <-errChan; err != nil {
		return nil, nil, errors.Trace(err)
	}
	return itemCache, itemCategories.ToSlice(), nil
}

func (w *Worker) pullUsers(peers []string, me string) ([]data.User, error) {
	ctx := context.Background()
	// locate me
	if !funk.ContainsString(peers, me) {
		return nil, errors.New("current node isn't in worker nodes")
	}
	// create consistent hash ring
	c := consistent.New()
	for _, peer := range peers {
		c.Add(peer)
	}
	// pull users from database
	var users []data.User
	userChan, errChan := w.DataClient.GetUserStream(ctx, batchSize)
	for batchUsers := range userChan {
		for _, user := range batchUsers {
			p, err := c.Get(user.UserId)
			if err != nil {
				return nil, errors.Trace(err)
			}
			if p == me {
				users = append(users, user)
			}
		}
	}
	if err := <-errChan; err != nil {
		return nil, errors.Trace(err)
	}
	return users, nil
}

// replacement inserts historical items back to recommendation.
func (w *Worker) replacement(recommend map[string][]cache.Score, user *data.User, feedbacks []data.Feedback, itemCache *ItemCache) (map[string][]cache.Score, error) {
	upperBounds := make(map[string]float64)
	lowerBounds := make(map[string]float64)
	newRecommend := make(map[string][]cache.Score)
	for category, scores := range recommend {
		// find minimal score
		if len(scores) > 0 {
			s := lo.Map(scores, func(score cache.Score, _ int) float64 {
				return score.Score
			})
			upperBounds[category] = funk.MaxFloat64(s)
			lowerBounds[category] = funk.MinFloat64(s)
		} else {
			upperBounds[category] = math.Inf(1)
			lowerBounds[category] = math.Inf(-1)
		}
		// add scores to filters
		newRecommend[category] = append(newRecommend[category], scores...)
	}

	// remove duplicates
	positiveItems := mapset.NewSet[string]()
	distinctItems := mapset.NewSet[string]()
	for _, feedback := range feedbacks {
		if funk.ContainsString(w.Config.Recommend.DataSource.PositiveFeedbackTypes, feedback.FeedbackType) {
			positiveItems.Add(feedback.ItemId)
			distinctItems.Add(feedback.ItemId)
		} else if funk.ContainsString(w.Config.Recommend.DataSource.ReadFeedbackTypes, feedback.FeedbackType) {
			distinctItems.Add(feedback.ItemId)
		}
	}

	for _, itemId := range distinctItems.ToSlice() {
		if item, exist := itemCache.Get(itemId); exist {
			// scoring item
			// 1. If click-through rate prediction model is available, use it.
			// 2. If collaborative filtering model is available, use it.
			// 3. Otherwise, give a random score.
			var score float64
			if w.Config.Recommend.Offline.EnableClickThroughPrediction && w.ClickModel != nil {
				score = float64(w.ClickModel.Predict(user.UserId, itemId, click.ConvertLabelsToFeatures(user.Labels), click.ConvertLabelsToFeatures(item.Labels)))
			} else if w.RankingModel != nil && !w.RankingModel.Invalid() && w.RankingModel.IsUserPredictable(w.RankingModel.GetUserIndex().Id(user.UserId)) {
				score = float64(w.RankingModel.Predict(user.UserId, itemId))
			} else {
				upper := upperBounds[""]
				lower := lowerBounds[""]
				if !math.IsInf(upper, 1) && !math.IsInf(lower, -1) {
					score = lower + w.randGenerator.Float64()*(upper-lower)
				} else {
					score = w.randGenerator.Float64()
				}
			}
			// replace item
			for _, category := range append([]string{""}, item.Categories...) {
				upperBound := upperBounds[category]
				lowerBound := lowerBounds[category]
				if !math.IsInf(upperBound, 1) && !math.IsInf(lowerBound, -1) {
					// decay item
					score -= lowerBound
					if score < 0 {
						continue
					} else if positiveItems.Contains(itemId) {
						score *= w.Config.Recommend.Replacement.PositiveReplacementDecay
					} else {
						score *= w.Config.Recommend.Replacement.ReadReplacementDecay
					}
					score += lowerBound
				}
				newRecommend[category] = append(newRecommend[category], cache.Score{Id: itemId, Score: score})
			}
		} else {
			log.Logger().Warn("item doesn't exists in database", zap.String("item_id", itemId))
		}
	}

	// rank items
	for _, r := range newRecommend {
		cache.SortDocuments(r)
	}
	return newRecommend, nil
}

type HealthStatus struct {
	DataStoreError      error
	CacheStoreError     error
	DataStoreConnected  bool
	CacheStoreConnected bool
}

func (w *Worker) checkHealth() HealthStatus {
	healthStatus := HealthStatus{}
	healthStatus.DataStoreError = w.DataClient.Ping()
	healthStatus.CacheStoreError = w.CacheClient.Ping()
	healthStatus.DataStoreConnected = healthStatus.DataStoreError == nil
	healthStatus.CacheStoreConnected = healthStatus.CacheStoreError == nil
	return healthStatus
}

func (w *Worker) checkLive(writer http.ResponseWriter, _ *http.Request) {
	healthStatus := w.checkHealth()
	writeJSON(writer, healthStatus)
}

// ItemCache is alias of map[string]data.Item.
type ItemCache struct {
	Data map[string]*data.Item
}

func NewItemCache() *ItemCache {
	return &ItemCache{Data: make(map[string]*data.Item)}
}

func (c *ItemCache) Len() int {
	return len(c.Data)
}

func (c *ItemCache) Set(itemId string, item data.Item) {
	if _, exist := c.Data[itemId]; !exist {
		c.Data[itemId] = &item
	}
}

func (c *ItemCache) Get(itemId string) (*data.Item, bool) {
	item, exist := c.Data[itemId]
	return item, exist
}

func (c *ItemCache) GetCategory(itemId string) []string {
	if item, exist := c.Data[itemId]; exist {
		return item.Categories
	} else {
		return nil
	}
}

// IsAvailable means the item exists in database and is not hidden.
func (c *ItemCache) IsAvailable(itemId string) bool {
	if item, exist := c.Data[itemId]; exist {
		return !item.IsHidden
	} else {
		return false
	}
}

// FeedbackCache is the cache for user feedbacks.
type FeedbackCache struct {
	*config.Config
	Client data.Database
	Types  []string
	Cache  cmap.ConcurrentMap
}

// NewFeedbackCache creates a new FeedbackCache.
func NewFeedbackCache(worker *Worker, feedbackTypes ...string) *FeedbackCache {
	return &FeedbackCache{
		Config: worker.Config,
		Client: worker.DataClient,
		Types:  feedbackTypes,
		Cache:  cmap.New(),
	}
}

// GetUserFeedback gets user feedback from cache or database.
func (c *FeedbackCache) GetUserFeedback(ctx context.Context, userId string) ([]string, error) {
	if tmp, ok := c.Cache.Get(userId); ok {
		return tmp.([]string), nil
	} else {
		items := make([]string, 0)
		feedbacks, err := c.Client.GetUserFeedback(ctx, userId, c.Config.Now(), c.Types...)
		if err != nil {
			return nil, err
		}
		for _, feedback := range feedbacks {
			items = append(items, feedback.ItemId)
		}
		c.Cache.Set(userId, items)
		return items, nil
	}
}
