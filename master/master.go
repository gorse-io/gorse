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
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/server"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"math"
	"math/rand"
	"net"
	"os"
	"path/filepath"
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

const (
	NumUsers             = "NumUsers"
	NumItems             = "NumItems"
	NumUserLabels        = "NumUserLabels"
	NumItemLabels        = "NumItemLabels"
	NumTotalPosFeedbacks = "NumTotalPosFeedbacks"
	NumValidPosFeedbacks = "NumValidPosFeedbacks"
	NumValidNegFeedbacks = "NumValidNegFeedbacks"
)

// Master is the master node.
type Master struct {
	protocol.UnimplementedMasterServer
	server.RestServer

	taskMonitor   *TaskMonitor
	taskScheduler *TaskScheduler

	// cluster meta cache
	ttlCache       *ttlcache.Cache
	nodesInfo      map[string]*Node
	nodesInfoMutex sync.RWMutex

	// users index
	userIndex        base.Index
	userIndexVersion int64
	userIndexMutex   sync.RWMutex

	// ranking dataset
	rankingTrainSet  *ranking.DataSet
	rankingTestSet   *ranking.DataSet
	rankingDataMutex sync.RWMutex

	// click dataset
	clickTrainSet  *click.Dataset
	clickTestSet   *click.Dataset
	clickDataMutex sync.RWMutex

	// ranking model
	rankingModel         ranking.Model
	rankingModelName     string
	rankingModelVersion  int64
	rankingScore         ranking.Score
	rankingModelMutex    sync.RWMutex
	rankingModelSearcher *ranking.ModelSearcher

	// click model
	clickModel         click.FactorizationMachine
	clickScore         click.Score
	clickModelVersion  int64
	clickModelMutex    sync.RWMutex
	clickModelSearcher *click.ModelSearcher

	localCache *LocalCache

	// events
	fitTicker    *time.Ticker
	importedChan chan bool // feedback inserted events
}

// NewMaster creates a master node.
func NewMaster(cfg *config.Config) *Master {
	rand.Seed(time.Now().UnixNano())
	// create task monitor
	taskMonitor := NewTaskMonitor()
	for _, taskName := range []string{TaskLoadDataset, TaskFindItemNeighbors, TaskFindUserNeighbors,
		TaskFitRankingModel, TaskFitClickModel, TaskAnalyze, TaskSearchRankingModel, TaskSearchClickModel} {
		taskMonitor.Pending(taskName)
	}
	return &Master{
		nodesInfo: make(map[string]*Node),
		// create task monitor
		taskMonitor:   taskMonitor,
		taskScheduler: NewTaskScheduler(),
		// init versions
		rankingModelVersion: rand.Int63(),
		clickModelVersion:   rand.Int63(),
		userIndexVersion:    rand.Int63(),
		// default ranking model
		rankingModelName: "bpr",
		rankingModel:     ranking.NewBPR(nil),
		rankingModelSearcher: ranking.NewModelSearcher(
			cfg.Recommend.SearchEpoch,
			cfg.Recommend.SearchTrials,
			cfg.Master.NumJobs),
		// default click model
		clickModel: click.NewFM(click.FMClassification, nil),
		clickModelSearcher: click.NewModelSearcher(
			cfg.Recommend.SearchEpoch,
			cfg.Recommend.SearchTrials,
			cfg.Master.NumJobs,
		),
		RestServer: server.RestServer{
			GorseConfig: cfg,
			HttpHost:    cfg.Master.HttpHost,
			HttpPort:    cfg.Master.HttpPort,
			EnableAuth:  false,
			WebService:  new(restful.WebService),
		},
		fitTicker:    time.NewTicker(time.Duration(cfg.Recommend.FitPeriod) * time.Minute),
		importedChan: make(chan bool),
	}
}

