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

package master

import (
	"context"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/server"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
)

// Master is the master node.
type Master struct {
	protocol.UnimplementedMasterServer
	server.RestServer
	grpcServer *grpc.Server

	taskMonitor   *task.Monitor
	taskScheduler *task.Scheduler
	cacheFile     string

	// cluster meta cache
	ttlCache       *ttlcache.Cache
	nodesInfo      map[string]*Node
	nodesInfoMutex sync.RWMutex

	// ranking dataset
	rankingTrainSet  *ranking.DataSet
	rankingTestSet   *ranking.DataSet
	rankingDataMutex sync.RWMutex

	// click dataset
	clickTrainSet  *click.Dataset
	clickTestSet   *click.Dataset
	clickDataMutex sync.RWMutex

	// ranking model
	rankingModelName     string
	rankingScore         ranking.Score
	rankingModelMutex    sync.RWMutex
	rankingModelSearcher *ranking.ModelSearcher

	// click model
	clickScore         click.Score
	clickModelMutex    sync.RWMutex
	clickModelSearcher *click.ModelSearcher

	localCache *LocalCache

	// events
	fitTicker    *time.Ticker
	importedChan chan bool // feedback inserted events
}

// NewMaster creates a master node.
func NewMaster(cfg *config.Config, cacheFile string) *Master {
	rand.Seed(time.Now().UnixNano())
	// create task monitor
	taskMonitor := task.NewTaskMonitor()
	for _, taskName := range []string{TaskLoadDataset, TaskFindItemNeighbors, TaskFindUserNeighbors,
		TaskFitRankingModel, TaskFitClickModel, TaskSearchRankingModel, TaskSearchClickModel,
		TaskCacheGarbageCollection} {
		taskMonitor.Pending(taskName)
	}
	return &Master{
		nodesInfo: make(map[string]*Node),
		// create task monitor
		cacheFile:     cacheFile,
		taskMonitor:   taskMonitor,
		taskScheduler: task.NewTaskScheduler(),
		// default ranking model
		rankingModelName: "bpr",
		rankingModelSearcher: ranking.NewModelSearcher(
			cfg.Recommend.Collaborative.ModelSearchEpoch,
			cfg.Recommend.Collaborative.ModelSearchTrials,
			task.NewConstantJobsAllocator(cfg.Master.NumJobs),
			cfg.Recommend.Collaborative.EnableModelSizeSearch,
		),
		// default click model
		clickModelSearcher: click.NewModelSearcher(
			cfg.Recommend.Collaborative.ModelSearchEpoch,
			cfg.Recommend.Collaborative.ModelSearchTrials,
			task.NewConstantJobsAllocator(cfg.Master.NumJobs),
			cfg.Recommend.Collaborative.EnableModelSizeSearch,
		),
		RestServer: server.RestServer{
			Settings: &config.Settings{
				Config:       cfg,
				CacheClient:  cache.NoDatabase{},
				DataClient:   data.NoDatabase{},
				RankingModel: ranking.NewBPR(nil),
				ClickModel:   click.NewFM(click.FMClassification, nil),
				// init versions
				RankingModelVersion: rand.Int63(),
				ClickModelVersion:   rand.Int63(),
			},
			HttpHost:   cfg.Master.HttpHost,
			HttpPort:   cfg.Master.HttpPort,
			WebService: new(restful.WebService),
		},
		fitTicker:    time.NewTicker(cfg.Recommend.Collaborative.ModelFitPeriod),
		importedChan: make(chan bool),
	}
}

