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
package leader

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/chewxy/math32"
	"github.com/hashicorp/memberlist"
	"github.com/jinzhu/copier"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/match"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage"
	"google.golang.org/grpc"
	"math/rand"
	"net"
	"sort"
	"sync"
	"time"
)

type Leader struct {
	db   storage.Database
	cfg  *config.Config
	meta *toml.MetaData

	// broadcast
	members          *memberlist.Memberlist
	startBroadcast   chan bool
	broadcastStarted bool

	// recommender
	model        model.Model
	modelVersion *protocol.Version
	modelMutex   sync.Mutex

	// hash value for dataset
	dataHash      uint32
	dataHashMutex sync.Mutex

	// user partition
	usersSplitter *Splitter
	userVersion   *protocol.Version
	userMutex     sync.Mutex
}

func NewLeader(db storage.Database, cfg *config.Config, meta *toml.MetaData) *Leader {
	l := &Leader{
		db:               db,
		cfg:              cfg,
		meta:             meta,
		startBroadcast:   make(chan bool),
		broadcastStarted: false,
		// versions
		modelVersion: &protocol.Version{Tag: rand.Int31()},
		userVersion:  &protocol.Version{Tag: rand.Int31()},
	}
	return l
}

func (l *Leader) Serve() {
	log.Infof("Leader: start gossip at %v:%v", l.cfg.Leader.Host, l.cfg.Leader.GossipPort)

	// start gossip
	cfg := memberlist.DefaultLocalConfig()
	cfg.BindAddr = l.cfg.Leader.Host
	cfg.BindPort = l.cfg.Leader.GossipPort
	cfg.Name = protocol.NewName(protocol.LeaderNodePrefix, l.cfg.Leader.GossipPort, cfg.Name)
	var err error
	l.members, err = memberlist.Create(cfg)
	if err != nil {
		log.Fatal("Leader:", err)
	}

	// start broadcast goroutine
	go l.Broadcast()

	// main loop
	for {

		// download dataset
		log.Infof("Leader: load data from database")
		dataSet, items, err := match.LoadDataFromDatabase(l.db)
		if err != nil {
			log.Error("Leader:", err)
			time.Sleep(time.Minute * time.Duration(l.cfg.Common.RetryInterval))
			continue
		}
		dataHash, err := dataSet.Hash()
		if err != nil {
			log.Error("Leader:", err)
			time.Sleep(time.Minute * time.Duration(l.cfg.Common.RetryInterval))
			continue
		}
		l.dataHashMutex.Lock()
		if dataHash == l.dataHash && l.model != nil {
			l.dataHashMutex.Unlock()
			log.Infof("Leader: dataset has no changes, next round starts after %v minutes", l.cfg.Leader.FitInterval)
			time.Sleep(time.Minute * time.Duration(l.cfg.Leader.FitInterval))
			continue
		}
		l.dataHash = dataHash
		l.dataHashMutex.Unlock()

		// find popular items
		log.Info("Leader: update popular items")
		if err := l.SetPopItem(items, dataSet); err != nil {
			log.Error("Leader:", err)
			time.Sleep(time.Minute * time.Duration(l.cfg.Common.RetryInterval))
			continue
		}

		// find latest items
		//log.Info("Leader: update latest items")
		//if err := l.SetLatest(items); err != nil {
		//	log.Error("Leader:", err)
		//	time.Sleep(time.Minute * time.Duration(l.cfg.Leader.RetryInterval))
		//	continue
		//}

		// find similar items
		//log.Info("Leader: update similar items")
		//if err := l.SetNeighbors(items, dataSet); err != nil {
		//	log.Error("Leader:", err)
		//	time.Sleep(time.Minute * time.Duration(l.cfg.Leader.RetryInterval))
		//	continue
		//}

		// training model
		log.Info("Leader: start to fit model")
		trainSet, testSet := dataSet.Split(l.cfg.Leader.Fit.NumTestUsers, 0)
		nextModel, err := model.NewModel(l.cfg.Leader.Model, model.NewParamsFromConfig(l.cfg, l.meta))
		if err != nil {
			log.Fatal("Leader: ", err)
		}
		nextModel.Fit(trainSet, testSet, &l.cfg.Leader.Fit)

		// update model
		l.modelMutex.Lock()
		l.model = nextModel
		l.modelVersion.Term++
		l.modelMutex.Unlock()

		// start broadcast
		if !l.broadcastStarted {
			l.broadcastStarted = true
			l.startBroadcast <- true
		}

		log.Infof("Leader: complete fit, next round starts after %v minutes", l.cfg.Leader.FitInterval)
		time.Sleep(time.Minute * time.Duration(l.cfg.Leader.FitInterval))
	}
}

