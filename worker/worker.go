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
	"github.com/juju/errors"
	"github.com/lafikl/consistent"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/scylladb/go-set"
	"github.com/scylladb/go-set/strset"
	"github.com/thoas/go-funk"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/heap"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/parallel"
	"github.com/zhenghaoz/gorse/base/search"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"math"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"time"
)

const batchSize = 10000

// Worker manages states of a worker node.
type Worker struct {
	oneMode  bool
	testMode bool
	*config.Settings

	// worker config
	jobs       int
	workerName string
	httpHost   string
	httpPort   int
	masterHost string
	masterPort int
	cacheFile  string

	// database connection path
	cachePath string
	dataPath  string

	// master connection
	masterClient protocol.MasterClient

	latestRankingModelVersion int64
	latestClickModelVersion   int64
	rankingIndex              *search.HNSW

	// peers
	peers []string
	me    string

	// events
	ticker     *time.Ticker
	syncedChan chan bool // meta synced events
	pulledChan chan bool // model pulled events
}

// NewWorker creates a new worker node.
func NewWorker(masterHost string, masterPort int, httpHost string, httpPort, jobs int, cacheFile string) *Worker {
	return &Worker{
		Settings: config.NewSettings(),
		// config
		cacheFile:  cacheFile,
		masterHost: masterHost,
		masterPort: masterPort,
		httpHost:   httpHost,
		httpPort:   httpPort,
		jobs:       jobs,
		// events
		ticker:     time.NewTicker(time.Minute),
		syncedChan: make(chan bool, 1024),
		pulledChan: make(chan bool, 1024),
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
				NodeType:      protocol.NodeType_WorkerNode,
				NodeName:      w.workerName,
				HttpPort:      int64(w.httpPort),
				BinaryVersion: version.Version,
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
		w.ticker.Reset(w.Config.Recommend.Offline.CheckRecommendPeriod)

		// connect to data store
		if w.dataPath != w.Config.Database.DataStore {
			log.Logger().Info("connect data store",
				zap.String("database", log.RedactDBURL(w.Config.Database.DataStore)))
			if w.DataClient, err = data.Open(w.Config.Database.DataStore); err != nil {
				log.Logger().Error("failed to connect data store", zap.Error(err))
				goto sleep
			}
			w.dataPath = w.Config.Database.DataStore
		}

		// connect to cache store
		if w.cachePath != w.Config.Database.CacheStore {
			log.Logger().Info("connect cache store",
				zap.String("database", log.RedactDBURL(w.Config.Database.CacheStore)))
			if w.CacheClient, err = cache.Open(w.Config.Database.CacheStore); err != nil {
				log.Logger().Error("failed to connect cache store", zap.Error(err))
				goto sleep
			}
			w.cachePath = w.Config.Database.CacheStore
		}

		// check ranking model version
		w.latestRankingModelVersion = meta.RankingModelVersion
		if w.latestRankingModelVersion != w.RankingModelVersion {
			log.Logger().Info("new ranking model found",
				zap.String("old_version", encoding.Hex(w.RankingModelVersion)),
				zap.String("new_version", encoding.Hex(w.latestRankingModelVersion)))
			w.syncedChan <- true
		}

		// check click model version
		w.latestClickModelVersion = meta.ClickModelVersion
		if w.latestClickModelVersion != w.ClickModelVersion {
			log.Logger().Info("new click model found",
				zap.String("old_version", encoding.Hex(w.ClickModelVersion)),
				zap.String("new_version", encoding.Hex(w.latestClickModelVersion)))
			w.syncedChan <- true
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
	for range w.syncedChan {
		pulled := false

		// pull ranking model
		if w.latestRankingModelVersion != w.RankingModelVersion {
			log.Logger().Info("start pull ranking model")
			if rankingModelReceiver, err := w.masterClient.GetRankingModel(context.Background(),
				&protocol.VersionInfo{Version: w.latestRankingModelVersion},
				grpc.MaxCallRecvMsgSize(math.MaxInt)); err != nil {
				log.Logger().Error("failed to pull ranking model", zap.Error(err))
			} else {
				var rankingModel ranking.MatrixFactorization
				rankingModel, err = protocol.UnmarshalRankingModel(rankingModelReceiver)
				if err != nil {
					log.Logger().Error("failed to unmarshal ranking model", zap.Error(err))
				} else {
					w.RankingModel = rankingModel
					w.rankingIndex = nil
					w.RankingModelVersion = w.latestRankingModelVersion
					log.Logger().Info("synced ranking model",
						zap.String("version", encoding.Hex(w.RankingModelVersion)))
					MemoryInuseBytesVec.WithLabelValues("collaborative_filtering_model").Set(float64(w.RankingModel.Bytes()))
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
				clickModel, err = protocol.UnmarshalClickModel(clickModelReceiver)
				if err != nil {
					log.Logger().Error("failed to unmarshal click model", zap.Error(err))
				} else {
					w.ClickModel = clickModel
					w.ClickModelVersion = w.latestClickModelVersion
					log.Logger().Info("synced click model",
						zap.String("version", encoding.Hex(w.ClickModelVersion)))
					MemoryInuseBytesVec.WithLabelValues("ranking_model").Set(float64(w.ClickModel.Bytes()))
					pulled = true
				}
			}
		}

		if w.testMode {
			return
		}
		if pulled {
			w.pulledChan <- true
		}
	}
}

// ServeMetrics serves Prometheus metrics.
func (w *Worker) ServeMetrics() {
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", w.httpHost, w.httpPort), nil)
	if err != nil {
		log.Logger().Fatal("failed to start http server", zap.Error(err))
	}
}

// Serve as a worker node.
func (w *Worker) Serve() {
	rand.Seed(time.Now().UTC().UnixNano())
	// open local store
	state, err := LoadLocalCache(w.cacheFile)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Logger().Info("no cache file found, create a new one", zap.String("path", state.path))
		} else {
			log.Logger().Error("failed to load persist state", zap.Error(err),
				zap.String("path", state.path))
		}
	}
	if state.WorkerName == "" {
		state.WorkerName = base.GetRandomName(0)
		err = state.WriteLocalCache()
		if err != nil {
			log.Logger().Fatal("failed to write meta", zap.Error(err))
		}
	}
	w.workerName = state.WorkerName
	log.Logger().Info("start worker",
		zap.Int("n_jobs", w.jobs),
		zap.String("worker_name", w.workerName))

	// connect to master
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", w.masterHost, w.masterPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	w.masterClient = protocol.NewMasterClient(conn)

	if w.oneMode {
		w.peers = []string{w.workerName}
		w.me = w.workerName
	} else {
		go w.Sync()
		go w.Pull()
		go w.ServeMetrics()
	}

	loop := func() {
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

	for {
		select {
		case <-w.ticker.C:
			loop()
		case <-w.pulledChan:
			loop()
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
	startRecommendTime := time.Now()
	log.Logger().Info("ranking recommendation",
		zap.Int("n_working_users", len(users)),
		zap.Int("n_jobs", w.jobs),
		zap.Int("cache_size", w.Config.Recommend.CacheSize))

	// progress tracker
	completed := make(chan struct{}, 1000)
	recommendTask := task.NewTask(fmt.Sprintf("Generate offline recommendation [%s]", w.workerName), len(users))
	if w.masterClient != nil {
		if _, err := w.masterClient.PushTaskInfo(context.Background(), recommendTask.ToPB()); err != nil {
			log.Logger().Error("failed to report start task", zap.Error(err))
		}
	}

	// pull items from database
	itemCache, itemCategories, err := w.pullItems()
	if err != nil {
		log.Logger().Error("failed to pull items", zap.Error(err))
		return
	}
	MemoryInuseBytesVec.WithLabelValues("item_cache").Set(float64(itemCache.Bytes()))
	defer MemoryInuseBytesVec.WithLabelValues("item_cache").Set(0)

	// build ranking index
	if w.RankingModel != nil && w.rankingIndex == nil && w.Config.Recommend.Collaborative.EnableIndex {
		startTime := time.Now()
		log.Logger().Info("start building ranking index")
		itemIndex := w.RankingModel.GetItemIndex()
		vectors := make([]search.Vector, itemIndex.Len())
		for i := int32(0); i < itemIndex.Len(); i++ {
			itemId := itemIndex.ToName(i)
			if itemCache.IsAvailable(itemId) {
				vectors[i] = search.NewDenseVector(w.RankingModel.GetItemFactor(i), itemCache.GetCategory(itemId), false)
			} else {
				vectors[i] = search.NewDenseVector(w.RankingModel.GetItemFactor(i), nil, true)
			}
		}
		builder := search.NewHNSWBuilder(vectors, w.Config.Recommend.CacheSize, 1000, w.jobs)
		var recall float32
		w.rankingIndex, recall = builder.Build(w.Config.Recommend.Collaborative.IndexRecall, w.Config.Recommend.Collaborative.IndexFitEpoch, false)
		CollaborativeFilteringIndexRecall.Set(float64(recall))
		if err = w.CacheClient.Set(cache.String(cache.Key(cache.GlobalMeta, cache.MatchingIndexRecall), encoding.FormatFloat32(recall))); err != nil {
			log.Logger().Error("failed to write meta", zap.Error(err))
		}
		log.Logger().Info("complete building ranking index",
			zap.Duration("build_time", time.Since(startTime)))
	} else if w.RankingModel != nil && w.rankingIndex == nil {
		CollaborativeFilteringIndexRecall.Set(1)
	}

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
						recommendTask.Update(completedCount)
						if _, err := w.masterClient.PushTaskInfo(context.Background(), recommendTask.ToPB()); err != nil {
							log.Logger().Error("failed to report update task", zap.Error(err))
						}
					}
					log.Logger().Info("ranking recommendation",
						zap.Int("n_complete_users", completedCount),
						zap.Int("n_working_users", len(users)),
						zap.Int("throughput", throughput))
				}
			}
		}
	}()
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

	userFeedbackCache := NewFeedbackCache(w.DataClient, w.Config.Recommend.DataSource.PositiveFeedbackTypes...)
	defer MemoryInuseBytesVec.WithLabelValues("user_feedback_cache").Set(0)
	err = parallel.Parallel(len(users), w.jobs, func(workerId, jobId int) error {
		defer func() {
			completed <- struct{}{}
		}()
		user := users[jobId]
		userId := user.UserId
		// skip inactive users before max recommend period
		if !w.checkRecommendCacheTimeout(userId, itemCategories) {
			return nil
		}
		updateUserCount.Add(1)

		// load historical items
		historyItems, feedbacks, err := loadUserHistoricalItems(w.DataClient, userId)
		excludeSet := set.NewStringSet(historyItems...)
		if err != nil {
			log.Logger().Error("failed to pull user feedback",
				zap.String("user_id", userId), zap.Error(err))
			return errors.Trace(err)
		}

		// load positive items
		var positiveItems []string
		if w.Config.Recommend.Offline.EnableItemBasedRecommend {
			positiveItems, err = userFeedbackCache.GetUserFeedback(userId)
			if err != nil {
				log.Logger().Error("failed to pull user feedback",
					zap.String("user_id", userId), zap.Error(err))
				return errors.Trace(err)
			}
			MemoryInuseBytesVec.WithLabelValues("user_feedback_cache").Set(float64(userFeedbackCache.Bytes()))
		}

		// create candidates container
		candidates := make(map[string][][]string)
		candidates[""] = make([][]string, 0)
		for _, category := range itemCategories {
			candidates[category] = make([][]string, 0)
		}

		// Recommender #1: collaborative filtering.
		collaborativeUsed := false
		if w.Config.Recommend.Offline.EnableColRecommend && w.RankingModel != nil {
			if userIndex := w.RankingModel.GetUserIndex().ToNumber(userId); w.RankingModel.IsUserPredictable(userIndex) {
				var recommend map[string][]string
				var usedTime time.Duration
				if w.Config.Recommend.Collaborative.EnableIndex {
					recommend, usedTime, err = w.collaborativeRecommendHNSW(w.rankingIndex, userId, itemCategories, excludeSet, itemCache)
				} else {
					recommend, usedTime, err = w.collaborativeRecommendBruteForce(userId, itemCategories, excludeSet, itemCache)
				}
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
			} else if !w.RankingModel.IsUserPredictable(userIndex) {
				log.Logger().Debug("user is unpredictable", zap.String("user_id", userId))
			}
		} else if w.RankingModel == nil {
			log.Logger().Warn("no collaborative filtering model")
		}

		// Recommender #2: item-based.
		itemNeighborDigests := strset.New()
		if w.Config.Recommend.Offline.EnableItemBasedRecommend {
			localStartTime := time.Now()
			for _, category := range append([]string{""}, itemCategories...) {
				// collect candidates
				scores := make(map[string]float64)
				for _, itemId := range positiveItems {
					// load similar items
					similarItems, err := w.CacheClient.GetSorted(cache.Key(cache.ItemNeighbors, itemId, category), 0, w.Config.Recommend.CacheSize)
					if err != nil {
						log.Logger().Error("failed to load similar items", zap.Error(err))
						return errors.Trace(err)
					}
					// add unseen items
					for _, item := range similarItems {
						if !excludeSet.Has(item.Id) && itemCache.IsAvailable(item.Id) {
							scores[item.Id] += item.Score
						}
					}
					// load item neighbors digest
					digest, err := w.CacheClient.Get(cache.Key(cache.ItemNeighborsDigest, itemId)).String()
					if err != nil {
						if !errors.IsNotFound(err) {
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
				candidates[category] = append(candidates[category], ids)
			}
			itemBasedRecommendSeconds.Add(time.Since(localStartTime).Seconds())
		}

		// Recommender #3: insert user-based items
		userNeighborDigests := strset.New()
		if w.Config.Recommend.Offline.EnableUserBasedRecommend {
			localStartTime := time.Now()
			scores := make(map[string]float64)
			// load similar users
			similarUsers, err := w.CacheClient.GetSorted(cache.Key(cache.UserNeighbors, userId), 0, w.Config.Recommend.CacheSize)
			if err != nil {
				log.Logger().Error("failed to load similar users", zap.Error(err))
				return errors.Trace(err)
			}
			for _, user := range similarUsers {
				// load historical feedback
				similarUserPositiveItems, err := userFeedbackCache.GetUserFeedback(user.Id)
				if err != nil {
					log.Logger().Error("failed to pull user feedback",
						zap.String("user_id", userId), zap.Error(err))
					return errors.Trace(err)
				}
				MemoryInuseBytesVec.WithLabelValues("user_feedback_cache").Set(float64(userFeedbackCache.Bytes()))
				// add unseen items
				for _, itemId := range similarUserPositiveItems {
					if !excludeSet.Has(itemId) && itemCache.IsAvailable(itemId) {
						scores[itemId] += user.Score
					}
				}
				// load user neighbors digest
				digest, err := w.CacheClient.Get(cache.Key(cache.UserNeighborsDigest, user.Id)).String()
				if err != nil {
					if !errors.IsNotFound(err) {
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
				latestItems, err := w.CacheClient.GetSorted(cache.Key(cache.LatestItems, category), 0, w.Config.Recommend.CacheSize)
				if err != nil {
					log.Logger().Error("failed to load latest items", zap.Error(err))
					return errors.Trace(err)
				}
				var recommend []string
				for _, latestItem := range latestItems {
					if !excludeSet.Has(latestItem.Id) && itemCache.IsAvailable(latestItem.Id) {
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
				popularItems, err := w.CacheClient.GetSorted(cache.Key(cache.PopularItems, category), 0, w.Config.Recommend.CacheSize)
				if err != nil {
					log.Logger().Error("failed to load popular items", zap.Error(err))
					return errors.Trace(err)
				}
				var recommend []string
				for _, popularItem := range popularItems {
					if !excludeSet.Has(popularItem.Id) && itemCache.IsAvailable(popularItem.Id) {
						recommend = append(recommend, popularItem.Id)
					}
				}
				candidates[category] = append(candidates[category], recommend)
			}
			popularRecommendSeconds.Add(time.Since(localStartTime).Seconds())
		}

		// rank items from different recommenders
		// 1. If click-through rate prediction model is available, use it to rank items.
		// 2. If collaborative filtering model is available, use it to rank items.
		// 3. Otherwise, merge all recommenders' results randomly.
		ctrUsed := false
		results := make(map[string][]cache.Scored)
		for category, catCandidates := range candidates {
			if w.Config.Recommend.Offline.EnableClickThroughPrediction && w.ClickModel != nil {
				results[category], err = w.rankByClickTroughRate(&user, catCandidates, itemCache)
				if err != nil {
					log.Logger().Error("failed to rank items", zap.Error(err))
					return errors.Trace(err)
				}
				ctrUsed = true
			} else if w.RankingModel != nil &&
				w.RankingModel.IsUserPredictable(w.RankingModel.GetUserIndex().ToNumber(userId)) {
				results[category], err = w.rankByCollaborativeFiltering(userId, catCandidates)
				if err != nil {
					log.Logger().Error("failed to rank items", zap.Error(err))
					return errors.Trace(err)
				}
			} else {
				results[category] = mergeAndShuffle(catCandidates)
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
		for category, result := range results {
			results[category], err = w.exploreRecommend(result, excludeSet, category)
			if err != nil {
				log.Logger().Error("failed to explore latest and popular items", zap.Error(err))
				return errors.Trace(err)
			}

			if err = w.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, userId, category), results[category]); err != nil {
				log.Logger().Error("failed to cache recommendation", zap.Error(err))
				return errors.Trace(err)
			}
		}
		if err = w.CacheClient.Set(
			cache.Time(cache.Key(cache.LastUpdateUserRecommendTime, userId), time.Now()),
			cache.String(cache.Key(cache.OfflineRecommendDigest, userId), w.Config.OfflineRecommendDigest(
				config.WithCollaborative(collaborativeUsed),
				config.WithRanking(ctrUsed),
				config.WithItemNeighborDigest(strings.Join(itemNeighborDigests.List(), "-")),
				config.WithUserNeighborDigest(strings.Join(userNeighborDigests.List(), "-")),
			))); err != nil {
			log.Logger().Error("failed to cache recommendation time", zap.Error(err))
		}

		// refresh cache
		err = w.refreshCache(userId)
		if err != nil {
			log.Logger().Error("failed to refresh cache", zap.Error(err))
			return errors.Trace(err)
		}
		return nil
	})
	close(completed)
	if err != nil {
		log.Logger().Error("failed to continue offline recommendation", zap.Error(err))
		return
	}
	if w.masterClient != nil {
		recommendTask.Finish()
		if _, err := w.masterClient.PushTaskInfo(context.Background(), recommendTask.ToPB()); err != nil {
			log.Logger().Error("failed to report finish task", zap.Error(err))
		}
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

func (w *Worker) collaborativeRecommendBruteForce(userId string, itemCategories []string, excludeSet *strset.Set, itemCache *ItemCache) (map[string][]string, time.Duration, error) {
	userIndex := w.RankingModel.GetUserIndex().ToNumber(userId)
	itemIds := w.RankingModel.GetItemIndex().GetNames()
	localStartTime := time.Now()
	recItemsFilters := make(map[string]*heap.TopKFilter[string, float64])
	recItemsFilters[""] = heap.NewTopKFilter[string, float64](w.Config.Recommend.CacheSize)
	for _, category := range itemCategories {
		recItemsFilters[category] = heap.NewTopKFilter[string, float64](w.Config.Recommend.CacheSize)
	}
	for itemIndex, itemId := range itemIds {
		if !excludeSet.Has(itemId) && itemCache.IsAvailable(itemId) && w.RankingModel.IsItemPredictable(int32(itemIndex)) {
			prediction := w.RankingModel.InternalPredict(userIndex, int32(itemIndex))
			recItemsFilters[""].Push(itemId, float64(prediction))
			for _, category := range itemCache.GetCategory(itemId) {
				recItemsFilters[category].Push(itemId, float64(prediction))
			}
		}
	}
	// save result
	recommend := make(map[string][]string)
	for category, recItemsFilter := range recItemsFilters {
		recommendItems, recommendScores := recItemsFilter.PopAll()
		recommend[category] = recommendItems
		if err := w.CacheClient.SetSorted(cache.Key(cache.CollaborativeRecommend, userId, category), cache.CreateScoredItems(recommendItems, recommendScores)); err != nil {
			log.Logger().Error("failed to cache collaborative filtering recommendation result", zap.String("user_id", userId), zap.Error(err))
			return nil, 0, errors.Trace(err)
		}
	}
	return recommend, time.Since(localStartTime), nil
}

func (w *Worker) collaborativeRecommendHNSW(rankingIndex *search.HNSW, userId string, itemCategories []string, excludeSet *strset.Set, itemCache *ItemCache) (map[string][]string, time.Duration, error) {
	userIndex := w.RankingModel.GetUserIndex().ToNumber(userId)
	localStartTime := time.Now()
	values, scores := rankingIndex.MultiSearch(search.NewDenseVector(w.RankingModel.GetUserFactor(userIndex), nil, false),
		itemCategories, w.Config.Recommend.CacheSize+excludeSet.Size(), false)
	// save result
	recommend := make(map[string][]string)
	for category, catValues := range values {
		recommendItems := make([]string, 0, len(catValues))
		recommendScores := make([]float64, 0, len(catValues))
		for i := range catValues {
			itemId := w.RankingModel.GetItemIndex().ToName(catValues[i])
			if !excludeSet.Has(itemId) && itemCache.IsAvailable(itemId) {
				recommendItems = append(recommendItems, itemId)
				recommendScores = append(recommendScores, float64(scores[category][i]))
			}
		}
		recommend[category] = recommendItems
		if err := w.CacheClient.SetSorted(cache.Key(cache.CollaborativeRecommend, userId, category),
			cache.CreateScoredItems(recommendItems, recommendScores)); err != nil {
			log.Logger().Error("failed to cache collaborative filtering recommendation result", zap.String("user_id", userId), zap.Error(err))
			return nil, 0, errors.Trace(err)
		}
	}
	return recommend, time.Since(localStartTime), nil
}

func (w *Worker) rankByCollaborativeFiltering(userId string, candidates [][]string) ([]cache.Scored, error) {
	// concat candidates
	memo := strset.New()
	var itemIds []string
	for _, v := range candidates {
		for _, itemId := range v {
			if !memo.Has(itemId) {
				memo.Add(itemId)
				itemIds = append(itemIds, itemId)
			}
		}
	}
	// rank by collaborative filtering
	topItems := make([]cache.Scored, 0, len(candidates))
	for _, itemId := range itemIds {
		topItems = append(topItems, cache.Scored{
			Id:    itemId,
			Score: float64(w.RankingModel.Predict(userId, itemId)),
		})
	}
	cache.SortScores(topItems)
	return topItems, nil
}

// rankByClickTroughRate ranks items by predicted click-through-rate.
func (w *Worker) rankByClickTroughRate(user *data.User, candidates [][]string, itemCache *ItemCache) ([]cache.Scored, error) {
	// concat candidates
	memo := strset.New()
	var itemIds []string
	for _, v := range candidates {
		for _, itemId := range v {
			if !memo.Has(itemId) {
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
	topItems := make([]cache.Scored, 0, len(items))
	for _, item := range items {
		topItems = append(topItems, cache.Scored{
			Id:    item.ItemId,
			Score: float64(w.ClickModel.Predict(user.UserId, item.ItemId, user.Labels, item.Labels)),
		})
	}
	cache.SortScores(topItems)
	return topItems, nil
}

func mergeAndShuffle(candidates [][]string) []cache.Scored {
	memo := strset.New()
	pos := make([]int, len(candidates))
	var recommend []cache.Scored
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
		j := src[rand.Intn(len(src))]
		candidateId := candidates[j][pos[j]]
		pos[j]++
		if !memo.Has(candidateId) {
			memo.Add(candidateId)
			recommend = append(recommend, cache.Scored{Score: math.Exp(float64(-len(recommend))), Id: candidateId})
		}
	}
	return recommend
}

func (w *Worker) exploreRecommend(exploitRecommend []cache.Scored, excludeSet *strset.Set, category string) ([]cache.Scored, error) {
	var localExcludeSet *strset.Set
	if w.Config.Recommend.Replacement.EnableReplacement {
		localExcludeSet = strset.New()
	} else {
		localExcludeSet = excludeSet.Copy()
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
	popularItems, err := w.CacheClient.GetSorted(cache.Key(cache.PopularItems, category), 0, w.Config.Recommend.CacheSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// load the latest items
	latestItems, err := w.CacheClient.GetSorted(cache.Key(cache.LatestItems, category), 0, w.Config.Recommend.CacheSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// explore recommendation
	var exploreRecommend []cache.Scored
	score := 1.0
	if len(exploitRecommend) > 0 {
		score += exploitRecommend[0].Score
	}
	for range exploitRecommend {
		dice := rand.Float64()
		var recommendItem cache.Scored
		if dice < explorePopularThreshold && len(popularItems) > 0 {
			score -= 1e-5
			recommendItem = popularItems[0]
			recommendItem.Score = score
			popularItems = popularItems[1:]
		} else if dice < exploreLatestThreshold && len(latestItems) > 0 {
			score -= 1e-5
			recommendItem = latestItems[0]
			recommendItem.Score = score
			latestItems = latestItems[1:]
		} else if len(exploitRecommend) > 0 {
			recommendItem = exploitRecommend[0]
			exploitRecommend = exploitRecommend[1:]
			score = recommendItem.Score
		} else {
			break
		}
		if !localExcludeSet.Has(recommendItem.Id) {
			localExcludeSet.Add(recommendItem.Id)
			exploreRecommend = append(exploreRecommend, recommendItem)
		}
	}
	return exploreRecommend, nil
}

// checkRecommendCacheTimeout checks if recommend cache stale.
// 1. if cache is empty, stale.
// 2. if active time > recommend time, stale.
// 3. if recommend time + timeout < now, stale.
func (w *Worker) checkRecommendCacheTimeout(userId string, categories []string) bool {
	var (
		activeTime    time.Time
		recommendTime time.Time
		cacheDigest   string
		err           error
	)
	// check cache
	for _, category := range append([]string{""}, categories...) {
		items, err := w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, userId, category), 0, -1)
		if err != nil {
			log.Logger().Error("failed to load offline recommendation", zap.String("user_id", userId), zap.Error(err))
			return true
		} else if len(items) == 0 {
			return true
		}
	}
	// read digest
	cacheDigest, err = w.CacheClient.Get(cache.Key(cache.OfflineRecommendDigest, userId)).String()
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Logger().Error("failed to load offline recommendation digest", zap.String("user_id", userId), zap.Error(err))
		}
		return true
	}
	if cacheDigest != w.Config.OfflineRecommendDigest() {
		return true
	}
	// read active time
	activeTime, err = w.CacheClient.Get(cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Logger().Error("failed to read last modify user time", zap.Error(err))
		}
		return true
	}
	// read recommend time
	recommendTime, err = w.CacheClient.Get(cache.Key(cache.LastUpdateUserRecommendTime, userId)).Time()
	if err != nil {
		if !errors.IsNotFound(err) {
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

func loadUserHistoricalItems(database data.Database, userId string) ([]string, []data.Feedback, error) {
	items := make([]string, 0)
	feedbacks, err := database.GetUserFeedback(userId, false)
	if err != nil {
		return nil, nil, err
	}
	for _, feedback := range feedbacks {
		items = append(items, feedback.ItemId)
	}
	return items, feedbacks, nil
}

func (w *Worker) refreshCache(userId string) error {
	var timeLimit *time.Time
	// read recommend time
	recommendTime, err := w.CacheClient.Get(cache.Key(cache.LastUpdateUserRecommendTime, userId)).Time()
	if err == nil {
		timeLimit = &recommendTime
	} else if !errors.IsNotFound(err) {
		return errors.Trace(err)
	}
	// reload cache
	if w.Config.Recommend.Replacement.EnableReplacement {
		err = w.CacheClient.SetSorted(cache.IgnoreItems, nil)
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		feedback, err := w.DataClient.GetUserFeedback(userId, true)
		if err != nil {
			return errors.Trace(err)
		}
		var items []cache.Scored
		for _, v := range feedback {
			if v.Timestamp.Unix() > timeLimit.Unix() {
				items = append(items, cache.Scored{Id: v.ItemId, Score: float64(v.Timestamp.Unix())})
			}
		}
		err = w.CacheClient.AddSorted(cache.Sorted(cache.Key(cache.IgnoreItems, userId), items))
		if err != nil {
			return errors.Trace(err)
		}
		err = w.CacheClient.RemSortedByScore(cache.Key(cache.IgnoreItems, userId), math.Inf(-1), float64(timeLimit.Unix())-1)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (w *Worker) pullItems() (*ItemCache, []string, error) {
	// pull items from database
	itemCache := NewItemCache()
	itemCategories := strset.New()
	itemChan, errChan := w.DataClient.GetItemStream(batchSize, nil)
	for batchItems := range itemChan {
		for _, item := range batchItems {
			itemCache.Set(item.ItemId, item)
			itemCategories.Add(item.Categories...)
		}
	}
	if err := <-errChan; err != nil {
		return nil, nil, errors.Trace(err)
	}
	return itemCache, itemCategories.List(), nil
}

func (w *Worker) pullUsers(peers []string, me string) ([]data.User, error) {
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
	userChan, errChan := w.DataClient.GetUserStream(batchSize)
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
func (w *Worker) replacement(recommend map[string][]cache.Scored, user *data.User, feedbacks []data.Feedback, itemCache *ItemCache) (map[string][]cache.Scored, error) {
	upperBounds := make(map[string]float64)
	lowerBounds := make(map[string]float64)
	newRecommend := make(map[string][]cache.Scored)
	for category, scores := range recommend {
		// find minimal score
		if len(scores) > 0 {
			s := cache.GetScores(scores)
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
	positiveItems := strset.New()
	distinctItems := strset.New()
	for _, feedback := range feedbacks {
		if funk.ContainsString(w.Config.Recommend.DataSource.PositiveFeedbackTypes, feedback.FeedbackType) {
			positiveItems.Add(feedback.ItemId)
			distinctItems.Add(feedback.ItemId)
		} else if funk.ContainsString(w.Config.Recommend.DataSource.ReadFeedbackTypes, feedback.FeedbackType) {
			distinctItems.Add(feedback.ItemId)
		}
	}

	for _, itemId := range distinctItems.List() {
		if item, exist := itemCache.Get(itemId); exist {
			// scoring item
			// 1. If click-through rate prediction model is available, use it.
			// 2. If collaborative filtering model is available, use it.
			// 3. Otherwise, give a random score.
			var score float64
			if w.Config.Recommend.Offline.EnableClickThroughPrediction && w.ClickModel != nil {
				score = float64(w.ClickModel.Predict(user.UserId, itemId, user.Labels, item.Labels))
			} else if w.RankingModel != nil && w.RankingModel.IsUserPredictable(w.RankingModel.GetUserIndex().ToNumber(user.UserId)) {
				score = float64(w.RankingModel.Predict(user.UserId, itemId))
			} else {
				upper := upperBounds[""]
				lower := lowerBounds[""]
				if !math.IsInf(upper, 1) && !math.IsInf(lower, -1) {
					score = lower + rand.Float64()*(upper-lower)
				} else {
					score = rand.Float64()
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
					} else if positiveItems.Has(itemId) {
						score *= w.Config.Recommend.Replacement.PositiveReplacementDecay
					} else {
						score *= w.Config.Recommend.Replacement.ReadReplacementDecay
					}
					score += lowerBound
				}
				newRecommend[category] = append(newRecommend[category], cache.Scored{Id: itemId, Score: score})
			}
		} else {
			log.Logger().Warn("item doesn't exists in database", zap.String("item_id", itemId))
		}
	}

	// rank items
	for _, r := range newRecommend {
		cache.SortScores(r)
	}
	return newRecommend, nil
}

// ItemCache is alias of map[string]data.Item.
type ItemCache struct {
	Data      map[string]*data.Item
	ByteCount uintptr
}

func NewItemCache() *ItemCache {
	return &ItemCache{Data: make(map[string]*data.Item)}
}

func (c *ItemCache) Set(itemId string, item data.Item) {
	if _, exist := c.Data[itemId]; !exist {
		c.Data[itemId] = &item
		c.ByteCount += reflect.TypeOf(rune(0)).Size() * uintptr(len(itemId))
		c.ByteCount += reflect.TypeOf(item.ItemId).Size() * uintptr(len(itemId))
		c.ByteCount += reflect.TypeOf(item.Comment).Size() * uintptr(len(itemId))
		c.ByteCount += encoding.StringsBytes(item.Categories)
		c.ByteCount += encoding.StringsBytes(item.Labels)
		c.ByteCount += reflect.TypeOf(item).Size()
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

func (c *ItemCache) Bytes() int {
	return int(c.ByteCount)
}

// FeedbackCache is the cache for user feedbacks.
type FeedbackCache struct {
	Types     []string
	Cache     cmap.ConcurrentMap
	Client    data.Database
	ByteCount uintptr
}

// NewFeedbackCache creates a new FeedbackCache.
func NewFeedbackCache(client data.Database, feedbackTypes ...string) *FeedbackCache {
	return &FeedbackCache{
		Types:  feedbackTypes,
		Client: client,
		Cache:  cmap.New(),
	}
}

// GetUserFeedback gets user feedback from cache or database.
func (c *FeedbackCache) GetUserFeedback(userId string) ([]string, error) {
	if tmp, ok := c.Cache.Get(userId); ok {
		return tmp.([]string), nil
	} else {
		items := make([]string, 0)
		feedbacks, err := c.Client.GetUserFeedback(userId, false, c.Types...)
		if err != nil {
			return nil, err
		}
		for _, feedback := range feedbacks {
			items = append(items, feedback.ItemId)
			c.ByteCount += reflect.TypeOf(rune(0)).Size() * uintptr(len(feedback.FeedbackType))
			c.ByteCount += reflect.TypeOf(rune(0)).Size() * uintptr(len(feedback.UserId))
			c.ByteCount += reflect.TypeOf(rune(0)).Size() * uintptr(len(feedback.ItemId))
			c.ByteCount += reflect.TypeOf(rune(0)).Size() * uintptr(len(feedback.Comment))
		}
		c.Cache.Set(userId, items)
		c.ByteCount += reflect.TypeOf(feedbacks).Elem().Size() * uintptr(len(feedbacks))
		c.ByteCount += reflect.TypeOf(rune(0)).Size() * uintptr(len(userId))
		return items, nil
	}
}

func (c *FeedbackCache) Bytes() int {
	return int(c.ByteCount)
}
