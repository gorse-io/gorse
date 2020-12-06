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
	"github.com/hashicorp/memberlist"
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
		dataSet, err := model.LoadDataFromDatabase(l.db)
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
			time.Sleep(time.Minute * time.Duration(l.cfg.Leader.Watch))
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

		time.Sleep(time.Minute * time.Duration(l.cfg.Leader.Watch))
	}
}

func (l *Leader) Pull() {
	// pull dataset
	//
	// pull labels
	// pull users
	// pull items
	// pull feedback
}

func (l *Leader) Broadcast() {
	//
}

// UpdatePopItem updates popular items for the database.
func (w *Worker) UpdatePopItem() error {
	items, err := db.GetItems(0, 0)
	if err != nil {
		return err
	}
	pop := make([]float64, len(items))
	for i, item := range items {
		pop[i] = item.Popularity
	}
	results := TopLabledItems(items, pop, collectSize)
	for label, result := range results {
		if err = db.SetPop(label, result); err != nil {
			return err
		}
	}
	return nil
}

// RefreshLatest updates latest items.
func RefreshLatest(db *database.Database, collectSize int) error {
	// update latest items
	items, err := db.GetItems(0, 0)
	if err != nil {
		return err
	}
	timestamp := make([]float64, len(items))
	for i, item := range items {
		timestamp[i] = float64(item.Timestamp.Unix())
	}
	results := TopLabledItems(items, timestamp, collectSize)
	for label, result := range results {
		if err = db.SetLatest(label, result); err != nil {
			return err
		}
	}
	return nil
}
