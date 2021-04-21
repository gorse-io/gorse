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
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ReneKroon/ttlcache/v2"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/pr"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/grpc"
)

const (
	ServerNode = "server"
	WorkerNode = "worker"
)

type Master struct {
	protocol.UnimplementedMasterServer

	// cluster meta cache
	ttlCache       *ttlcache.Cache
	nodesInfo      map[string]string
	nodesInfoMutex sync.Mutex

	// configuration
	cfg  *config.Config
	meta *toml.MetaData

	// database connection
	dataStore  data.Database
	cacheStore cache.Database

	// users index
	userIndex        base.Index
	userIndexVersion int64
	userIndexMutex   sync.Mutex

	// personal ranking model
	prModel     pr.Model
	prModelName string
	prVersion   int64
	prMutex     sync.Mutex
	prSearcher  *pr.ModelSearcher

	// factorization machine
	//fmModel    ctr.FactorizationMachine
	//ctrVersion int64
	//fmMutex    sync.mutex
}

func NewMaster(cfg *config.Config, meta *toml.MetaData) *Master {
	rand.Seed(time.Now().UnixNano())
	return &Master{
		nodesInfo: make(map[string]string),
		cfg:       cfg,
		meta:      meta,
		// init versions
		prVersion: rand.Int63(),
		// ctrVersion:       rand.Int63(),
		userIndexVersion: rand.Int63(),
		// default model
		prModelName: "knn",
		prModel:     pr.NewKNN(nil),
		prSearcher:  pr.NewModelSearcher(cfg.Recommend.SearchEpoch, cfg.Recommend.SearchTrials),
	}
}

func (m *Master) Serve() {

	// create cluster meta cache
	m.ttlCache = ttlcache.NewCache()
	m.ttlCache.SetExpirationCallback(m.nodeDown)
	m.ttlCache.SetNewItemCallback(m.nodeUp)
	if err := m.ttlCache.SetTTL(
		time.Duration(m.cfg.Master.MetaTimeout+10) * time.Second,
	); err != nil {
		base.Logger().Fatal("failed to set TTL", zap.Error(err))
	}

	// connect data database
	var err error
	m.dataStore, err = data.Open(m.cfg.Database.DataStore)
	if err != nil {
		base.Logger().Fatal("failed to connect data database", zap.Error(err))
	}
	if err = m.dataStore.Init(); err != nil {
		base.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// connect cache database
	m.cacheStore, err = cache.Open(m.cfg.Database.CacheStore)
	if err != nil {
		base.Logger().Fatal("failed to connect cache database", zap.Error(err),
			zap.String("database", m.cfg.Database.CacheStore))
	}

	// start loop
	go m.FitLoop()
	base.Logger().Info("start model fit", zap.Int("period", m.cfg.Recommend.FitPeriod))
	go m.SearchLoop()
	base.Logger().Info("start model searcher", zap.Int("period", m.cfg.Recommend.SearchPeriod))

	// start rpc server
	base.Logger().Info("start rpc server",
		zap.String("host", m.cfg.Master.Host),
		zap.Int("port", m.cfg.Master.Port))
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", m.cfg.Master.Host, m.cfg.Master.Port))
	if err != nil {
		base.Logger().Fatal("failed to listen", zap.Error(err))
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	protocol.RegisterMasterServer(grpcServer, m)
	if err = grpcServer.Serve(lis); err != nil {
		base.Logger().Fatal("failed to start rpc server", zap.Error(err))
	}
}

func (m *Master) FitLoop() {
	defer base.CheckPanic()
	lastNumUsers, lastNumItems, lastNumFeedback := 0, 0, 0
	for {
		// download dataset
		base.Logger().Info("load dataset for personal ranking", zap.Strings("feedback_types", m.cfg.Database.PositiveFeedbackType))
		dataSet, items, feedbacks, err := pr.LoadDataFromDatabase(m.dataStore, m.cfg.Database.PositiveFeedbackType)
		if err != nil {
			base.Logger().Error("failed to load database", zap.Error(err))
			goto sleep
		}
		// sleep if empty
		if dataSet.Count() == 0 {
			base.Logger().Warn("empty dataset", zap.Strings("feedback_type", m.cfg.Database.PositiveFeedbackType))
			goto sleep
		}
		// sleep if nothing changed
		if dataSet.UserCount() == lastNumUsers && dataSet.ItemCount() == lastNumItems && dataSet.Count() == lastNumFeedback {
			goto sleep
		}
		base.Logger().Info("data loaded for matching",
			zap.Int("n_users", dataSet.UserCount()),
			zap.Int("n_items", dataSet.ItemCount()),
			zap.Int("n_feedback", dataSet.Count()))
		lastNumUsers, lastNumItems, lastNumFeedback = dataSet.UserCount(), dataSet.ItemCount(), dataSet.Count()
		// update user index
		m.userIndexMutex.Lock()
		m.userIndex = dataSet.UserIndex
		m.userIndexVersion++
		m.userIndexMutex.Unlock()
		// fit model
		m.fitPRModel(dataSet, m.prModel)
		// collect similar items
		m.similar(items, dataSet, model.SimilarityDot)
		// collect popular items
		m.popItem(items, feedbacks)
		// collect latest items
		m.latest(items)
		// sleep
	sleep:
		time.Sleep(time.Duration(m.cfg.Recommend.FitPeriod) * time.Minute)
	}
}

func (m *Master) SearchLoop() {
	defer base.CheckPanic()
	lastNumUsers, lastNumItems, lastNumFeedback := 0, 0, 0
	for {
		var trainSet, valSet *pr.DataSet
		// download dataset
		base.Logger().Info("load dataset for personal ranking", zap.Strings("feedback_types", m.cfg.Database.PositiveFeedbackType))
		dataSet, _, _, err := pr.LoadDataFromDatabase(m.dataStore, m.cfg.Database.PositiveFeedbackType)
		if err != nil {
			base.Logger().Error("failed to load database", zap.Error(err))
			goto sleep
		}
		// sleep if empty
		if dataSet.Count() == 0 {
			base.Logger().Warn("empty dataset", zap.Strings("feedback_type", m.cfg.Database.PositiveFeedbackType))
			goto sleep
		}
		// sleep if nothing changed
		if dataSet.UserCount() == lastNumUsers && dataSet.ItemCount() == lastNumItems && dataSet.Count() == lastNumFeedback {
			goto sleep
		}
		// start search
		trainSet, valSet = dataSet.Split(0, 0)
		err = m.prSearcher.Fit(trainSet, valSet)
		if err != nil {
			base.Logger().Error("failed to search model", zap.Error(err))
		}
	sleep:
		time.Sleep(time.Duration(m.cfg.Recommend.SearchPeriod) * time.Minute)
	}
}