// Serve starts the master node.
func (m *Master) Serve() {

	// load local cached model
	var err error
	m.localCache, err = LoadLocalCache(filepath.Join(os.TempDir(), "gorse-master"))
	if err != nil {
		base.Logger().Warn("failed to load local cache", zap.Error(err))
	}
	if m.localCache.RankingModel != nil {
		base.Logger().Info("load cached ranking model",
			zap.String("model_name", m.localCache.RankingModelName),
			zap.String("model_version", base.Hex(m.localCache.RankingModelVersion)),
			zap.Float32("model_score", m.localCache.RankingModelScore.NDCG),
			zap.Any("params", m.localCache.RankingModel.GetParams()))
		m.rankingModel = m.localCache.RankingModel
		m.rankingModelName = m.localCache.RankingModelName
		m.rankingModelVersion = m.localCache.RankingModelVersion
		m.rankingScore = m.localCache.RankingModelScore
	}
	if m.localCache.ClickModel != nil {
		base.Logger().Info("load cached click model",
			zap.String("model_version", base.Hex(m.localCache.ClickModelVersion)),
			zap.Float32("model_score", m.localCache.ClickModelScore.Precision),
			zap.Any("params", m.localCache.ClickModel.GetParams()))
		m.clickModel = m.localCache.ClickModel
		m.clickScore = m.localCache.ClickModelScore
		m.clickModelVersion = m.localCache.ClickModelVersion
	}

	// create cluster meta cache
	m.ttlCache = ttlcache.NewCache()
	m.ttlCache.SetExpirationCallback(m.nodeDown)
	m.ttlCache.SetNewItemCallback(m.nodeUp)
	if err = m.ttlCache.SetTTL(
		time.Duration(m.GorseConfig.Master.MetaTimeout+10) * time.Second,
	); err != nil {
		base.Logger().Fatal("failed to set TTL", zap.Error(err))
	}

	// connect data database
	m.DataClient, err = data.Open(m.GorseConfig.Database.DataStore)
	if err != nil {
		base.Logger().Fatal("failed to connect data database", zap.Error(err))
	}
	if err = m.DataClient.Init(); err != nil {
		base.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// connect cache database
	m.CacheClient, err = cache.Open(m.GorseConfig.Database.CacheStore)
	if err != nil {
		base.Logger().Fatal("failed to connect cache database", zap.Error(err),
			zap.String("database", m.GorseConfig.Database.CacheStore))
	}

	// pre-lock privileged tasks
	tasksNames := []string{TaskLoadDataset, TaskFindItemNeighbors, TaskFindUserNeighbors, TaskFitRankingModel, TaskFitClickModel}
	for _, taskName := range tasksNames {
		m.taskScheduler.PreLock(taskName)
	}

	go m.StartHttpServer()
	go m.RunPrivilegedTasksLoop()
	base.Logger().Info("start model fit", zap.Int("period", m.GorseConfig.Recommend.FitPeriod))
	go m.RunRagtagTasksLoop()
	base.Logger().Info("start model searcher", zap.Int("period", m.GorseConfig.Recommend.SearchPeriod))

	// start rpc server
	base.Logger().Info("start rpc server",
		zap.String("host", m.GorseConfig.Master.Host),
		zap.Int("port", m.GorseConfig.Master.Port))
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", m.GorseConfig.Master.Host, m.GorseConfig.Master.Port))
	if err != nil {
		base.Logger().Fatal("failed to listen", zap.Error(err))
	}
	grpcServer := grpc.NewServer(grpc.MaxSendMsgSize(math.MaxInt))
	protocol.RegisterMasterServer(grpcServer, m)
	if err = grpcServer.Serve(lis); err != nil {
		base.Logger().Fatal("failed to start rpc server", zap.Error(err))
	}
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
			base.Logger().Error("failed to load ranking dataset", zap.Error(err))
			continue
		}

		// fit ranking model
		lastNumRankingUsers, lastNumRankingItems, lastNumRankingFeedback, err =
			m.runRankingRelatedTasks(lastNumRankingUsers, lastNumRankingItems, lastNumRankingFeedback)
		if err != nil {
			base.Logger().Error("failed to fit ranking model", zap.Error(err))
			continue
		}

		// fit click model
		lastNumClickUsers, lastNumClickItems, lastNumClickFeedback, err =
			m.runFitClickModelTask(lastNumClickUsers, lastNumClickItems, lastNumClickFeedback)
		if err != nil {
			base.Logger().Error("failed to fit click model", zap.Error(err))
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
		// analyze click-through-rate
		if err := m.runAnalyzeTask(); err != nil {
			base.Logger().Error("failed to analyze", zap.Error(err))
			m.taskMonitor.Fail(TaskAnalyze, err.Error())
		}
		// search optimal ranking model
		lastNumRankingUsers, lastNumRankingItems, lastNumRankingFeedbacks, err =
			m.runSearchRankingModelTask(lastNumRankingUsers, lastNumRankingItems, lastNumRankingFeedbacks)
		if err != nil {
			base.Logger().Error("failed to search ranking model", zap.Error(err))
			m.taskMonitor.Fail(TaskSearchRankingModel, err.Error())
			time.Sleep(time.Minute)
			continue
		}
		// search optimal click model
		lastNumClickUsers, lastNumClickItems, lastNumClickFeedbacks, err =
			m.runSearchClickModelTask(lastNumClickUsers, lastNumClickItems, lastNumClickFeedbacks)
		if err != nil {
			base.Logger().Error("failed to search click model", zap.Error(err))
			m.taskMonitor.Fail(TaskSearchClickModel, err.Error())
			time.Sleep(time.Minute)
			continue
		}
		time.Sleep(time.Duration(m.GorseConfig.Recommend.SearchPeriod) * time.Minute)
	}
}

func (m *Master) checkDataImported() bool {
	isDataImported, err := m.CacheClient.GetInt(cache.GlobalMeta, cache.DataImported)
	if err != nil {
		base.Logger().Error("failed to read meta", zap.Error(err))
		return false
	}
	if isDataImported > 0 {
		err = m.CacheClient.SetInt(cache.GlobalMeta, cache.DataImported, 0)
		if err != nil {
			base.Logger().Error("failed to write meta", zap.Error(err))
		}
		return true
	}
	return false
}

func (m *Master) notifyDataImported() {
	err := m.CacheClient.IncrInt(cache.GlobalMeta, cache.DataImported)
	if err != nil {
		base.Logger().Error("failed to write meta", zap.Error(err))
	}
}
