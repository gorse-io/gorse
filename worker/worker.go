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
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/araddon/dateparse"
	"go.uber.org/zap"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/grpc"
)

type Worker struct {
	cfg        *config.Config
	cacheStore cache.Database
	dataStore  data.Database

	// master
	MasterHost   string
	MasterPort   int
	MasterClient protocol.MasterClient

	// user index
	userVersion int64
	userIndex   *base.MapIndex

	// collaborative filtering model
	Jobs      int
	cfVersion int64
	cfModel   cf.MatrixFactorization

	// channels
	ticker     *time.Ticker
	syncedChan chan bool
}

func NewWorker(masterHost string, masterPort int, jobs int) *Worker {
	return &Worker{
		MasterPort: masterPort,
		MasterHost: masterHost,
		Jobs:       jobs,
		ticker:     time.NewTicker(time.Minute),
		syncedChan: make(chan bool, 1024),
	}
}

// Register this worker to the master.
func (w *Worker) Register() {
	defer base.CheckPanic()
	for {
		if _, err := w.MasterClient.RegisterWorker(context.Background(), &protocol.Void{}); err != nil {
			base.Logger().Error("failed to register", zap.Error(err))
		}
		time.Sleep(time.Duration(w.cfg.Master.ClusterMetaTimeout/2) * time.Second)
	}
}

// Sync user index and collaborative filtering model from master.
func (w *Worker) Sync() {
	defer base.CheckPanic()
	for {
		synced := false

		// pull user index
		base.Logger().Debug("check user index version")
		if userIndexVersionResponse, err := w.MasterClient.GetUserIndexVersion(context.Background(), &protocol.Void{}); err != nil {
			base.Logger().Error("failed to check user index version", zap.Error(err))
		} else if userIndexVersionResponse.Version == 0 {
			base.Logger().Debug("user index doesn't exist")
		} else if userIndexVersionResponse.Version != w.userVersion {
			if userIndexResponse, err := w.MasterClient.GetUserIndex(context.Background(), &protocol.Void{}); err != nil {
				base.Logger().Error("failed to pull user index", zap.Error(err))
			} else {
				// encode user index
				var userIndex base.MapIndex
				reader := bytes.NewReader(userIndexResponse.Users)
				decoder := gob.NewDecoder(reader)
				if err = decoder.Decode(&userIndex); err != nil {
					base.Logger().Error("failed to decode user index", zap.Error(err))
				} else {
					w.userIndex = &userIndex
					w.userVersion = userIndexResponse.Version
					base.Logger().Info("sync user index", zap.Int64("version", w.userVersion))
					synced = true
				}
			}
		}

		// pull collaborative filtering model
		base.Logger().Debug("check collaborative filtering model version")
		if mfVersionResponse, err := w.MasterClient.GetCollaborativeFilteringModelVersion(context.Background(), &protocol.Void{}); err != nil {
			base.Logger().Error("failed to check collaborative filtering model version", zap.Error(err))
		} else if mfVersionResponse.Version == 0 {
			base.Logger().Debug("remote collaborative filtering model doesn't exist")
		} else if mfVersionResponse.Version != w.cfVersion {
			if mfResponse, err := w.MasterClient.GetCollaborativeFilteringModel(context.Background(), &protocol.Void{}, grpc.MaxCallRecvMsgSize(10e9)); err != nil {
				base.Logger().Error("failed to pull collaborative filtering model", zap.Error(err))
			} else {
				w.cfModel, err = cf.DecodeModel(mfResponse.Name, mfResponse.Model)
				if err != nil {
					base.Logger().Error("failed to decode collaborative filtering model", zap.Error(err))
				} else {
					w.cfVersion = mfResponse.Version
					base.Logger().Info("sync collaborative filtering model", zap.Int64("version", w.cfVersion))
					synced = true
				}
			}
		}

		if synced {
			w.syncedChan <- true
		}
		time.Sleep(time.Minute)
	}
}

