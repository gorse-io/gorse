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

	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/grpc"
)

type Worker struct {
	cfg  *config.Config
	Jobs int

	// database connection
	cacheAddress string
	cacheStore   cache.Database
	dataAddress  string
	dataStore    data.Database

	// master connection
	masterHost   string
	masterPort   int
	MasterClient protocol.MasterClient

	// user index
	latestUserVersion int64
	userVersion       int64
	userIndex         *base.MapIndex

	// collaborative filtering model
	latestCFVersion int64
	cfVersion       int64
	cfModel         cf.MatrixFactorization

	// peers
	peers []string
	me    string

	// events
	ticker     *time.Ticker
	syncedChan chan bool // meta synced events
	pullChan   chan bool // model pulled events
}

func NewWorker(masterHost string, masterPort int, jobs int) *Worker {
	return &Worker{
		// database
		dataStore:  data.NoDatabase{},
		cacheStore: cache.NoDatabase{},
		// config
		masterPort: masterPort,
		masterHost: masterHost,
		Jobs:       jobs,
		cfg:        (*config.Config)(nil).LoadDefaultIfNil(),
		// events
		ticker:     time.NewTicker(time.Minute),
		syncedChan: make(chan bool, 1024),
		pullChan:   make(chan bool, 1024),
	}
}

// Sync this worker to the master.
func (w *Worker) Sync() {
	defer base.CheckPanic()
	base.Logger().Info("start meta sync", zap.Int("meta_timeout", w.cfg.Master.MetaTimeout))
	for {
		var meta *protocol.Meta
		var err error
		if meta, err = w.MasterClient.GetMeta(context.Background(), &protocol.RequestInfo{NodeType: protocol.NodeType_WorkerNode}); err != nil {
			base.Logger().Error("failed to get meta", zap.Error(err))
			goto sleep
		}

		// load master config
		err = json.Unmarshal([]byte(meta.Config), &w.cfg)
		if err != nil {
			base.Logger().Error("failed to parse master config", zap.Error(err))
			goto sleep
		}

		// connect to data store
		if w.dataAddress != w.cfg.Database.DataStore {
			base.Logger().Info("connect data store", zap.String("database", w.cfg.Database.DataStore))
			if w.dataStore, err = data.Open(w.cfg.Database.DataStore); err != nil {
				base.Logger().Error("failed to connect data store", zap.Error(err))
				goto sleep
			}
			w.dataAddress = w.cfg.Database.DataStore
		}

		// connect to cache store
		if w.cacheAddress != w.cfg.Database.CacheStore {
			base.Logger().Info("connect cache store", zap.String("database", w.cfg.Database.CacheStore))
			if w.cacheStore, err = cache.Open(w.cfg.Database.CacheStore); err != nil {
				base.Logger().Error("failed to connect cache store", zap.Error(err))
				goto sleep
			}
			w.cacheAddress = w.cfg.Database.CacheStore
		}

		// check CF version
		w.latestCFVersion = meta.CfVersion
		if w.latestCFVersion != w.cfVersion {
			base.Logger().Info("new collaborative filtering model found",
				zap.Int64("old_version", w.cfVersion),
				zap.Int64("new_version", w.latestCFVersion))
			w.syncedChan <- true
		}

		// check user index version
		w.latestUserVersion = meta.UserIndexVersion
		if w.latestUserVersion != w.userVersion {
			base.Logger().Info("new user index found",
				zap.Int64("old_version", w.userVersion),
				zap.Int64("new_version", w.latestUserVersion))
			w.syncedChan <- true
		}

		w.peers = meta.Workers
		w.me = meta.Me
	sleep:
		time.Sleep(time.Duration(w.cfg.Master.MetaTimeout) * time.Second)
	}
}

// Pull user index and collaborative filtering model from master.
func (w *Worker) Pull() {
	defer base.CheckPanic()
	for range w.syncedChan {
		pulled := false

		// pull user index
		if w.latestUserVersion != w.userVersion {
			base.Logger().Info("start pull user index")
			if userIndexResponse, err := w.MasterClient.GetUserIndex(context.Background(),
				&protocol.RequestInfo{}, grpc.MaxCallRecvMsgSize(10e8)); err != nil {
				base.Logger().Error("failed to pull user index", zap.Error(err))
			} else {
				// encode user index
				var userIndex base.MapIndex
				reader := bytes.NewReader(userIndexResponse.UserIndex)
				decoder := gob.NewDecoder(reader)
				if err = decoder.Decode(&userIndex); err != nil {
					base.Logger().Error("failed to decode user index", zap.Error(err))
				} else {
					w.userIndex = &userIndex
					w.userVersion = userIndexResponse.Version
					base.Logger().Info("synced user index", zap.Int64("version", w.userVersion))
					pulled = true
				}
			}
		}

		// pull collaborative filtering model
		if w.latestCFVersion != w.cfVersion {
			base.Logger().Info("start pull collaborative filtering")
			if mfResponse, err := w.MasterClient.GetCollaborativeFilteringModel(context.Background(),
				&protocol.RequestInfo{}, grpc.MaxCallRecvMsgSize(10e8)); err != nil {
				base.Logger().Error("failed to pull collaborative filtering model", zap.Error(err))
			} else {
				w.cfModel, err = cf.DecodeModel(mfResponse.Name, mfResponse.Model)
				if err != nil {
					base.Logger().Error("failed to decode collaborative filtering model", zap.Error(err))
				} else {
					w.cfVersion = mfResponse.Version
					base.Logger().Info("synced collaborative filtering model", zap.Int64("version", w.cfVersion))
					pulled = true
				}
			}
		}

		if pulled {
			w.syncedChan <- true
		}
	}
}

func (w *Worker) Serve() {

	// connect to master
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", w.masterHost, w.masterPort), grpc.WithInsecure())
	if err != nil {
		base.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	w.MasterClient = protocol.NewMasterClient(conn)

	// sync
	go w.Sync()
	go w.Pull()

	loop := func() {
		if w.userIndex == nil {
			base.Logger().Debug("user index doesn't exist")
		} else {
			// split users
			workingUsers, err := split(w.cfModel.GetUserIndex(), w.peers, w.me)
			if err != nil {
				base.Logger().Error("failed to split users", zap.Error(err),
					zap.String("me", w.me),
					zap.Strings("workers", w.peers))
				return
			}

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
		ticker := time.NewTicker(time.Second)
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

func split(userIndex base.Index, nodes []string, me string) ([]string, error) {
	// locate me
	pos := -1
	for i, node := range nodes {
		if node == me {
			pos = i
		}
	}
	if pos == -1 {
		return nil, fmt.Errorf("current node isn't in worker nodes")
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
	return workingUsers, nil
}
