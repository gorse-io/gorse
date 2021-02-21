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
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/ReneKroon/ttlcache/v2"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/match"
	"github.com/zhenghaoz/gorse/model/rank"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"net"
	"sync"
	"time"
)

const (
	ServerNode = "server"
	WorkerNode = "worker"
)

type Master struct {
	protocol.UnimplementedMasterServer

	// cluster meta cache
	ttlCache   *ttlcache.Cache
	nodesMap   map[string]string
	nodesMutex sync.Mutex

	// configuration
	cfg  *config.Config
	meta *toml.MetaData

	// database connection
	dataStore  data.Database
	cacheStore cache.Database

	// match model
	matchModel match.MatrixFactorization
	matchTerm  int
	matchMutex sync.Mutex

	// rank model
	rankModel rank.FactorizationMachine
	rankTerm  int
	rankMutex sync.Mutex
}

func NewMaster(cfg *config.Config, meta *toml.MetaData) *Master {
	l := &Master{
		nodesMap: make(map[string]string),
		cfg:      cfg,
		meta:     meta,
	}
	return l
}

func (m *Master) Serve() {

	// create cluster meta cache
	m.ttlCache = ttlcache.NewCache()
	m.ttlCache.SetExpirationCallback(m.NodeDown)
	m.ttlCache.SetNewItemCallback(m.NodeUp)
	if err := m.ttlCache.SetTTL(
		time.Duration(m.cfg.Common.ClusterMetaTimeout) * time.Second,
	); err != nil {
		log.Error("master:", err)
	}

	// connect data database
	var err error
	m.dataStore, err = data.Open(m.cfg.Database.DataStore)
	if err != nil {
		log.Fatalf("master: failed to connect data database (%v)", err)
	}

	// connect cache database
	m.cacheStore, err = cache.Open(m.cfg.Database.CacheStore)
	if err != nil {
		log.Fatalf("master: failed to connect cache database (%v)", err)
	}

	// start model fit loop
	//go m.FitLoop()

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

func (m *Master) GetCluster(context.Context, *protocol.Void) (*protocol.Cluster, error) {
	cluster := &protocol.Cluster{
		Workers: make([]string, 0),
		Servers: make([]string, 0),
	}
	// add master
	cluster.Master = fmt.Sprintf("%s:%d", m.cfg.Master.Host, m.cfg.Master.Port)
	// add servers/workers
	m.nodesMutex.Lock()
	defer m.nodesMutex.Unlock()
	for addr, nodeType := range m.nodesMap {
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

func (m *Master) NodeUp(key string, value interface{}) {
	nodeType := value.(string)
	log.Infof("master: %s (%s) up", nodeType, key)
	m.nodesMutex.Lock()
	defer m.nodesMutex.Unlock()
	m.nodesMap[key] = nodeType
}

func (m *Master) NodeDown(key string, value interface{}) {
	nodeType := value.(string)
	log.Infof("master: %s (%s) down", nodeType, key)
	m.nodesMutex.Lock()
	defer m.nodesMutex.Unlock()
	delete(m.nodesMap, key)
}

func (m *Master) FitLoop() {
	//for {

	// download dataset
	log.Infof("master: load data from database")
	dataSet, _, err := match.LoadDataFromDatabase(m.dataStore, m.cfg.Common.MatchFeedbackType)
	if err != nil {
		log.Fatal("master: ", err)
	}
	log.Infof("master: data loaded (#user = %v, #item = %v, #feedback = %v)",
		dataSet.UserCount(), dataSet.ItemCount(), dataSet.Count())
	//		dataHash, err := dataSet.Hash()
	//		if err != nil {
	//			log.Error("Master:", err)
	//			time.Sleep(time.Minute * time.Duration(l.cfg.Common.RetryInterval))
	//			continue
	//		}
	//		l.dataHashMutex.Lock()
	//		if dataHash == l.dataHash && l.model != nil {
	//			l.dataHashMutex.Unlock()
	//			log.Infof("Master: dataset has no changes, next round starts after %v minutes", l.cfg.Master.FitInterval)
	//			time.Sleep(time.Minute * time.Duration(l.cfg.Master.FitInterval))
	//			continue
	//		}
	//		l.dataHash = dataHash
	//		l.dataHashMutex.Unlock()
	//
	// find popular items
	//log.Info("Master: update popular items")
	//if err := m.SetPopItem(items, dataSet); err != nil {
	//	log.Error("Master:", err)
	//	time.Sleep(time.Minute * time.Duration(l.cfg.Common.RetryInterval))
	//	continue
	//}

	// find latest items
	//log.Info("master: collect latest items")
	//if err := l.SetLatest(items); err != nil {
	//
	//}

	//		// find similar items
	//		//log.Info("Master: update similar items")
	//		//if err := l.SetNeighbors(items, dataSet); err != nil {
	//		//	log.Error("Master:", err)
	//		//	time.Sleep(time.Minute * time.Duration(l.cfg.Master.RetryInterval))
	//		//	continue
	//		//}

	// training match model
	log.Info("master: start to fit match model")
	trainSet, testSet := dataSet.Split(m.cfg.Master.Fit.NumTestUsers, 0)
	nextModel, err := match.NewModel(m.cfg.Master.Model, model.NewParamsFromConfig(m.cfg, m.meta))
	if err != nil {
		log.Fatal("master: ", err)
	}
	nextModel.Fit(trainSet, testSet, &m.cfg.Master.Fit)

	// update match model
	m.matchMutex.Lock()
	m.matchModel = nextModel
	m.matchTerm++
	m.matchMutex.Unlock()
}

//
//func (l *Master) Serve() {
//	log.Infof("Master: start gossip at %v:%v", l.cfg.Master.Host, l.cfg.Master.Port)
//
//	// start gossip
//	cfg := memberlist.DefaultLocalConfig()
//	cfg.BindAddr = l.cfg.Master.Host
//	cfg.BindPort = l.cfg.Master.Port
//	cfg.Name = protocol.NewName(protocol.LeaderNodePrefix, l.cfg.Master.Port, cfg.Name)
//	var err error
//	l.members, err = memberlist.Create(cfg)
//	if err != nil {
//		log.Fatal("Master:", err)
//	}
//
//	// start broadcast goroutine
//	go l.Broadcast()
//

//}
//
//// SetPopItem updates popular items for the database.
//func (l *Master) SetPopItem(items []storage.Item, dataset *match.DataSet) error {
//	// create item map
//	itemMap := make(map[string]storage.Item)
//	for _, item := range items {
//		itemMap[item.ItemId] = item
//	}
//	// find pop items
//	count := make([]int, dataset.ItemCount())
//	for _, userIndex := range dataset.FeedbackItems {
//		count[userIndex]++
//	}
//	popItems := make(map[string]*base.TopKStringFilter)
//	popItems[""] = base.NewTopKStringFilter(l.cfg.Common.CacheSize)
//	for itemIndex := range count {
//		itemId := dataset.ItemIndex.ToName(itemIndex)
//		item := itemMap[itemId]
//		popItems[""].Push(itemId, float32(count[itemIndex]))
//		for _, label := range item.Labels {
//			if _, exist := popItems[label]; !exist {
//				popItems[label] = base.NewTopKStringFilter(l.cfg.Common.CacheSize)
//			}
//			popItems[label].Push(item.ItemId, float32(count[itemIndex]))
//		}
//	}
//	result := make(map[string][]storage.RecommendedItem)
//	for label := range popItems {
//		elem, scores := popItems[label].PopAll()
//		items := make([]storage.RecommendedItem, len(elem))
//		for i := range items {
//			items[i].ItemId = elem[i]
//			items[i].Score = float64(scores[i])
//		}
//		result[label] = items
//	}
//	// write back
//	for label, items := range result {
//		if err := l.db.SetPop(label, items); err != nil {
//			return err
//		}
//	}
//	return nil
//}

// SetLatest updates latest items.
//func (l *Master) SetLatest(items []data.Item) error {
//	// find latest items
//	latestItems := make(map[string]*base.TopKStringFilter)
//	latestItems[""] = base.NewTopKStringFilter(l.cfg.Common.CacheSize)
//	for _, item := range items {
//		latestItems[""].Push(item.ItemId, float32(item.Timestamp.Unix()))
//		for _, label := range item.Labels {
//			if _, exist := latestItems[label]; !exist {
//				latestItems[label] = base.NewTopKStringFilter(l.cfg.Common.CacheSize)
//			}
//			latestItems[label].Push(item.ItemId, float32(item.Timestamp.Unix()))
//		}
//	}
//	result := make(map[string][]storage.RecommendedItem)
//	for label := range latestItems {
//		elem, scores := latestItems[label].PopAll()
//		items := make([]storage.RecommendedItem, len(elem))
//		for i := range items {
//			items[i].ItemId = elem[i]
//			items[i].Score = float64(scores[i])
//		}
//		result[label] = items
//	}
//	// write back
//	for label, items := range result {
//		if err := l.db.SetLatest(label, items); err != nil {
//			return err
//		}
//	}
//	return nil
//}

//// SetNeighbors updates neighbors for the database.
//func (l *Master) SetNeighbors(items []storage.Item, dataset *match.DataSet) error {
//	// create item map
//	itemMap := make(map[string]storage.Item)
//	for _, item := range items {
//		itemMap[item.ItemId] = item
//	}
//	for i, users := range dataset.ItemFeedback {
//		// Collect candidates
//		itemSet := base.NewSet(nil)
//		for _, u := range users {
//			itemSet.Add(dataset.UserFeedback[u]...)
//		}
//		// Ranking
//		nearItems := base.NewTopKFilter(l.cfg.Common.CacheSize)
//		for j := range itemSet {
//			if j != i {
//				nearItems.Push(j, Cosine(dataset.ItemFeedback[i], dataset.ItemFeedback[j]))
//			}
//		}
//		elem, scores := nearItems.PopAll()
//		recommends := make([]storage.RecommendedItem, len(elem))
//		for i := range recommends {
//			recommends[i] = storage.RecommendedItem{ItemId: dataset.ItemIndex.ToName(elem[i]), Score: float64(scores[i])}
//		}
//		if err := l.db.SetNeighbors(dataset.ItemIndex.ToName(i), recommends); err != nil {
//			return err
//		}
//	}
//	return nil
//}
//
//func Cosine(a, b []int) float32 {
//	interSet := base.NewSet(a)
//	intersect := float32(0.0)
//	for _, i := range b {
//		if interSet.Contain(i) {
//			intersect++
//		}
//	}
//	if intersect == 0 {
//		return 0
//	}
//	return intersect / math32.Sqrt(float32(len(a))) / math32.Sqrt(float32(len(b)))
//}