// Serve starts the master node.
func (m *Master) Serve() {

	// load local cached model
	var err error
	m.localCache, err = LoadLocalCache(m.cacheFile)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			log.Logger().Info("no local cache found, create a new one", zap.String("path", m.cacheFile))
		} else {
			log.Logger().Error("failed to load local cache", zap.String("path", m.cacheFile), zap.Error(err))
		}
	}
	if m.localCache.RankingModel != nil {
		log.Logger().Info("load cached ranking model",
			zap.String("model_name", m.localCache.RankingModelName),
			zap.String("model_version", encoding.Hex(m.localCache.RankingModelVersion)),
			zap.Float32("model_score", m.localCache.RankingModelScore.NDCG),
			zap.Any("params", m.localCache.RankingModel.GetParams()))
		m.RankingModel = m.localCache.RankingModel
		m.rankingModelName = m.localCache.RankingModelName
		m.RankingModelVersion = m.localCache.RankingModelVersion
		m.rankingScore = m.localCache.RankingModelScore
		CollaborativeFilteringPrecision10.Set(float64(m.rankingScore.Precision))
		CollaborativeFilteringRecall10.Set(float64(m.rankingScore.Recall))
		CollaborativeFilteringNDCG10.Set(float64(m.rankingScore.NDCG))
		MemoryInuseBytesVec.WithLabelValues("collaborative_filtering_model").Set(float64(m.RankingModel.Bytes()))
	}
	if m.localCache.ClickModel != nil {
		log.Logger().Info("load cached click model",
			zap.String("model_version", encoding.Hex(m.localCache.ClickModelVersion)),
			zap.Float32("model_score", m.localCache.ClickModelScore.Precision),
			zap.Any("params", m.localCache.ClickModel.GetParams()))
		m.ClickModel = m.localCache.ClickModel
		m.clickScore = m.localCache.ClickModelScore
		m.ClickModelVersion = m.localCache.ClickModelVersion
		RankingPrecision.Set(float64(m.clickScore.Precision))
		RankingRecall.Set(float64(m.clickScore.Recall))
		RankingAUC.Set(float64(m.clickScore.AUC))
		MemoryInuseBytesVec.WithLabelValues("ranking_model").Set(float64(m.ClickModel.Bytes()))
	}

	// create cluster meta cache
	m.ttlCache = ttlcache.NewCache()
	m.ttlCache.SetExpirationCallback(m.nodeDown)
	m.ttlCache.SetNewItemCallback(m.nodeUp)
	if err = m.ttlCache.SetTTL(m.Config.Master.MetaTimeout + 10*time.Second); err != nil {
		log.Logger().Fatal("failed to set TTL", zap.Error(err))
	}

	// connect data database
	m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.TablePrefix)
	if err != nil {
		log.Logger().Fatal("failed to connect data database", zap.Error(err),
			zap.String("database", log.RedactDBURL(m.Config.Database.DataStore)))
	}
	if err = m.DataClient.Init(); err != nil {
		log.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// connect cache database
	m.CacheClient, err = cache.Open(m.Config.Database.CacheStore, m.Config.Database.TablePrefix)
	if err != nil {
		log.Logger().Fatal("failed to connect cache database", zap.Error(err),
			zap.String("database", log.RedactDBURL(m.Config.Database.CacheStore)))
	}
	if err = m.CacheClient.Init(); err != nil {
		log.Logger().Fatal("failed to init database", zap.Error(err))
	}

	m.RestServer.HiddenItemsManager = server.NewHiddenItemsManager(&m.RestServer)
	m.RestServer.PopularItemsCache = server.NewPopularItemsCache(&m.RestServer)

	// pre-lock privileged tasks
	tasksNames := []string{TaskLoadDataset, TaskFindItemNeighbors, TaskFindUserNeighbors, TaskFitRankingModel, TaskFitClickModel}
	for _, taskName := range tasksNames {
		m.taskScheduler.PreLock(taskName)
	}

	go m.RunPrivilegedTasksLoop()
	log.Logger().Info("start model fit", zap.Duration("period", m.Config.Recommend.Collaborative.ModelFitPeriod))
	go m.RunRagtagTasksLoop()
	log.Logger().Info("start model searcher", zap.Duration("period", m.Config.Recommend.Collaborative.ModelSearchPeriod))

	// start rpc server
	go func() {
		log.Logger().Info("start rpc server",
			zap.String("host", m.Config.Master.Host),
			zap.Int("port", m.Config.Master.Port))
		lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", m.Config.Master.Host, m.Config.Master.Port))
		if err != nil {
			log.Logger().Fatal("failed to listen", zap.Error(err))
		}
		m.grpcServer = grpc.NewServer(grpc.MaxSendMsgSize(math.MaxInt))
		protocol.RegisterMasterServer(m.grpcServer, m)
		if err = m.grpcServer.Serve(lis); err != nil {
			log.Logger().Fatal("failed to start rpc server", zap.Error(err))
		}
	}()

	// start http server
	m.StartHttpServer()
}

func (m *Master) Shutdown() {
	// stop http server
	err := m.HttpServer.Shutdown(context.TODO())
	if err != nil {
		log.Logger().Error("failed to shutdown http server", zap.Error(err))
	}
	// stop grpc server
	m.grpcServer.GracefulStop()
}

