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
	"github.com/BurntSushi/toml"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/hashicorp/memberlist"
	"github.com/jinzhu/copier"
	"github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage"
	"google.golang.org/genproto/googleapis/spanner/admin/database/v1"
	"log"
	"sync"
	"time"
)

type Leader struct {
	db   storage.Database
	cfg  *config.Config
	meta *toml.MetaData

	workers      []protocol.WorkerClient
	workersMutex sync.Mutex

	members    *memberlist.Memberlist
	model      model.Model
	modelMutex sync.Mutex
	dataHash   uint32

	// user partition
	usersSplitter *Splitter
	userMutex     sync.Mutex

	// item partition
	itemSplitter *Splitter
	itemMutex    sync.Mutex
}

func (l *Leader) Serve() {

	// start gossip
	cfg := memberlist.DefaultLocalConfig()
	cfg.BindAddr = l.cfg.Leader.Host
	cfg.BindPort = l.cfg.Leader.Port
	cfg.Name = protocol.NewName(protocol.LeaderNodePrefix, l.cfg.Leader.Port, cfg.Name)
	var err error
	l.members, err = memberlist.Create(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// main loop
	for {

		// download dataset
		dataSet, items, err := model.LoadDataFromDatabase(l.db)
		if err != nil {
			log.Fatal(err)
		}
		dataHash, err := dataSet.Hash()
		if err != nil {
			log.Fatal(err)
		}
		l.modelMutex.Lock()
		if dataHash == l.dataHash && l.model == nil {
			l.modelMutex.Unlock()
			time.Sleep(time.Minute * time.Duration(l.cfg.Leader.FitPeriod))
			continue
		}

		// training model
		trainSet, testSet := dataSet.Split(l.cfg.Leader.Fit.NumTestUsers, 0)
		nextModel := model.NewModel(l.cfg.Leader.Model, model.NewParamsFromConfig(l.cfg, l.meta))
		nextModel.Fit(trainSet, testSet, &l.cfg.Leader.Fit)

		// update model
		l.modelMutex.Lock()
		l.model = nextModel
		l.modelMutex.Unlock()

		time.Sleep(time.Minute * time.Duration(l.cfg.Leader.FitPeriod))
	}
}

func (l *Leader) Broadcast() {
	for {

		// check members
		nodes := make([]protocol.Meta, 0)
		for _, member := range l.members.Members() {
			meta, err := protocol.ParseName(member.Name)
			if err != nil {
				logrus.Error(err)
				time.Sleep(time.Second * time.Duration(l.cfg.Leader.RetryPeriod))
				continue
			}
			nodes = append(nodes, meta)
		}
		if len(nodes) == 0 {
			logrus.Error("no workers found")
			time.Sleep(time.Second * time.Duration(l.cfg.Leader.RetryPeriod))
			continue
		}

		// broadcast users
		l.userMutex.Lock()
		var userSplitter Splitter
		err := copier.Copy(&userSplitter, l.usersSplitter)
		if err != nil {
			l.userMutex.Unlock()
			logrus.Error(err)
			time.Sleep(time.Second * time.Duration(l.cfg.Leader.RetryPeriod))
			continue
		}
		l.userMutex.Unlock()
		//userParts := userSplitter.Split(len(nodes))

		// broadcast items
		l.itemMutex.Lock()
		var itemSplitter Splitter
		err = copier.Copy(&itemSplitter, l.itemSplitter)
		if err != nil {
			l.itemMutex.Unlock()
			logrus.Error(err)
			time.Sleep(time.Second * time.Duration(l.cfg.Leader.RetryPeriod))
			continue
		}
		l.itemMutex.Unlock()

		time.Sleep(time.Minute * time.Duration(l.cfg.Leader.BroadcastPeriod))
	}
}

// SetPopItem updates popular items for the database.
func (l *Leader) SetPopItem(items []storage.Item, dataset *model.DataSet, collectSize int) {
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
	popItems := make(map[string]*base.MaxHeap)
	popItems[""] = base.NewMaxHeap(collectSize)
	for _, itemIndex := range count {
		itemId := dataset.ItemIndex.ToName(itemIndex)
		item := itemMap[itemId]
		popItems[""].Add(itemId, float32(item.Timestamp.Unix()))
		for _, label := range item.Labels {
			if _, exist := popItems[label]; !exist {
				popItems[label] = base.NewMaxHeap(collectSize)
			}
			popItems[label].Add(item.ItemId, float32(item.Timestamp.Unix()))
		}
	}
	result := make(map[string][]storage.RecommendedItem)
	for label := range popItems {
		elem, scores := popItems[label].ToSorted()
		items := make([]storage.RecommendedItem, len(elem))
		for i := range items {
			items[i].ItemId = elem[i].(string)
			items[i].Score = float64(scores[i])
		}
		result[label] = items
	}
	// write back
	for label, items := range result {
		if err := l.db.SetPop(label, items); err != nil {
			logrus.Error(err)
		}
	}
}

// SetLatest updates latest items.
func (l *Leader) SetLatest(items []storage.Item, collectSize int) {
	// find latest items
	latestItems := make(map[string]*base.MaxHeap)
	latestItems[""] = base.NewMaxHeap(collectSize)
	for _, item := range items {
		latestItems[""].Add(item.ItemId, float32(item.Timestamp.Unix()))
		for _, label := range item.Labels {
			if _, exist := latestItems[label]; !exist {
				latestItems[label] = base.NewMaxHeap(collectSize)
			}
			latestItems[label].Add(item.ItemId, float32(item.Timestamp.Unix()))
		}
	}
	result := make(map[string][]storage.RecommendedItem)
	for label := range latestItems {
		elem, scores := latestItems[label].ToSorted()
		items := make([]storage.RecommendedItem, len(elem))
		for i := range items {
			items[i].ItemId = elem[i].(string)
			items[i].Score = float64(scores[i])
		}
		result[label] = items
	}
	// write back
	for label, items := range result {
		if err := l.db.SetLatest(label, items); err != nil {
			logrus.Error(err)
		}
	}
}