func (l *Leader) Broadcast() {
	_ = <-l.startBroadcast
	log.Info("Leader: start to broadcast")
	for {

		// check members
		localNode := l.members.LocalNode()
		nodes := make([]string, 0)
		addresses := make([]net.IP, 0)
		for _, member := range l.members.Members() {
			if member.Name != localNode.Name {
				nodes = append(nodes, member.Name)
				addresses = append(addresses, member.Addr)
			}
		}
		if len(nodes) == 0 {
			log.Error("Leader: no workers found")
			time.Sleep(time.Minute * time.Duration(l.cfg.Common.RetryInterval))
			continue
		}

		// broadcast users
		l.userMutex.Lock()
		userSplitter := NewSplitter()
		err := copier.Copy(&userSplitter, l.usersSplitter)
		if err != nil {
			l.userMutex.Unlock()
			log.Error(err)
			time.Sleep(time.Second * time.Duration(l.cfg.Common.RetryInterval))
			continue
		}
		l.userMutex.Unlock()
		log.Infof("Leader: split users to %v partitions", len(nodes))
		userParts := userSplitter.Split(len(nodes))

		sort.Strings(nodes)
		for i, node := range nodes {
			meta, err := protocol.ParseName(node)
			if err != nil {
				log.Error(err)
				time.Sleep(time.Second * time.Duration(l.cfg.Common.RetryInterval))
				continue
			}
			// create connection
			conn, err := grpc.Dial(fmt.Sprintf("%v:%v", addresses[i], meta.Port), grpc.WithInsecure())
			if err != nil {
				log.Error(err)
				time.Sleep(time.Second * time.Duration(l.cfg.Common.RetryInterval))
				continue
			}
			defer conn.Close()
			client := protocol.NewWorkerClient(conn)

			// request model version
			ctx := context.Background()
			log.Infof("Leader: request model version from %v", node)
			modelVersion, err := client.GetModelVersion(ctx, &protocol.Void{})
			if err != nil {
				log.Error("Leader: ", err)
				time.Sleep(time.Second * time.Duration(l.cfg.Common.RetryInterval))
				continue
			}
			// send model partition
			if modelVersion.Tag != l.modelVersion.Tag || modelVersion.Term != l.modelVersion.Term {
				log.Infof("Leader: send model to %v", node)
				var buf bytes.Buffer
				writer := bufio.NewWriter(&buf)
				encoder := gob.NewEncoder(writer)
				l.modelMutex.Lock()
				if err = encoder.Encode(l.model); err != nil {
					l.modelMutex.Unlock()
					log.Error(err)
					time.Sleep(time.Second * time.Duration(l.cfg.Common.RetryInterval))
					continue
				}
				l.modelMutex.Unlock()
				_, err = client.BroadcastModel(ctx, &protocol.Model{
					Model:   buf.Bytes(),
					Name:    l.cfg.Leader.Model,
					Version: l.modelVersion,
				})
				if err != nil {
					log.Error(err)
					time.Sleep(time.Second * time.Duration(l.cfg.Common.RetryInterval))
					continue
				}
			}

			// request user version
			//userVersion, err := client.GetUserPartitionVersion(ctx, &protocol.Void{})
			//if err != nil {
			//	log.Error(err)
			//	time.Sleep(time.Second * time.Duration(l.cfg.Leader.RetryInterval))
			//	continue
			//}
			// send model
			//if userVersion.Tag != l.userVersion.Tag || userVersion.Term != l.userVersion.Term {
			log.Infof("Leader: send user partition to %v", node)
			_, err = client.BroadcastUserPartition(ctx, &protocol.Partition{
				Version:  l.userVersion,
				Prefixes: userParts[i],
			})
			if err != nil {
				log.Error(err)
				time.Sleep(time.Second * time.Duration(l.cfg.Common.RetryInterval))
				continue
			}
			//}
		}

		log.Infof("Leader: complete broadcast, next round starts after %v minute", l.cfg.Leader.BroadcastInterval)
		time.Sleep(time.Minute * time.Duration(l.cfg.Leader.BroadcastInterval))
	}
}

