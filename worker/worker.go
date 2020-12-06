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
	"github.com/hashicorp/memberlist"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/protocol"
	"google.golang.org/genproto/googleapis/spanner/admin/database/v1"
	"log"
	"sync"
	"time"
)

type Worker struct {
	protocol.UnimplementedWorkerServer
	cfg     *config.WorkerConfig
	members *memberlist.Memberlist

	// recommender
	model      model.Model
	modelMutex sync.Mutex

	// user partition
	userPartition *protocol.Partition
	userMutex     sync.Mutex

	// item partition
	itemPartition *protocol.Partition
	itemMutex     sync.Mutex

	// label partition
	labelPartition *protocol.Partition
	labelMutex     sync.Mutex
}

func (w *Worker) Serve() {
	cfg := memberlist.DefaultLocalConfig()
	cfg.Name = protocol.NewName(protocol.WorkerNodePrefix, w.cfg.RPCPort, cfg.Name)
	cfg.BindPort = w.cfg.GossipPort
	var err error
	w.members, err = memberlist.Create(cfg)
	if err != nil {
		log.Fatal("Failed to create memberlist: " + err.Error())
	}
	_, err = w.members.Join([]string{w.cfg.LeaderAddr})
	if err != nil {
		log.Fatal("Failed to join cluster: " + err.Error())
	}

	for {
		time.Sleep(time.Minute * time.Duration(w.cfg.Watch))
	}
}

func (w *Worker) BroadcastModel(ctx context.Context, data *protocol.Model) (*protocol.Response, error) {
	// decode model
	reader := bytes.NewReader(data.Model)
	decoder := gob.NewDecoder(reader)
	var m model.Model
	if err := decoder.Decode(&m); err != nil {
		return nil, err
	}
	// replace model
	w.modelMutex.Lock()
	w.model = m
	w.modelMutex.Unlock()
	return &protocol.Response{}, nil
}

func (w *Worker) BroadcastUserPartition(ctx context.Context, userPartition *protocol.Partition) (*protocol.Response, error) {
	// update partition
	w.userMutex.Lock()
	w.userPartition = userPartition
	w.userMutex.Unlock()
	return &protocol.Response{}, nil
}

func (w *Worker) BroadcastItemPartition(ctx context.Context, itemPartition *protocol.Partition) (*protocol.Response, error) {
	// update partition
	w.itemMutex.Lock()
	w.itemPartition = itemPartition
	w.itemMutex.Unlock()
	return &protocol.Response{}, nil
}

func (w *Worker) BroadcastLabelPartition(ctx context.Context, labelPartition *protocol.Partition) (*protocol.Response, error) {
	// update partition
	w.labelMutex.Lock()
	w.labelPartition = labelPartition
	w.labelMutex.Unlock()
	return &protocol.Response{}, nil
}

func LabelCosine(db *database.Database, item1 string, item2 string) (float64, error) {
	a, err := db.GetItem(item1)
	if err != nil {
		return 0, err
	}
	b, err := db.GetItem(item2)
	if err != nil {
		return 0, err
	}
	labelSet := make(map[string]interface{})
	for _, label := range a.Labels {
		labelSet[label] = nil
	}
	intersect := 0.0
	for _, label := range b.Labels {
		if _, ok := labelSet[label]; ok {
			intersect++
		}
	}
	if intersect == 0 {
		return 0, nil
	}
	return intersect / math.Sqrt(float64(len(a.Labels))) / math.Sqrt(float64(len(b.Labels))), nil
}

func FeedbackCosine(db *database.Database, item1 string, item2 string) (float64, error) {
	feedback1, err := db.GetFeedbackByItem(item1)
	if err != nil {
		return 0, err
	}
	feedback2, err := db.GetFeedbackByItem(item2)
	if err != nil {
		return 0, err
	}
	userSet := make(map[string]interface{})
	for _, f := range feedback1 {
		userSet[f.UserId] = nil
	}
	intersect := 0.0
	for _, f := range feedback2 {
		if _, ok := userSet[f.UserId]; ok {
			intersect++
		}
	}
	if intersect == 0 {
		return 0, nil
	}
	return intersect / math.Sqrt(float64(len(feedback1))) / math.Sqrt(float64(len(feedback2))), nil
}

