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
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ReneKroon/ttlcache/v2"
	"github.com/araddon/dateparse"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/model/rank"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
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
	userIndex   base.Index
	userVersion int
	userMutex   sync.Mutex

	// matrix factorization
	mfModel   cf.MatrixFactorization
	mfVersion int
	mfMutex   sync.Mutex

	// factorization machine
	fmModel   rank.FactorizationMachine
	fmVersion int
	fmMutex   sync.Mutex
}

func NewMaster(cfg *config.Config, meta *toml.MetaData) *Master {
	l := &Master{
		nodesInfo:   make(map[string]string),
		cfg:         cfg,
		meta:        meta,
		mfVersion:   rand.Int(),
		fmVersion:   rand.Int(),
		userVersion: rand.Int(),
	}
	return l
}

func (m *Master) Serve() {

	// create cluster meta cache
	m.ttlCache = ttlcache.NewCache()
	m.ttlCache.SetExpirationCallback(m.nodeDown)
	m.ttlCache.SetNewItemCallback(m.nodeUp)
	if err := m.ttlCache.SetTTL(
		time.Duration(m.cfg.Master.ClusterMetaTimeout) * time.Second,
	); err != nil {
		log.Error("master:", err)
	}

	// connect data database
	var err error
	m.dataStore, err = data.Open(m.cfg.Database.DataStore)
	if err != nil {
		log.Fatalf("master: failed to connect data database (%v)", err)
	}
	if err = m.dataStore.Init(); err != nil {
		base.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// connect cache database
	m.cacheStore, err = cache.Open(m.cfg.Database.CacheStore)
	if err != nil {
		log.Fatalf("master: failed to connect cache database (%v)", err)
	}

	// start loop
	go m.Loop()

	// start rpc server
	log.Infof("master: start rpc server %v:%v", m.cfg.Master.Host, m.cfg.Master.Port)
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", m.cfg.Master.Host, m.cfg.Master.Port))
	if err != nil {
		log.Fatalf("master: failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	protocol.RegisterMasterServer(grpcServer, m)
	if err = grpcServer.Serve(lis); err != nil {
		log.Fatalf("master: failed to start rpc server (%v)", err)
	}
}

func (m *Master) GetConfig(context.Context, *protocol.Void) (*protocol.Config, error) {
	s, err := json.Marshal(m.cfg)
	if err != nil {
		return nil, err
	}
	return &protocol.Config{Json: string(s)}, nil
}

func (m *Master) RegisterServer(ctx context.Context, _ *protocol.Void) (*protocol.Void, error) {
	p, _ := peer.FromContext(ctx)
	addr := p.Addr.String()
	if err := m.ttlCache.Set(addr, ServerNode); err != nil {
		log.Errorf("master: failed to set ttlcache (%v)", err)
		return nil, err
	}
	return &protocol.Void{}, nil
}

func (m *Master) RegisterWorker(ctx context.Context, _ *protocol.Void) (*protocol.Void, error) {
	p, _ := peer.FromContext(ctx)
	addr := p.Addr.String()
	if err := m.ttlCache.Set(addr, WorkerNode); err != nil {
		log.Errorf("master: failed to set ttlcache (%v)", err)
		return nil, err
	}
	return &protocol.Void{}, nil
}

func (m *Master) GetCluster(ctx context.Context, _ *protocol.Void) (*protocol.Cluster, error) {
	cluster := &protocol.Cluster{
		Workers: make([]string, 0),
		Servers: make([]string, 0),
	}
	// add me
	p, _ := peer.FromContext(ctx)
	cluster.Me = p.Addr.String()
	// add master
	cluster.Master = fmt.Sprintf("%s:%d", m.cfg.Master.Host, m.cfg.Master.Port)
	// add servers/workers
	m.nodesInfoMutex.Lock()
	defer m.nodesInfoMutex.Unlock()
	for addr, nodeType := range m.nodesInfo {
		switch nodeType {
		case WorkerNode:
			cluster.Workers = append(cluster.Workers, addr)
		case ServerNode:
			cluster.Servers = append(cluster.Servers, addr)
		default:
			log.Fatalf("master: unkown node (%v)", nodeType)
		}
	}
	return cluster, nil
}

func (m *Master) GetCollaborativeFilteringModelVersion(context.Context, *protocol.Void) (*protocol.Model, error) {
	m.mfMutex.Lock()
	defer m.mfMutex.Unlock()
	// skip empty model
	if m.mfModel == nil {
		return &protocol.Model{Version: 0}, nil
	}
	return &protocol.Model{
		Name:    m.cfg.Collaborative.CFModel,
		Version: int64(m.mfVersion),
	}, nil
}

func (m *Master) GetCollaborativeFilteringModel(context.Context, *protocol.Void) (*protocol.Model, error) {
	m.mfMutex.Lock()
	defer m.mfMutex.Unlock()
	// skip empty model
	if m.mfModel == nil {
		return &protocol.Model{Version: 0}, nil
	}
	// encode model
	modelData, err := cf.EncodeModel(m.mfModel)
	if err != nil {
		return nil, err
	}
	return &protocol.Model{
		Name:    m.cfg.Collaborative.CFModel,
		Version: int64(m.mfVersion),
		Model:   modelData,
	}, nil
}

func (m *Master) GetFactorizationMachineVersion(context.Context, *protocol.Void) (*protocol.Model, error) {
	m.fmMutex.Lock()
	defer m.fmMutex.Unlock()
	// skip empty model
	if m.fmModel == nil {
		return &protocol.Model{Version: 0}, nil
	}
	return &protocol.Model{Version: int64(m.fmVersion)}, nil
}

func (m *Master) GetFactorizationMachine(context.Context, *protocol.Void) (*protocol.Model, error) {
	m.fmMutex.Lock()
	defer m.fmMutex.Unlock()
	// skip empty model
	if m.fmModel == nil {
		return &protocol.Model{Version: 0}, nil
	}
	// encode model
	modelData, err := rank.EncodeModel(m.fmModel)
	if err != nil {
		return nil, err
	}
	return &protocol.Model{
		Version: int64(m.fmVersion),
		Model:   modelData,
	}, nil
}

func (m *Master) GetUserIndexVersion(context.Context, *protocol.Void) (*protocol.Users, error) {
	m.userMutex.Lock()
	defer m.userMutex.Unlock()
	if m.userIndex == nil {
		return &protocol.Users{Version: 0}, nil
	}
	return &protocol.Users{Version: int64(m.fmVersion)}, nil
}

func (m *Master) GetUserIndex(context.Context, *protocol.Void) (*protocol.Users, error) {
	m.userMutex.Lock()
	defer m.userMutex.Unlock()
	// skip empty model
	if m.userIndex == nil {
		return &protocol.Users{Version: 0}, nil
	}
	// encode index
	buf := bytes.NewBuffer(nil)
	writer := bufio.NewWriter(buf)
	encoder := gob.NewEncoder(writer)
	if err := encoder.Encode(m.userIndex); err != nil {
		return nil, err
	}
	return &protocol.Users{
		Version: int64(m.fmVersion),
		Users:   buf.Bytes(),
	}, nil
}

func (m *Master) nodeUp(key string, value interface{}) {
	nodeType := value.(string)
	base.Logger().Info("node up",
		zap.String("node_id", key),
		zap.String("node_type", nodeType))
	m.nodesInfoMutex.Lock()
	defer m.nodesInfoMutex.Unlock()
	m.nodesInfo[key] = nodeType
}

func (m *Master) nodeDown(key string, value interface{}) {
	nodeType := value.(string)
	base.Logger().Info("node down",
		zap.String("node_id", key),
		zap.String("node_type", nodeType))
	m.nodesInfoMutex.Lock()
	defer m.nodesInfoMutex.Unlock()
	delete(m.nodesInfo, key)
}

func (m *Master) Loop() {
	defer base.CheckPanic()

	// calculate loop period
	loopPeriod := base.GCD(
		m.cfg.Collaborative.FitPeriod,
		m.cfg.Rank.FitPeriod,
		m.cfg.Similar.UpdatePeriod,
		m.cfg.Popular.UpdatePeriod,
		m.cfg.Latest.UpdatePeriod)
	log.Infof("master: start loop (period = %v min)", loopPeriod)

	for {
		// check stale
		isPopItemStale := m.IsStale(cache.CollectPopularTime, m.cfg.Popular.UpdatePeriod)
		isLatestStale := m.IsStale(cache.CollectLatestTime, m.cfg.Latest.UpdatePeriod)
		isSimilarStale := m.IsStale(cache.CollectSimilarTime, m.cfg.Similar.UpdatePeriod)
		isRankModelStale := m.IsStale(cache.FitFactorizationMachineTime, m.cfg.Rank.FitPeriod)
		isCFModelStale := m.IsStale(cache.FitMatrixFactorizationTime, m.cfg.Collaborative.FitPeriod)

		// pull dataset for rank
		if isRankModelStale || m.fmModel == nil {
			base.Logger().Info("load dataset for ranking task", zap.Strings("feedback_types", m.cfg.Database.RankFeedbackType))
			rankDataSet, err := rank.LoadDataFromDatabase(m.dataStore, m.cfg.Database.RankFeedbackType)
			if err != nil {
				log.Fatalf("master: failed to pull dataset for ranking (%v)", err)
			}
			if rankDataSet.PositiveCount == 0 {
				log.Infof("master: empty dataset (feedback_type = %v)", m.cfg.Database.RankFeedbackType)
			} else {
				m.FitFactorizationMachine(rankDataSet)
			}
		}

		if isCFModelStale || isLatestStale || isPopItemStale || isSimilarStale || m.mfModel == nil {
			// download dataset
			base.Logger().Info("load dataset for matching task", zap.Strings("feedback_types", m.cfg.Database.MatchFeedbackType))
			dataSet, items, feedbacks, err := cf.LoadDataFromDatabase(m.dataStore, m.cfg.Database.MatchFeedbackType)
			if err != nil {
				log.Fatal("master: ", err)
			}
			if dataSet.Count() == 0 {
				log.Info("master: empty dataset")
			} else {
				log.Infof("master: data loaded (#user = %v, #item = %v, #feedback = %v)",
					dataSet.UserCount(), dataSet.ItemCount(), dataSet.Count())
				// update user index
				m.userMutex.Lock()
				m.userIndex = dataSet.UserIndex
				m.userVersion++
				m.userMutex.Unlock()
				// collect popular items
				if isPopItemStale {
					m.CollectPopItem(items, feedbacks)
				}
				// collect latest items
				if isLatestStale {
					m.CollectLatest(items)
				}
				// collect similar items
				if isSimilarStale {
					m.CollectSimilar(items, dataSet)
				}
				if isCFModelStale || m.mfModel == nil {
					m.FitMatrixFactorization(dataSet)
				}
			}
		}

		// sleep
		time.Sleep(time.Duration(loopPeriod) * time.Minute)
	}
}

func (m *Master) FitFactorizationMachine(dataSet *rank.Dataset) {
	base.Logger().Info("fit factorization machine",
		zap.Float32("split_ratio", m.cfg.Rank.SplitRatio))
	trainSet, testSet := dataSet.Split(m.cfg.Rank.SplitRatio, 0)
	testSet.NegativeSample(1, trainSet, 0)
	fmModel := rank.NewFM(rank.FMTask(m.cfg.Rank.Task), nil)
	fmModel.Fit(trainSet, testSet, nil)

	m.fmMutex.Lock()
	m.fmModel = fmModel
	m.fmVersion++
	m.fmMutex.Unlock()

	if err := m.cacheStore.SetString(cache.GlobalMeta, cache.FitFactorizationMachineTime, base.Now()); err != nil {
		log.Infof("master: failed to write meta (%v)", err)
	}
	if err := m.cacheStore.SetString(cache.GlobalMeta, cache.FactorizationMachineVersion, fmt.Sprintf("%x", m.fmVersion)); err != nil {
		log.Infof("master: failed to write meta (%v)", err)
	}
}

func (m *Master) FitMatrixFactorization(dataSet *cf.DataSet) {
	if m.cfg.Collaborative.NumCached > 0 {
		base.Logger().Info("fit matrix factorization", zap.Int("n_jobs", m.cfg.Master.Jobs))
		// training match model
		trainSet, testSet := dataSet.Split(m.cfg.Collaborative.NumTestUsers, 0)
		mfModel, err := cf.NewModel(m.cfg.Collaborative.CFModel, m.cfg.Collaborative.GetParams(m.meta))
		if err != nil {
			log.Infof("master: failed to fit matrix factorization (%v)", err)
		}
		mfModel.Fit(trainSet, testSet, m.cfg.Collaborative.GetFitConfig(m.cfg.Master.Jobs))

		// update match model
		m.mfMutex.Lock()
		m.mfModel = mfModel
		m.mfVersion++
		m.mfMutex.Unlock()

		if err = m.cacheStore.SetString(cache.GlobalMeta, cache.FitMatrixFactorizationTime, base.Now()); err != nil {
			log.Infof("master: failed to write meta (%v)", err)
		}
		if err = m.cacheStore.SetString(cache.GlobalMeta, cache.MatrixFactorizationVersion, fmt.Sprintf("%x", m.mfVersion)); err != nil {
			log.Infof("master: failed to write meta (%v)", err)
		}
	}
}

func (m *Master) IsStale(dateTimeField string, timeLimit int) bool {
	updateTimeText, err := m.cacheStore.GetString(cache.GlobalMeta, dateTimeField)
	if err != nil {
		if err.Error() == "redis: nil" {
			return true
		}
		log.Fatalf("master: failed to get timestamp (%v)", err)
	}
	updateTime, err := dateparse.ParseAny(updateTimeText)
	if err != nil {
		log.Error("master: ", err)
		return true
	}
	return time.Since(updateTime).Minutes() > float64(timeLimit)
}

// CollectPopItem updates popular items for the database.
func (m *Master) CollectPopItem(items []data.Item, feedback []data.Feedback) {
	if m.cfg.Popular.NumCache > 0 {
		log.Info("master: collect popular items")
		// create item mapping
		itemMap := make(map[string]data.Item)
		for _, item := range items {
			itemMap[item.ItemId] = item
		}
		// count feedback
		timeWindowLimit := time.Now().AddDate(0, 0, -m.cfg.Popular.TimeWindow)
		count := make(map[string]int)
		for _, fb := range feedback {
			if fb.Timestamp.After(timeWindowLimit) {
				count[fb.ItemId]++
			}
		}
		// collect pop items
		popItems := make(map[string]*base.TopKStringFilter)
		popItems[""] = base.NewTopKStringFilter(m.cfg.Popular.NumCache)
		for itemId, f := range count {
			popItems[""].Push(itemId, float32(f))
			item := itemMap[itemId]
			for _, label := range item.Labels {
				if _, exists := popItems[label]; !exists {
					popItems[label] = base.NewTopKStringFilter(m.cfg.Popular.NumCache)
				}
				popItems[label].Push(itemId, float32(f))
			}
		}
		// write back
		for label, topItems := range popItems {
			result, _ := topItems.PopAll()
			if err := m.cacheStore.SetList(cache.PopularItems, label, result); err != nil {
				log.Errorf("master: failed to cache popular items (%v)", err)
			}
		}
		if err := m.cacheStore.SetString(cache.GlobalMeta, cache.CollectPopularTime, base.Now()); err != nil {
			log.Errorf("master: failed to cache popular items (%v)", err)
		}
	}
}

// CollectLatest updates latest items.
func (m *Master) CollectLatest(items []data.Item) {
	if m.cfg.Latest.NumCache > 0 {
		log.Info("master: collect latest items")
		var err error
		latestItems := make(map[string]*base.TopKStringFilter)
		latestItems[""] = base.NewTopKStringFilter(m.cfg.Latest.NumCache)
		// find latest items
		for _, item := range items {
			latestItems[""].Push(item.ItemId, float32(item.Timestamp.Unix()))
			for _, label := range item.Labels {
				if _, exist := latestItems[label]; !exist {
					latestItems[label] = base.NewTopKStringFilter(m.cfg.Latest.NumCache)
				}
				latestItems[label].Push(item.ItemId, float32(item.Timestamp.Unix()))
			}
		}
		for label, topItems := range latestItems {
			result, _ := topItems.PopAll()
			if err = m.cacheStore.SetList(cache.LatestItems, label, result); err != nil {
				log.Errorf("master: failed to cache latest items (%v)", err)
			}
		}
		if err = m.cacheStore.SetString(cache.GlobalMeta, cache.CollectLatestTime, base.Now()); err != nil {
			log.Errorf("master: failed to cache latest items (%v)", err)
		}
	}
}

// CollectSimilar updates neighbors for the database.
func (m *Master) CollectSimilar(items []data.Item, dataset *cf.DataSet) {
	if m.cfg.Similar.NumCache > 0 {
		base.Logger().Info("collect similar items")
		// create progress tracker
		completed := make(chan []interface{}, 1000)
		go func() {
			completedCount := 0
			ticker := time.NewTicker(time.Second)
			for {
				select {
				case _, ok := <-completed:
					if !ok {
						return
					}
					completedCount++
				case <-ticker.C:
					log.Infof("master: collect similar items (%v/%v)", completedCount, dataset.ItemCount())
				}
			}
		}()
		if err := base.Parallel(dataset.ItemCount(), m.cfg.Master.Jobs, func(workerId, jobId int) error {
			users := dataset.ItemFeedback[jobId]
			// Collect candidates
			itemSet := base.NewSet()
			for _, u := range users {
				itemSet.Add(dataset.UserFeedback[u]...)
			}
			// Ranking
			nearItems := base.NewTopKFilter(m.cfg.Similar.NumCache)
			for j := range itemSet {
				if j != jobId {
					nearItems.Push(j, Dot(dataset.ItemFeedback[jobId], dataset.ItemFeedback[j]))
				}
			}
			elem, _ := nearItems.PopAll()
			recommends := make([]string, len(elem))
			for i := range recommends {
				recommends[i] = dataset.ItemIndex.ToName(elem[i])
			}
			if err := m.cacheStore.SetList(cache.SimilarItems, dataset.ItemIndex.ToName(jobId), recommends); err != nil {
				return err
			}
			completed <- nil
			return nil
		}); err != nil {
			log.Errorf("master: failed to cache similar items (%v)", err)
		}
		close(completed)
		if err := m.cacheStore.SetString(cache.GlobalMeta, cache.CollectSimilarTime, base.Now()); err != nil {
			log.Errorf("master: failed to cache similar items (%v)", err)
		}
	}
}

func Dot(a, b []int) float32 {
	interSet := base.NewSet(a...)
	intersect := float32(0.0)
	for _, i := range b {
		if interSet.Contain(i) {
			intersect++
		}
	}
	if intersect == 0 {
		return 0
	}
	return intersect // math32.Sqrt(float32(len(a))) / math32.Sqrt(float32(len(b)))
}