func (m *Master) RunPrivilegedTasksLoop() {
	defer base.CheckPanic()
	var (
		lastNumRankingUsers    int
		lastNumRankingItems    int
		lastNumRankingFeedback int
		lastNumClickUsers      int
		lastNumClickItems      int
		lastNumClickFeedback   int
		err                    error
	)
	go func() {
		m.importedChan <- true
		for {
			if m.checkDataImported() {
				m.importedChan <- true
			}
			time.Sleep(time.Second)
		}
	}()
	for {
		select {
		case <-m.fitTicker.C:
		case <-m.importedChan:
		}
		// pre-lock privileged tasks
		tasksNames := []string{TaskLoadDataset, TaskFindItemNeighbors, TaskFindUserNeighbors, TaskFitRankingModel, TaskFitClickModel}
		for _, taskName := range tasksNames {
			m.taskScheduler.PreLock(taskName)
		}

		// download dataset
		err = m.runLoadDatasetTask()
		if err != nil {
			log.Logger().Error("failed to load ranking dataset", zap.Error(err))
			continue
		}

		// fit ranking model
		lastNumRankingUsers, lastNumRankingItems, lastNumRankingFeedback, err =
			m.runRankingRelatedTasks(lastNumRankingUsers, lastNumRankingItems, lastNumRankingFeedback)
		if err != nil {
			log.Logger().Error("failed to fit ranking model", zap.Error(err))
			continue
		}

		// fit click model
		lastNumClickUsers, lastNumClickItems, lastNumClickFeedback, err =
			m.runFitClickModelTask(lastNumClickUsers, lastNumClickItems, lastNumClickFeedback)
		if err != nil {
			log.Logger().Error("failed to fit click model", zap.Error(err))
			m.taskMonitor.Fail(TaskFitClickModel, err.Error())
			continue
		}

		// release locks
		for _, taskName := range tasksNames {
			m.taskScheduler.UnLock(taskName)
		}
	}
}

// RunRagtagTasksLoop searches optimal recommendation model in background. It never modifies variables other than
// rankingModelSearcher, clickSearchedModel and clickSearchedScore.
func (m *Master) RunRagtagTasksLoop() {
	defer base.CheckPanic()
	var (
		lastNumRankingUsers     int
		lastNumRankingItems     int
		lastNumRankingFeedbacks int
		lastNumClickUsers       int
		lastNumClickItems       int
		lastNumClickFeedbacks   int
		err                     error
	)
	for {
		// garbage collection
		m.taskScheduler.Lock(TaskCacheGarbageCollection)
		if err = m.runCacheGarbageCollectionTask(); err != nil {
			log.Logger().Error("failed to collect garbage", zap.Error(err))
			m.taskMonitor.Fail(TaskCacheGarbageCollection, err.Error())
		}
		m.taskScheduler.UnLock(TaskCacheGarbageCollection)
		// search optimal ranking model
		lastNumRankingUsers, lastNumRankingItems, lastNumRankingFeedbacks, err =
			m.runSearchRankingModelTask(lastNumRankingUsers, lastNumRankingItems, lastNumRankingFeedbacks)
		if err != nil {
			log.Logger().Error("failed to search ranking model", zap.Error(err))
			m.taskMonitor.Fail(TaskSearchRankingModel, err.Error())
			time.Sleep(time.Minute)
			continue
		}
		// search optimal click model
		lastNumClickUsers, lastNumClickItems, lastNumClickFeedbacks, err =
			m.runSearchClickModelTask(lastNumClickUsers, lastNumClickItems, lastNumClickFeedbacks)
		if err != nil {
			log.Logger().Error("failed to search click model", zap.Error(err))
			m.taskMonitor.Fail(TaskSearchClickModel, err.Error())
			time.Sleep(time.Minute)
			continue
		}
		time.Sleep(m.Config.Recommend.Collaborative.ModelSearchPeriod)
	}
}

func (m *Master) checkDataImported() bool {
	isDataImported, err := m.CacheClient.Get(cache.Key(cache.GlobalMeta, cache.DataImported)).Integer()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read meta", zap.Error(err))
		}
		return false
	}
	if isDataImported > 0 {
		err = m.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.DataImported), 0))
		if err != nil {
			log.Logger().Error("failed to write meta", zap.Error(err))
		}
		return true
	}
	return false
}

func (m *Master) notifyDataImported() {
	err := m.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.DataImported), 1))
	if err != nil {
		log.Logger().Error("failed to write meta", zap.Error(err))
	}
}