func (w *Worker) Serve() {

	// connect to master
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", w.MasterHost, w.MasterPort), grpc.WithInsecure())
	if err != nil {
		base.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	w.MasterClient = protocol.NewMasterClient(conn)

	// load master config
	masterCfgJson, err := w.MasterClient.GetConfig(context.Background(), &protocol.Void{})
	if err != nil {
		base.Logger().Fatal("failed to load config from master", zap.Error(err))
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
	// sync model
	go w.Sync()

	loop := func() {
		if w.userIndex == nil {
			base.Logger().Debug("user index doesn't exist")
		} else {
			// split users
			cluster, err := w.MasterClient.GetCluster(context.Background(), &protocol.Void{})
			if err != nil {
				base.Logger().Error("failed to get cluster info", zap.Error(err))
				return
			}
			workingUsers := split(w.cfModel.GetUserIndex(), cluster.Workers, cluster.Me)

			// offline recommendation
			if !w.isStale(cache.CollaborativeRecommendTime, w.cfg.Collaborative.PredictPeriod) {
				base.Logger().Debug("collaborative filtering recommendations are up-to-date")
			} else if w.cfModel != nil {
				w.CollaborativeFilteringRecommend(w.cfModel, workingUsers)
			} else {
				base.Logger().Debug("local collaborative filtering model doesn't exist")
			}
		}
	}

	for {
		select {
		case <-w.ticker.C:
			loop()
		case <-w.syncedChan:
			loop()
		}
	}
}

func (w *Worker) CollaborativeFilteringRecommend(m cf.MatrixFactorization, users []string) {
	// get items
	items := m.GetItemIndex().GetNames()
	base.Logger().Info("collaborative filtering recommendation",
		zap.Int("n_working_users", len(users)),
		zap.Int("n_items", len(items)),
		zap.Int("n_jobs", w.Jobs),
		zap.Int("n_cache", w.cfg.Collaborative.NumCached))
	// progress tracker
	completed := make(chan interface{})
	go func() {
		defer base.CheckPanic()
		completedCount := 0
		ticker := time.NewTicker(time.Second * 5)
		for {
			select {
			case _, ok := <-completed:
				if !ok {
					return
				}
				completedCount++
			case <-ticker.C:
				base.Logger().Info("collaborative filtering recommendation",
					zap.Int("n_complete_users", completedCount),
					zap.Int("n_working_users", len(users)))
			}
		}
	}()
	// collaborative filtering recommendation
	_ = base.Parallel(len(users), w.Jobs, func(workerId, jobId int) error {
		user := users[jobId]
		// remove saw items
		historyFeedback, err := w.dataStore.GetUserFeedback(user, nil)
		if err != nil {
			base.Logger().Error("failed to pull user feedback",
				zap.String("user_id", user), zap.Error(err))
			return err
		}
		historySet := base.NewStringSet()
		for _, feedback := range historyFeedback {
			historySet.Add(feedback.ItemId)
		}
		recItems := base.NewTopKStringFilter(w.cfg.Similar.NumCache)
		for _, item := range items {
			if !historySet.Contain(item) {
				recItems.Push(item, m.Predict(user, item))
			}
		}
		elems, _ := recItems.PopAll()
		if err = w.cacheStore.SetList(cache.CollaborativeItems, user, elems); err != nil {
			base.Logger().Error("failed to cache collaborative filtering recommendation", zap.Error(err))
			return err
		}
		if err = w.cacheStore.SetString(cache.GlobalMeta, cache.CollaborativeRecommendTime, base.Now()); err != nil {
			base.Logger().Error("failed to cache collaborative filtering recommendation time", zap.Error(err))
		}
		completed <- nil
		return nil
	})
	close(completed)
}

//
//func (w *Worker) Subscribe(users []string) {
//	base.Logger().Info("subscribe",
//		zap.Bool("implicit_subscribe", w.cfg.Subscribe.ImplicitSubscribe))
//	completed := make(chan interface{})
//	go func() {
//		defer base.CheckPanic()
//		completedCount := 0
//		ticker := time.NewTicker(time.Second * 5)
//		for {
//			select {
//			case _, ok := <-completed:
//				if !ok {
//					return
//				}
//				completedCount++
//			case <-ticker.C:
//				base.Logger().Info("subscribe",
//					zap.Int("n_complete_users", completedCount),
//					zap.Int("n_working_users", len(users)))
//			}
//		}
//	}()
//	_ = base.Parallel(len(users), w.Jobs, func(workerId, jobId int) error {
//		user := users[jobId]
//		// collect items
//		historySet := base.NewStringSet()
//		for _, feedbackType := range w.cfg.Database.MatchFeedbackType {
//			historyFeedback, err := w.dataStore.GetUserFeedback(user, &feedbackType)
//			if err != nil {
//				base.Logger().Error("failed to pull user feedback",
//					zap.String("user_id", user), zap.Error(err))
//				return err
//			}
//			for _, feedback := range historyFeedback {
//				historySet.Add(feedback.ItemId)
//			}
//		}
//		// collect labels
//		labelSet := make(map[string]int)
//		for itemId, _ := range historySet {
//			if item, err := w.dataStore.GetItem(itemId); err != nil {
//				base.Logger().Error("failed to get item", zap.String("item_id", itemId), zap.Error(err))
//			} else {
//				for _, label := range item.Labels {
//					labelSet[label] ++
//				}
//			}
//		}
//		base.Logger().Info("items", zap.Any("items", labelSet))
//		completed <- nil
//		return nil
//	})
//	close(completed)
//}

func (w *Worker) isStale(dateTimeField string, minuteLimit int) bool {
	updateTimeText, err := w.cacheStore.GetString(cache.GlobalMeta, dateTimeField)
	if err != nil {
		if err.Error() == "redis: nil" {
			return true
		}
		base.Logger().Error("failed to read timestamp", zap.Error(err))
	}
	updateTime, err := dateparse.ParseAny(updateTimeText)
	if err != nil {
		base.Logger().Error("failed to parse date", zap.Error(err))
		return true
	}
	return time.Since(updateTime).Minutes() > float64(minuteLimit)
}

func split(userIndex base.Index, nodes []string, me string) []string {
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
	base.Logger().Info("allocate working users",
		zap.Int("n_working_users", len(workingUsers)),
		zap.Int("n_users", len(users)))
	return workingUsers
}
