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
	"github.com/hashicorp/memberlist"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/match"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/grpc"
	"time"
)

type Worker struct {
	cfg        *config.Config
	members    *memberlist.Memberlist
	cacheStore cache.Database
	dataStore  data.Database

	// master
	MasterHost   string
	MasterPort   int
	MasterClient protocol.MasterClient

	// match model
	Jobs              int
	MatchModelVersion int64
	MatchModel        match.MatrixFactorization
}

func NewWorker(masterHost string, masterPort int, jobs int) *Worker {
	return &Worker{
		MasterPort: masterPort,
		MasterHost: masterHost,
		Jobs:       jobs,
	}
}

func (w *Worker) Register() {
	for {
		if _, err := w.MasterClient.RegisterWorker(context.Background(), &protocol.Void{}); err != nil {
			log.Fatal("worker:", err)
		}
		time.Sleep(time.Duration(w.cfg.Common.ClusterMetaTimeout/2) * time.Second)
	}
}

func (w *Worker) Serve() {

	// connect to master
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", w.MasterHost, w.MasterPort), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("worker: failed to connect master (%v)", err)
	}
	w.MasterClient = protocol.NewMasterClient(conn)

	// load master config
	masterCfgJson, err := w.MasterClient.GetConfig(context.Background(), &protocol.Void{})
	if err != nil {
		log.Fatalf("worker: failed to load master config (%v)", err)
	}
	err = json.Unmarshal([]byte(masterCfgJson.Json), &w.cfg)
	if err != nil {
		log.Fatalf("worker: failed to parse master config (%v)", err)
	}

	// connect to data store
	if w.dataStore, err = data.Open(w.cfg.Database.DataStore); err != nil {
		log.Fatalf("worker: failed to connect data store (%v)", err)
	}

	// connect to cache store
	if w.cacheStore, err = cache.Open(w.cfg.Database.CacheStore); err != nil {
		log.Fatalf("worker: failed to connect cache store (%v)", err)
	}

	// register to master
	go w.Register()

	// pull model version
	log.Info("worker: pull model version from master")
	matchModel, err := w.MasterClient.GetMatchModelVersion(context.Background(), &protocol.Void{})
	if err != nil {
		log.Errorf("worker: failed to pull model version (%v)", err)
	}

	// pull model
	if matchModel.Version != w.MatchModelVersion {
		log.Infof("worker: found new model version (%x)", matchModel.Version)
		// pull model
		matchModel, err = w.MasterClient.GetMatchModel(context.Background(), &protocol.Void{},
			grpc.MaxCallRecvMsgSize(10e9))
		if err != nil {
			log.Errorf("worker: failed to pull model (%v)", err)
		}
	}

	// get cluster
	cluster, err := w.MasterClient.GetCluster(context.Background(), &protocol.Void{})
	if err != nil {
		log.Errorf("worker: failed to get cluster info (%v)", err)
	}

	w.MatchModel, err = match.DecodeModel(matchModel.Name, matchModel.Model)
	if err != nil {
		log.Errorf("worker: failed to decode model (%v)", err)
	}
	workingUsers := Split(w.MatchModel.GetUserIndex(), cluster.Workers, cluster.Me)
	w.GenerateMatchItems(w.MatchModel, workingUsers)

}

func (w *Worker) GenerateMatchItems(m match.MatrixFactorization, users []string) {
	// get items
	items := m.GetItemIndex().GetNames()
	log.Infof("worker: generate match items for %v users among %v items (n_jobs = %v)", len(users), len(items), w.Jobs)
	// progress tracker
	completed := make(chan interface{})
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
			case _ = <-ticker.C:
				log.Infof("worker: generate match items (%v/%v)", completedCount, len(users))
			}
		}
	}()
	// generate match items
	_ = base.Parallel(len(users), w.Jobs, func(workerId, jobId int) error {
		user := users[jobId]
		// remove saw items
		historyFeedback, err := w.dataStore.GetUserFeedback("", user)
		if err != nil {
			log.Fatalf("worker: failed to pull user feedback (%v)", err)
		}
		historySet := base.NewStringSet()
		for _, feedback := range historyFeedback {
			historySet.Add(feedback.ItemId)
		}
		recItems := base.NewTopKStringFilter(w.cfg.Common.CacheSize)
		for _, item := range items {
			if !historySet.Contain(item) {
				recItems.Push(item, m.Predict(user, item))
			}
		}
		elems, _ := recItems.PopAll()
		if err := w.cacheStore.SetList(cache.MatchedItems, user, elems); err != nil {
			log.Fatalf("worker: failed to push matched items (%v)", err)
		}
		completed <- nil
		return nil
	})
	close(completed)
}

func Split(userIndex base.Index, nodes []string, me string) []string {
	// locate me
	pos := -1
	for i, node := range nodes {
		if node == me {
			pos = i
		}
	}
	if pos == -1 {
		log.Fatalf("worker: who am I?")
	}
	// split users
	users := userIndex.GetNames()
	workingUsers := make([]string, 0)
	for ; pos < len(users); pos += len(nodes) {
		workingUsers = append(workingUsers, users[pos])
	}
	log.Infof("worker: allocate working users (%v/%v)", len(workingUsers), len(users))
	return workingUsers
}