// RefreshNeighbors updates neighbors for the database.
func RefreshNeighbors(db *database.Database, collectSize int, numJobs int) error {
	items1, err := db.GetItems(0, 0)
	if err != nil {
		return err
	}
	return base.Parallel(len(items1), numJobs, func(i int) error {
		item1 := items1[i]
		// Collect candidates
		itemSet := make(map[string]interface{})
		feedback1, err := db.GetFeedbackByItem(item1.ItemId)
		if err != nil {
			return err
		}
		for _, f := range feedback1 {
			items2, err := db.GetFeedbackByUser(f.UserId)
			if err != nil {
				return err
			}
			for _, item2 := range items2 {
				itemSet[item2.ItemId] = nil
			}
		}
		// Ranking
		nearItems := base.NewMaxHeap(collectSize)
		for item2 := range itemSet {
			if item2 != item1.ItemId {
				score, err := FeedbackCosine(db, item1.ItemId, item2)
				if err != nil {
					return err
				}
				nearItems.Add(item2, score)
			}
		}
		elem, scores := nearItems.ToSorted()
		recommends := make([]database.RecommendedItem, len(elem))
		for i := range recommends {
			itemId := elem[i].(string)
			item, err := db.GetItem(itemId)
			if err != nil {
				return err
			}
			recommends[i] = database.RecommendedItem{Item: item, Score: scores[i]}
		}
		if err := db.SetNeighbors(item1.ItemId, recommends); err != nil {
			return err
		}
		return nil
	})
}

// TopItems finds top items by weights.
func TopItems(itemId []database.Item, weight []float64, n int) (topItemId []database.Item, topWeight []float64) {
	popItems := base.NewMaxHeap(n)
	for i := range itemId {
		popItems.Add(itemId[i], weight[i])
	}
	elem, scores := popItems.ToSorted()
	recommends := make([]database.Item, len(elem))
	for i := range recommends {
		recommends[i] = elem[i].(database.Item)
	}
	return recommends, scores
}

func TopLabledItems(items []database.Item, weight []float64, n int) map[string][]database.RecommendedItem {
	popItems := make(map[string]*base.MaxHeap)
	popItems[""] = base.NewMaxHeap(n)
	for i, item := range items {
		popItems[""].Add(items[i], weight[i])
		for _, label := range item.Labels {
			if _, exist := popItems[label]; !exist {
				popItems[label] = base.NewMaxHeap(n)
			}
			popItems[label].Add(items[i], weight[i])
		}
	}
	result := make(map[string][]database.RecommendedItem)
	for label := range popItems {
		elem, scores := popItems[label].ToSorted()
		items := make([]database.RecommendedItem, len(elem))
		for i := range items {
			items[i].Item = elem[i].(database.Item)
			items[i].Score = scores[i]
		}
		result[label] = items
	}
	return result
}

// RefreshRecommends updates personalized recommendations for the database.
func RefreshRecommends(db *database.Database, ranker model.ModelInterface, n int, numJobs int, collectors ...Collector) error {
	// Get users
	users, err := db.GetUsers()
	if err != nil {
		return err
	}
	return base.Parallel(len(users), numJobs, func(i int) error {
		userId := users[i]
		// Check cache
		recommends, err := db.GetRecommend(userId, 0, 0)
		if err != nil {
			return err
		}
		if len(recommends) >= n {
			return nil
		}
		// Collect candidates
		itemSet, err := Collect(db, userId, n, collectors...)
		if err != nil {
			return err
		}
		items := make([]database.Item, 0, len(itemSet))
		ratings := make([]float64, 0, len(itemSet))
		for _, item := range itemSet {
			items = append(items, item.Item)
			ratings = append(ratings, ranker.Predict(userId, item.ItemId))
		}
		items, ratings = TopItems(items, ratings, n)
		recommends = make([]database.RecommendedItem, len(items))
		for i := range recommends {
			recommends[i].Item = items[i]
			recommends[i].Score = ratings[i]
		}
		if err := db.SetRecommend(userId, recommends); err != nil {
			return err
		}
		return nil
	})
}
