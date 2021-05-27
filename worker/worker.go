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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/scylladb/go-set"
	"go.uber.org/zap"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/pr"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/grpc"
)

type Worker struct {
	// worker config
	cfg        *config.Config
	Jobs       int
	workerName string
	httpHost   string
	httpPort   int
	masterHost string
	masterPort int

	// database connection
	cacheAddress string
	cacheStore   cache.Database
	dataAddress  string
	dataStore    data.Database

	// master connection
	MasterClient protocol.MasterClient

	// user index
	latestUserVersion int64
	userVersion       int64
	userIndex         *base.MapIndex

	// collaborative filtering model
	latestPRVersion int64
	prModelVersion  int64
	prModel         pr.Model

	// peers
	peers []string
	me    string

	// events
	ticker     *time.Ticker
	syncedChan chan bool // meta synced events
	pullChan   chan bool // model pulled events
}

func NewWorker(masterHost string, masterPort int, httpHost string, httpPort int, jobs int) *Worker {
	return &Worker{
		// database
		dataStore:  data.NoDatabase{},
		cacheStore: cache.NoDatabase{},
		// config
		masterHost: masterHost,
		masterPort: masterPort,
		httpHost:   httpHost,
		httpPort:   httpPort,
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
		if meta, err = w.MasterClient.GetMeta(context.Background(),
			&protocol.NodeInfo{
				NodeType: protocol.NodeType_WorkerNode,
				NodeName: w.workerName,
				HttpPort: int64(w.httpPort),
			}); err != nil {
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
		w.latestPRVersion = meta.PrVersion
		if w.latestPRVersion != w.prModelVersion {
			base.Logger().Info("new personal ranking model found",
				zap.String("old_version", base.Hex(w.prModelVersion)),
				zap.String("new_version", base.Hex(w.latestPRVersion)))
			w.syncedChan <- true
		}

		// check user index version
		w.latestUserVersion = meta.UserIndexVersion
		if w.latestUserVersion != w.userVersion {
			base.Logger().Info("new user index found",
				zap.String("old_version", base.Hex(w.userVersion)),
				zap.String("new_version", base.Hex(w.latestUserVersion)))
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
				&protocol.NodeInfo{NodeType: protocol.NodeType_WorkerNode, NodeName: w.workerName},
				grpc.MaxCallRecvMsgSize(10e8)); err != nil {
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
					base.Logger().Info("synced user index",
						zap.String("version", base.Hex(w.userVersion)))
					pulled = true
				}
			}
		}

		// pull personal ranking model
		if w.latestPRVersion != w.prModelVersion {
			base.Logger().Info("start pull personal ranking model")
			if mfResponse, err := w.MasterClient.GetPRModel(context.Background(),
				&protocol.NodeInfo{
					NodeType: protocol.NodeType_WorkerNode,
					NodeName: w.workerName,
				}, grpc.MaxCallRecvMsgSize(10e8)); err != nil {
				base.Logger().Error("failed to pull personal ranking model", zap.Error(err))
			} else {
				w.prModel, err = pr.DecodeModel(mfResponse.Name, mfResponse.Model)
				if err != nil {
					base.Logger().Error("failed to decode personal ranking model", zap.Error(err))
				} else {
					w.prModelVersion = mfResponse.Version
					base.Logger().Info("synced personal ranking model",
						zap.String("version", base.Hex(w.prModelVersion)))
					pulled = true
				}
			}
		}

		if pulled {
			w.syncedChan <- true
		}
	}
}

func (w *Worker) ServeMetrics() {
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", w.httpHost, w.httpPort), nil)
	if err != nil {
		base.Logger().Fatal("failed to start http server", zap.Error(err))
	}
}

