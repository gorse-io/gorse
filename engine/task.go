// Copyright 2020 Zhenghao Zhang
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
package engine

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"math"
)
import "github.com/zhenghaoz/gorse/database"

// RefreshPopItem updates popular items for the database.
func RefreshPopItem(db *database.Database, collectSize int) error {
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
