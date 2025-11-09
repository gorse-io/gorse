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
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/common/sizeof"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/blob"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/juju/errors"
	"github.com/lafikl/consistent"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/lo"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

const batchSize = 10000

// Worker manages states of a worker node.
type Worker struct {
	tracer      *monitor.Monitor
	testMode    bool
	Config      *config.Config
	CacheClient cache.Database
	DataClient  data.Database

	collaborativeFilteringModelId int64
	matrixFactorizationItems      *logics.MatrixFactorizationItems
	matrixFactorizationUsers      *logics.MatrixFactorizationUsers
	clickThroughRateModelId       int64
	clickThroughRateModel         ctr.FactorizationMachines

	// spawned rankers
	rankers []ctr.FactorizationMachines

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

	blobConfig string
	blobStore  blob.Store

	// master connection
	conn         *grpc.ClientConn
	masterClient protocol.MasterClient

	latestCollaborativeFilteringModelId int64
	latestClickThroughRateModelId       int64
	randGenerator                       *rand.Rand

	// peers
	peers []string
	me    string

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
	tlsConfig *util.TLSConfig,
) *Worker {
	return &Worker{
		rankers:       make([]ctr.FactorizationMachines, jobs),
		Config:        config.GetDefaultConfig(),
		CacheClient:   new(cache.NoDatabase),
		DataClient:    new(data.NoDatabase),
		randGenerator: util.NewRand(time.Now().UTC().UnixNano()),
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

// Sync this worker to the master.
func (w *Worker) Sync() {
	defer util.CheckPanic()
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
		err = json.Unmarshal([]byte(meta.Config), &w.Config)
		if err != nil {
			log.Logger().Error("failed to parse master config", zap.Error(err))
			goto sleep
		}

		// reset ticker
		if w.tickDuration != w.Config.Recommend.Ranker.CheckRecommendPeriod {
			w.tickDuration = w.Config.Recommend.Ranker.CheckRecommendPeriod
			w.ticker.Reset(w.Config.Recommend.Ranker.CheckRecommendPeriod)
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

		// connect to blob store
		if w.blobConfig != w.Config.S3.ToJSON() {
			if w.Config.S3.Endpoint == "" {
				log.Logger().Info("connect blob store via master")
				w.blobStore = blob.NewMasterStoreClient(w.conn)
			} else {
				log.Logger().Info("connect s3 endpoint", zap.String("endpoint", w.Config.S3.Endpoint))
				if w.blobStore, err = blob.NewS3(w.Config.S3); err != nil {
					log.Logger().Error("failed to connect s3 endpoint", zap.Error(err))
					goto sleep
				}
			}
			w.blobConfig = w.Config.S3.ToJSON()
		}

		// synchronize collaborative filtering model
		w.latestCollaborativeFilteringModelId = meta.CollaborativeFilteringModelId
		if w.latestCollaborativeFilteringModelId > w.collaborativeFilteringModelId {
			log.Logger().Info("new ranking model found",
				zap.Int64("old_version", w.collaborativeFilteringModelId),
				zap.Int64("new_version", w.latestCollaborativeFilteringModelId))
			w.syncedChan.Signal()
		}

		// synchronize click-through rate model
		w.latestClickThroughRateModelId = meta.ClickThroughRateModelId
		if w.latestClickThroughRateModelId > w.clickThroughRateModelId {
			log.Logger().Info("new click model found",
				zap.Int64("old_version", w.clickThroughRateModelId),
				zap.Int64("new_version", w.latestClickThroughRateModelId))
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
	defer util.CheckPanic()
	for range w.syncedChan.C {
		pulled := false

		// pull ranking model
		if w.latestCollaborativeFilteringModelId > w.collaborativeFilteringModelId {
			log.Logger().Info("start pull collaborative filtering model")
			r, err := w.blobStore.Open(strconv.FormatInt(w.latestCollaborativeFilteringModelId, 10))
			if err != nil {
				log.Logger().Error("failed to open collaborative filtering model", zap.Error(err))
			} else {
				items := logics.NewMatrixFactorizationItems(time.Time{})
				users := logics.NewMatrixFactorizationUsers()
				if err = items.Unmarshal(r); err != nil {
					log.Logger().Error("failed to unmarshal matrix factorization items", zap.Error(err))
				} else if err = users.Unmarshal(r); err != nil {
					log.Logger().Error("failed to unmarshal matrix factorization users", zap.Error(err))
				} else {
					w.matrixFactorizationItems = items
					w.matrixFactorizationUsers = users
					w.collaborativeFilteringModelId = w.latestCollaborativeFilteringModelId
					log.Logger().Info("synced collaborative filtering model",
						zap.Int64("id", w.collaborativeFilteringModelId))
					pulled = true
				}
			}
		}

		// pull click model
		if w.latestClickThroughRateModelId > w.clickThroughRateModelId {
			log.Logger().Info("start pull click model")
			r, err := w.blobStore.Open(strconv.FormatInt(w.latestClickThroughRateModelId, 10))
			if err != nil {
				log.Logger().Error("failed to open click-through rate model", zap.Error(err))
			} else {
				model, err := ctr.UnmarshalModel(r)
				if err != nil {
					log.Logger().Error("failed to unmarshal click-through rate model", zap.Error(err))
				} else {
					w.clickThroughRateModel = model
					w.clickThroughRateModelId = w.latestClickThroughRateModelId
					log.Logger().Info("synced click-through rate model",
						zap.Int64("version", w.clickThroughRateModelId))
					// spawn rankers
					for i := 0; i < w.jobs; i++ {
						if i == 0 {
							w.rankers[i] = w.clickThroughRateModel
						} else {
							w.rankers[i] = ctr.Spawn(w.clickThroughRateModel)
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
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", w.httpHost, w.httpPort), nil)
	if err != nil {
		log.Logger().Fatal("failed to start http server", zap.Error(err))
	}
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
	var err error
	if w.workerName, err = w.WorkerName(); err != nil {
		log.Logger().Fatal("failed to get worker name", zap.Error(err))
	}

	// create progress tracer
	w.tracer = monitor.NewTracer(w.workerName)

	// connect to master
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(512*1024*1024)))
	if w.tlsConfig != nil {
		c, err := util.NewClientCreds(w.tlsConfig)
		if err != nil {
			log.Logger().Fatal("failed to create credentials", zap.Error(err))
		}
		opts = append(opts, grpc.WithTransportCredentials(c))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	w.conn, err = grpc.Dial(net.JoinHostPort(w.masterHost, strconv.Itoa(w.masterPort)), opts...)
	if err != nil {
		log.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	w.masterClient = protocol.NewMasterClient(w.conn)

	go w.Sync()
	go w.Pull()
	go w.ServeHTTP()

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
		case tick := <-w.ticker.C:
			if time.Since(tick) >= w.Config.Recommend.Ranker.CheckRecommendPeriod {
				loop()
			}
		case <-w.pulledChan.C:
			loop()
		}
	}
}

func (w *Worker) WorkerName() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	hash := md5.New()
	hash.Write([]byte(hostname))
	hash.Write([]byte(w.httpHost))
	hash.Write([]byte(strconv.Itoa(w.httpPort)))
	b := hash.Sum(nil)
	return hex.EncodeToString(b), nil
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
	itemCache, _, err := w.pullItems(ctx)
	if err != nil {
		log.Logger().Error("failed to pull items", zap.Error(err))
		return
	}
	MemoryInuseBytesVec.WithLabelValues("item_cache").Set(float64(sizeof.DeepSize(itemCache)))
	defer MemoryInuseBytesVec.WithLabelValues("item_cache").Set(0)

	// progress tracker
	completed := make(chan struct{}, 1000)
	_, span := w.tracer.Start(context.Background(), "Generate Offline Recommend", len(users))
	defer span.End()

	go func() {
		defer util.CheckPanic()
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

	defer MemoryInuseBytesVec.WithLabelValues("user_feedback_cache").Set(0)
	err = parallel.Parallel(len(users), w.jobs, func(workerId, jobId int) error {
		defer func() {
			completed <- struct{}{}
		}()
		user := users[jobId]
		userId := user.UserId
		// skip inactive users before max recommend period
		if !w.checkUserActiveTime(ctx, userId) || !w.checkRecommendCacheOutOfDate(ctx, userId) {
			return nil
		}
		updateUserCount.Add(1)

		recommendTime := time.Now()
		recommender, err := logics.NewRecommender(w.Config.Recommend, w.CacheClient, w.DataClient, false, userId, nil)
		if err != nil {
			return errors.Trace(err)
		}

		// Update collaborative filtering recommendation.
		if w.matrixFactorizationUsers != nil && w.matrixFactorizationItems != nil {
			if userEmbedding, ok := w.matrixFactorizationUsers.Get(userId); ok {
				err = w.updateCollaborativeRecommend(w.matrixFactorizationItems, userId, userEmbedding, recommender.ExcludeSet(), itemCache)
				if err != nil {
					log.Logger().Error("failed to recommend by collaborative filtering",
						zap.String("user_id", userId), zap.Error(err))
					return errors.Trace(err)
				}
			}
		}

		// Generate recommendation from recommenders.
		var (
			scores           []cache.Score
			digest           string
			recommenderNames []string
		)
		if len(w.Config.Recommend.Ranker.Recommenders) > 0 {
			recommenderNames = w.Config.Recommend.Ranker.Recommenders
		} else {
			recommenderNames = w.Config.Recommend.ListRecommenders()
		}
		scores, digest, err = recommender.RecommendSequential(context.Background(), scores, 0, recommenderNames...)
		if err != nil {
			return errors.Trace(err)
		}

		candidates := make([]cache.Score, 0, len(scores))
		for _, score := range scores {
			if itemCache.IsAvailable(score.Id) {
				score.Timestamp = recommendTime
				candidates = append(candidates, score)
			}
		}

		// rank by click-through-rate
		var results []cache.Score
		if len(w.rankers) > 0 && w.rankers[workerId] != nil && !w.rankers[workerId].Invalid() {
			results, err = w.rankByClickTroughRate(w.rankers[workerId], &user, candidates, itemCache, recommendTime)
			if err != nil {
				log.Logger().Error("failed to rank items", zap.Error(err))
				return errors.Trace(err)
			}
		} else {
			results = candidates
		}

		if w.Config.Recommend.Replacement.EnableReplacement {
			results, err = w.replacement(w.rankers[workerId], results, &user,
				recommender.UserFeedback(), itemCache, recommendTime)
			if err != nil {
				log.Logger().Error("failed to insert historical items into recommendation",
					zap.String("user_id", userId), zap.Error(err))
				return errors.Trace(err)
			}
		}

		// cache recommendation
		if err = w.CacheClient.AddScores(ctx, cache.Recommend, userId, results); err != nil {
			log.Logger().Error("failed to cache recommendation", zap.Error(err))
			return errors.Trace(err)
		}
		if err = w.CacheClient.Set(ctx,
			cache.Time(cache.Key(cache.RecommendUpdateTime, userId), recommendTime),
			cache.String(cache.Key(cache.RecommendDigest, userId), digest),
		); err != nil {
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

func (w *Worker) updateCollaborativeRecommend(
	items *logics.MatrixFactorizationItems,
	userId string,
	userEmbedding []float32,
	excludeSet mapset.Set[string],
	itemCache *ItemCache,
) error {
	ctx := context.Background()
	localStartTime := time.Now()
	scores := items.Search(userEmbedding, w.Config.Recommend.CacheSize+excludeSet.Cardinality())
	// remove excluded items
	scores = lo.Filter(scores, func(score cache.Score, _ int) bool {
		return !excludeSet.Contains(score.Id)
	})
	// update categories
	for i := range scores {
		scores[i].Categories = itemCache.GetCategory(scores[i].Id)
		// the scores use the timestamp of the ranking index, which is only refreshed every so often.
		// if we don't overwrite the timestamp here, the code below will delete all scores that were
		// just written.
		scores[i].Timestamp = localStartTime
	}
	if err := w.CacheClient.AddScores(ctx, cache.CollaborativeFiltering, userId, scores); err != nil {
		log.Logger().Error("failed to cache collaborative filtering recommendation result", zap.String("user_id", userId), zap.Error(err))
		return errors.Trace(err)
	}
	if err := w.CacheClient.DeleteScores(ctx, []string{cache.CollaborativeFiltering}, cache.ScoreCondition{Before: &localStartTime, Subset: proto.String(userId)}); err != nil {
		log.Logger().Error("failed to delete stale collaborative filtering recommendation result", zap.String("user_id", userId), zap.Error(err))
		return errors.Trace(err)
	}
	return nil
}

// rankByClickTroughRate ranks items by predicted click-through-rate.
func (w *Worker) rankByClickTroughRate(
	predictor ctr.FactorizationMachines,
	user *data.User,
	candidates []cache.Score,
	itemCache *ItemCache,
	recommendTime time.Time,
) ([]cache.Score, error) {
	// download items
	items := make([]*data.Item, 0, len(candidates))
	for _, candidate := range candidates {
		if item, exist := itemCache.Get(candidate.Id); exist {
			items = append(items, item)
		} else {
			log.Logger().Warn("item doesn't exists in database", zap.String("item_id", candidate.Id))
		}
	}
	// rank by CTR
	topItems := make([]cache.Score, 0, len(items))
	if batchPredictor, ok := predictor.(ctr.BatchInference); ok {
		inputs := make([]lo.Tuple4[string, string, []ctr.Label, []ctr.Label], len(items))
		for i, item := range items {
			inputs[i].A = user.UserId
			inputs[i].B = item.ItemId
			inputs[i].C = ctr.ConvertLabels(user.Labels)
			inputs[i].D = ctr.ConvertLabels(item.Labels)
		}
		output := batchPredictor.BatchPredict(inputs, w.jobs)
		for i, score := range output {
			topItems = append(topItems, cache.Score{
				Id:         items[i].ItemId,
				Score:      float64(score),
				Categories: itemCache.GetCategory(items[i].ItemId),
				Timestamp:  recommendTime,
			})
		}
	} else {
		for _, item := range items {
			topItems = append(topItems, cache.Score{
				Id:         item.ItemId,
				Score:      float64(predictor.Predict(user.UserId, item.ItemId, ctr.ConvertLabels(user.Labels), ctr.ConvertLabels(item.Labels))),
				Categories: itemCache.GetCategory(item.ItemId),
				Timestamp:  recommendTime,
			})
		}
	}
	cache.SortDocuments(topItems)
	return topItems, nil
}

func (w *Worker) checkUserActiveTime(ctx context.Context, userId string) bool {
	if w.Config.Recommend.ActiveUserTTL == 0 {
		return true
	}
	// read active time
	activeTime, err := w.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		log.Logger().Error("failed to read last modify user time", zap.String("user_id", userId), zap.Error(err))
		return true
	}
	if activeTime.IsZero() {
		return true
	}
	// check active time
	if time.Since(activeTime) < time.Duration(w.Config.Recommend.ActiveUserTTL*24)*time.Hour {
		return true
	}
	// remove recommend cache for inactive users
	if err := w.CacheClient.DeleteScores(ctx, []string{cache.Recommend, cache.CollaborativeFiltering},
		cache.ScoreCondition{Subset: proto.String(userId)}); err != nil {
		log.Logger().Error("failed to delete recommend cache", zap.String("user_id", userId), zap.Error(err))
	}
	return false
}

// checkRecommendCacheOutOfDate checks if recommend cache stale.
func (w *Worker) checkRecommendCacheOutOfDate(ctx context.Context, userId string) bool {
	var (
		activeTime    time.Time
		recommendTime time.Time
		err           error
	)

	// 1. If cache is empty, stale.
	items, err := w.CacheClient.SearchScores(ctx, cache.Recommend, userId, nil, 0, -1)
	if err != nil {
		log.Logger().Error("failed to load offline recommendation", zap.String("user_id", userId), zap.Error(err))
		return true
	} else if len(items) == 0 {
		return true
	}

	// 2. If digest is empty or not match, stale.
	digest, err := w.CacheClient.Get(ctx, cache.Key(cache.RecommendDigest, userId)).String()
	if err != nil {
		log.Logger().Error("failed to read offline recommendation digest", zap.String("user_id", userId), zap.Error(err))
		return true
	}
	if digest == "" {
		return true
	}
	// read active time
	activeTime, err = w.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		log.Logger().Error("failed to read last modify user time", zap.String("user_id", userId), zap.Error(err))
	}

	// 3. If update time is empty, stale.
	recommendTime, err = w.CacheClient.Get(ctx, cache.Key(cache.RecommendUpdateTime, userId)).Time()
	if err != nil {
		log.Logger().Error("failed to read last update user recommend time", zap.Error(err))
		return true
	}

	// 4. If update time + cache expire > current time, not stale.
	if recommendTime.Before(time.Now().Add(-w.Config.Recommend.CacheExpire)) {
		return true
	}

	// 5. If active time > recommend time, not stale.
	if activeTime.Before(recommendTime) {
		timeoutTime := recommendTime.Add(w.Config.Recommend.Ranker.RefreshRecommendPeriod)
		return timeoutTime.Before(time.Now())
	}
	return true
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
	if !lo.Contains(peers, me) {
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
func (w *Worker) replacement(
	predictor ctr.FactorizationMachines,
	recommend []cache.Score,
	user *data.User,
	feedbacks []data.Feedback,
	itemCache *ItemCache,
	recommendTime time.Time,
) ([]cache.Score, error) {
	recommendItems := mapset.NewSet[string]()
	positiveItems := mapset.NewSet[string]()
	distinctItems := mapset.NewSet[string]()
	for _, r := range recommend {
		recommendItems.Add(r.Id)
	}
	newRecommend := make([]cache.Score, 0, len(recommend))
	newRecommend = append(newRecommend, recommend...)
	for _, feedback := range feedbacks {
		if expression.MatchFeedbackTypeExpressions(w.Config.Recommend.DataSource.PositiveFeedbackTypes, feedback.FeedbackType, feedback.Value) {
			positiveItems.Add(feedback.ItemId)
			distinctItems.Add(feedback.ItemId)
		} else if expression.MatchFeedbackTypeExpressions(w.Config.Recommend.DataSource.ReadFeedbackTypes, feedback.FeedbackType, feedback.Value) {
			distinctItems.Add(feedback.ItemId)
		}
	}
	negativeItems := distinctItems.Difference(positiveItems)

	items := make([]*data.Item, 0, distinctItems.Cardinality())
	for itemId := range distinctItems.Iter() {
		if item, exist := itemCache.Get(itemId); exist {
			items = append(items, item)
		}
	}
	scoredItems := make([]cache.Score, 0, len(items))
	if batchPredictor, ok := predictor.(ctr.BatchInference); ok {
		inputs := make([]lo.Tuple4[string, string, []ctr.Label, []ctr.Label], len(items))
		for i, item := range items {
			inputs[i].A = user.UserId
			inputs[i].B = item.ItemId
			inputs[i].C = ctr.ConvertLabels(user.Labels)
			inputs[i].D = ctr.ConvertLabels(item.Labels)
		}
		output := batchPredictor.BatchPredict(inputs, w.jobs)
		for i, score := range output {
			scoredItems = append(scoredItems, cache.Score{
				Id:         items[i].ItemId,
				Score:      float64(score),
				Categories: itemCache.GetCategory(items[i].ItemId),
				Timestamp:  recommendTime,
			})
		}
	} else {
		for _, item := range items {
			scoredItems = append(scoredItems, cache.Score{
				Id:         item.ItemId,
				Score:      float64(predictor.Predict(user.UserId, item.ItemId, ctr.ConvertLabels(user.Labels), ctr.ConvertLabels(item.Labels))),
				Categories: itemCache.GetCategory(item.ItemId),
				Timestamp:  recommendTime,
			})
		}
	}

	for _, scoredItem := range scoredItems {
		if recommendItems.Contains(scoredItem.Id) {
			continue
		}
		if positiveItems.Contains(scoredItem.Id) {
			scoredItem.Score *= w.Config.Recommend.Replacement.PositiveReplacementDecay
		} else if negativeItems.Contains(scoredItem.Id) {
			scoredItem.Score *= w.Config.Recommend.Replacement.ReadReplacementDecay
		} else {
			continue
		}
		newRecommend = append(newRecommend, scoredItem)
	}

	// rank items
	cache.SortDocuments(newRecommend)
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