func (w *Worker) Serve() {
	rand.Seed(time.Now().UTC().UnixNano())
	// open local store
	state, err := LoadLocalCache(filepath.Join(os.TempDir(), "gorse-worker"))
	if err != nil {
		base.Logger().Error("failed to load persist state", zap.Error(err),
			zap.String("path", filepath.Join(os.TempDir(), "gorse-server")))
	}
	if state.WorkerName == "" {
		state.WorkerName = base.GetRandomName(0)
		err = state.WriteLocalCache()
		if err != nil {
			base.Logger().Fatal("failed to write meta", zap.Error(err))
		}
	}
	w.workerName = state.WorkerName
	base.Logger().Info("start worker",
		zap.Int("n_jobs", w.Jobs),
		zap.String("worker_name", w.workerName))

	// connect to master
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", w.masterHost, w.masterPort), grpc.WithInsecure())
	if err != nil {
		base.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	w.MasterClient = protocol.NewMasterClient(conn)

	go w.Sync()
	go w.Pull()
	go w.ServeMetrics()

	loop := func() {
		if w.userIndex == nil {
			base.Logger().Debug("user index doesn't exist")
		} else {
			// split users
			workingUsers, err := split(w.userIndex, w.peers, w.me)
			if err != nil {
				base.Logger().Error("failed to split users", zap.Error(err),
					zap.String("me", w.me),
					zap.Strings("workers", w.peers))
				return
			}

			// offline recommendation
			if w.prModel != nil {
				w.Recommend(w.prModel, workingUsers)
			} else {
				base.Logger().Debug("local personal ranking model doesn't exist")
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

func (w *Worker) Recommend(m pr.Model, users []string) {
	var userIndexer base.Index
	// load user index
	if _, ok := m.(pr.MatrixFactorization); ok {
		userIndexer = m.(pr.MatrixFactorization).GetUserIndex()
	}
	// load item index
	itemIds := m.GetItemIndex().GetNames()
	base.Logger().Info("personal ranking recommendation",
		zap.Int("n_working_users", len(users)),
		zap.Int("n_items", len(itemIds)),
		zap.Int("n_jobs", w.Jobs),
		zap.Int("cache_size", w.cfg.Database.CacheSize))
	// progress tracker
	completed := make(chan interface{}, 1000)
	go func() {
		defer base.CheckPanic()
		completedCount := 0
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case _, ok := <-completed:
				if !ok {
					return
				}
				completedCount++
			case <-ticker.C:
				base.Logger().Info("personal ranking recommendation",
					zap.Int("n_complete_users", completedCount),
					zap.Int("n_working_users", len(users)))
			}
		}
	}()
	// collaborative filtering recommendation
	startTime := time.Now()
	_ = base.Parallel(len(users), w.Jobs, func(workerId, jobId int) error {
		userId := users[jobId]
		// convert to user index
		var userIndex int
		if _, ok := m.(pr.MatrixFactorization); ok {
			userIndex = userIndexer.ToNumber(userId)
		}
		// skip inactive users before max recommend period
		if !w.checkRecommendCacheTimeout(userId) {
			return nil
		}
		// Clear ignore items in cache. Since ignore items have been ignored
		// in offline recommendation stage.
		err := w.cacheStore.ClearList(cache.IgnoreItems, userId)
		if err != nil {
			return err
		}
		// remove saw items
		historyItems, err := loadFeedbackItems(w.dataStore, userId)
		historySet := set.NewStringSet(historyItems...)
		if err != nil {
			base.Logger().Error("failed to pull user feedback",
				zap.String("user_id", userId), zap.Error(err))
			return err
		}
		var favoredItemIndices []int
		if _, ok := m.(*pr.KNN); ok {
			favoredItems, err := loadFeedbackItems(w.dataStore, userId, w.cfg.Database.PositiveFeedbackType...)
			if err != nil {
				base.Logger().Error("failed to pull user feedback",
					zap.String("user_id", userId), zap.Error(err))
				return err
			}
			for _, itemId := range favoredItems {
				itemIndex := m.GetItemIndex().ToNumber(itemId)
				if itemIndex != base.NotId {
					favoredItemIndices = append(favoredItemIndices, itemIndex)
				}
			}
		}
		recItems := base.NewTopKStringFilter(w.cfg.Database.CacheSize)
		for itemIndex, itemId := range itemIds {
			if !historySet.Has(itemId) {
				switch m.(type) {
				case pr.MatrixFactorization:
					recItems.Push(itemId, m.(pr.MatrixFactorization).InternalPredict(userIndex, itemIndex))
				case *pr.KNN:
					recItems.Push(itemId, m.(*pr.KNN).InternalPredict(favoredItemIndices, itemIndex))
				default:
					base.Logger().Error("unknown model type")
				}
			}
		}
		elems, scores := recItems.PopAll()
		if err = w.cacheStore.SetScores(cache.CollaborativeItems, userId, cache.CreateScoredItems(elems, scores)); err != nil {
			base.Logger().Error("failed to cache collaborative filtering recommendation", zap.Error(err))
			return err
		}
		if err = w.cacheStore.SetString(cache.LastUpdateRecommendTime, userId, base.Now()); err != nil {
			base.Logger().Error("failed to cache collaborative filtering recommendation time", zap.Error(err))
		}
		completed <- nil
		return nil
	})
	close(completed)
	base.Logger().Info("complete personal ranking recommendation",
		zap.String("used_time", time.Since(startTime).String()))
}

// checkRecommendCacheTimeout checks if recommend cache stale.
// 1. if active time > recommend time, stale.
// 2. if recommend time + timeout < now, stale.
func (w *Worker) checkRecommendCacheTimeout(userId string) bool {
	var activeTime, recommendTime time.Time
	// read active time
	activeTimeLiteral, err := w.cacheStore.GetString(cache.LastActiveTime, userId)
	if err != nil {
		if err != cache.ErrObjectNotExist {
			base.Logger().Error("failed to read meta", zap.Error(err))
		}
	} else {
		activeTime, err = dateparse.ParseAny(activeTimeLiteral)
		if err != nil {
			base.Logger().Error("failed to time", zap.Error(err))
		}
	}
	// read recommend time
	recommendTimeLiteral, err := w.cacheStore.GetString(cache.LastUpdateRecommendTime, userId)
	if err != nil {
		if err != cache.ErrObjectNotExist {
			base.Logger().Error("failed to read meta", zap.Error(err))
		} else {
			return true
		}
	} else {
		recommendTime, err = dateparse.ParseAny(recommendTimeLiteral)
		if err != nil {
			base.Logger().Error("failed to time", zap.Error(err))
		}
	}
	// check time
	if activeTime.Unix() < recommendTime.Unix() {
		timeoutTime := recommendTime.Add(time.Hour * 24 * time.Duration(w.cfg.Recommend.MaxRecommendPeriod))
		return timeoutTime.Unix() < time.Now().Unix()
	}
	return true
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
//	_ = base.Parallel(len(users), w.FitJobs, func(workerId, jobId int) error {
//		user := users[jobId]
//		// collect items
//		historySet := base.NewStringSet()
//		for _, feedbackType := range w.cfg.Database.PositiveFeedbackType {
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

func loadFeedbackItems(database data.Database, userId string, feedbackTypes ...string) ([]string, error) {
	items := make([]string, 0)
	if len(feedbackTypes) == 0 {
		feedbacks, err := database.GetUserFeedback(userId, nil)
		if err != nil {
			return nil, err
		}
		for _, feedback := range feedbacks {
			items = append(items, feedback.ItemId)
		}
	} else {
		for _, tp := range feedbackTypes {
			feedbacks, err := database.GetUserFeedback(userId, &tp)
			if err != nil {
				return nil, err
			}
			for _, feedback := range feedbacks {
				items = append(items, feedback.ItemId)
			}
		}
	}
	return items, nil
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