// SetPopItem updates popular items for the database.
func (l *Leader) SetPopItem(items []storage.Item, dataset *match.DataSet) error {
	// create item map
	itemMap := make(map[string]storage.Item)
	for _, item := range items {
		itemMap[item.ItemId] = item
	}
	// find pop items
	count := make([]int, dataset.ItemCount())
	for _, userIndex := range dataset.FeedbackItems {
		count[userIndex]++
	}
	popItems := make(map[string]*base.TopKStringFilter)
	popItems[""] = base.NewTopKStringFilter(l.cfg.Common.CacheSize)
	for itemIndex := range count {
		itemId := dataset.ItemIndex.ToName(itemIndex)
		item := itemMap[itemId]
		popItems[""].Push(itemId, float32(count[itemIndex]))
		for _, label := range item.Labels {
			if _, exist := popItems[label]; !exist {
				popItems[label] = base.NewTopKStringFilter(l.cfg.Common.CacheSize)
			}
			popItems[label].Push(item.ItemId, float32(count[itemIndex]))
		}
	}
	result := make(map[string][]storage.RecommendedItem)
	for label := range popItems {
		elem, scores := popItems[label].PopAll()
		items := make([]storage.RecommendedItem, len(elem))
		for i := range items {
			items[i].ItemId = elem[i]
			items[i].Score = float64(scores[i])
		}
		result[label] = items
	}
	// write back
	for label, items := range result {
		if err := l.db.SetPop(label, items); err != nil {
			return err
		}
	}
	return nil
}

// SetLatest updates latest items.
func (l *Leader) SetLatest(items []storage.Item) error {
	// find latest items
	latestItems := make(map[string]*base.TopKStringFilter)
	latestItems[""] = base.NewTopKStringFilter(l.cfg.Common.CacheSize)
	for _, item := range items {
		latestItems[""].Push(item.ItemId, float32(item.Timestamp.Unix()))
		for _, label := range item.Labels {
			if _, exist := latestItems[label]; !exist {
				latestItems[label] = base.NewTopKStringFilter(l.cfg.Common.CacheSize)
			}
			latestItems[label].Push(item.ItemId, float32(item.Timestamp.Unix()))
		}
	}
	result := make(map[string][]storage.RecommendedItem)
	for label := range latestItems {
		elem, scores := latestItems[label].PopAll()
		items := make([]storage.RecommendedItem, len(elem))
		for i := range items {
			items[i].ItemId = elem[i]
			items[i].Score = float64(scores[i])
		}
		result[label] = items
	}
	// write back
	for label, items := range result {
		if err := l.db.SetLatest(label, items); err != nil {
			return err
		}
	}
	return nil
}

// SetNeighbors updates neighbors for the database.
func (l *Leader) SetNeighbors(items []storage.Item, dataset *match.DataSet) error {
	// create item map
	itemMap := make(map[string]storage.Item)
	for _, item := range items {
		itemMap[item.ItemId] = item
	}
	for i, users := range dataset.ItemFeedback {
		// Collect candidates
		itemSet := base.NewSet(nil)
		for _, u := range users {
			itemSet.Add(dataset.UserFeedback[u]...)
		}
		// Ranking
		nearItems := base.NewTopKFilter(l.cfg.Common.CacheSize)
		for j := range itemSet {
			if j != i {
				nearItems.Push(j, Cosine(dataset.ItemFeedback[i], dataset.ItemFeedback[j]))
			}
		}
		elem, scores := nearItems.PopAll()
		recommends := make([]storage.RecommendedItem, len(elem))
		for i := range recommends {
			recommends[i] = storage.RecommendedItem{ItemId: dataset.ItemIndex.ToName(elem[i]), Score: float64(scores[i])}
		}
		if err := l.db.SetNeighbors(dataset.ItemIndex.ToName(i), recommends); err != nil {
			return err
		}
	}
	return nil
}

func Cosine(a, b []int) float32 {
	interSet := base.NewSet(a)
	intersect := float32(0.0)
	for _, i := range b {
		if interSet.Contain(i) {
			intersect++
		}
	}
	if intersect == 0 {
		return 0
	}
	return intersect / math32.Sqrt(float32(len(a))) / math32.Sqrt(float32(len(b)))
}
