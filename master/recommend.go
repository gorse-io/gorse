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
	"fmt"
	"github.com/chewxy/math32"
	"github.com/scylladb/go-set"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"sort"
	"time"
)

// popItem updates popular items for the database.
func (m *Master) popItem(items []data.Item, feedback []data.Feedback) {
	base.Logger().Info("collect popular items", zap.Int("n_cache", m.GorseConfig.Database.CacheSize))
	// create item mapping
	itemMap := make(map[string]data.Item)
	for _, item := range items {
		itemMap[item.ItemId] = item
	}
	// count feedback
	timeWindowLimit := time.Now().AddDate(0, 0, -m.GorseConfig.Recommend.PopularWindow)
	count := make(map[string]int)
	for _, fb := range feedback {
		if fb.Timestamp.After(timeWindowLimit) {
			count[fb.ItemId]++
		}
	}
	// collect pop items
	popItems := make(map[string]*base.TopKStringFilter)
	popItems[""] = base.NewTopKStringFilter(m.GorseConfig.Database.CacheSize)
	for itemId, f := range count {
		popItems[""].Push(itemId, float32(f))
		item := itemMap[itemId]
		for _, label := range item.Labels {
			if _, exists := popItems[label]; !exists {
				popItems[label] = base.NewTopKStringFilter(m.GorseConfig.Database.CacheSize)
			}
			popItems[label].Push(itemId, float32(f))
		}
	}
	// write back
	for label, topItems := range popItems {
		result, scores := topItems.PopAll()
		if err := m.CacheClient.SetScores(cache.PopularItems, label, cache.CreateScoredItems(result, scores)); err != nil {
			base.Logger().Error("failed to cache popular items", zap.Error(err))
		}
	}
	if err := m.CacheClient.SetString(cache.GlobalMeta, cache.LastUpdatePopularTime, base.Now()); err != nil {
		base.Logger().Error("failed to cache popular items", zap.Error(err))
	}
}

// latest updates latest items.
func (m *Master) latest(items []data.Item) {
	base.Logger().Info("collect latest items", zap.Int("n_cache", m.GorseConfig.Database.CacheSize))
	var err error
	latestItems := make(map[string]*base.TopKStringFilter)
	latestItems[""] = base.NewTopKStringFilter(m.GorseConfig.Database.CacheSize)
	// find latest items
	for _, item := range items {
		latestItems[""].Push(item.ItemId, float32(item.Timestamp.Unix()))
		for _, label := range item.Labels {
			if _, exist := latestItems[label]; !exist {
				latestItems[label] = base.NewTopKStringFilter(m.GorseConfig.Database.CacheSize)
			}
			latestItems[label].Push(item.ItemId, float32(item.Timestamp.Unix()))
		}
	}
	for label, topItems := range latestItems {
		result, scores := topItems.PopAll()
		if err = m.CacheClient.SetScores(cache.LatestItems, label, cache.CreateScoredItems(result, scores)); err != nil {
			base.Logger().Error("failed to cache latest items", zap.Error(err))
		}
	}
	if err = m.CacheClient.SetString(cache.GlobalMeta, cache.LastUpdateLatestTime, base.Now()); err != nil {
		base.Logger().Error("failed to cache latest items time", zap.Error(err))
	}
}

// similar updates neighbors for the database.
func (m *Master) similar(items []data.Item, dataset *ranking.DataSet, similarity string) {
	base.Logger().Info("collect similar items", zap.Int("n_cache", m.GorseConfig.Database.CacheSize))
	// create progress tracker
	completed := make(chan []interface{}, 1000)
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
			case <-ticker.C:
				base.Logger().Debug("collect similar items",
					zap.Int("n_complete_items", completedCount),
					zap.Int("n_items", dataset.ItemCount()))
			}
		}
	}()

	// pre-ranking
	for _, feedbacks := range dataset.ItemFeedback {
		sort.Ints(feedbacks)
	}

	if err := base.Parallel(dataset.ItemCount(), m.GorseConfig.Master.FitJobs, func(workerId, jobId int) error {
		users := dataset.ItemFeedback[jobId]
		// Collect candidates
		itemSet := set.NewIntSet()
		for _, u := range users {
			itemSet.Add(dataset.UserFeedback[u]...)
		}
		// Ranking
		nearItems := base.NewTopKFilter(m.GorseConfig.Database.CacheSize)
		for j := range itemSet.List() {
			if j != jobId {
				var score float32
				score = dotInt(dataset.ItemFeedback[jobId], dataset.ItemFeedback[j])
				if similarity == model.SimilarityCosine {
					score /= math32.Sqrt(float32(len(dataset.ItemFeedback[jobId])))
					score /= math32.Sqrt(float32(len(dataset.ItemFeedback[j])))
				}
				nearItems.Push(j, score)
			}
		}
		elem, scores := nearItems.PopAll()
		recommends := make([]string, len(elem))
		for i := range recommends {
			recommends[i] = dataset.ItemIndex.ToName(elem[i])
		}
		if err := m.CacheClient.SetScores(cache.SimilarItems, dataset.ItemIndex.ToName(jobId), cache.CreateScoredItems(recommends, scores)); err != nil {
			return err
		}
		completed <- nil
		return nil
	}); err != nil {
		base.Logger().Error("failed to cache similar items", zap.Error(err))
	}
	close(completed)
	if err := m.CacheClient.SetString(cache.GlobalMeta, cache.LastUpdateNeighborTime, base.Now()); err != nil {
		base.Logger().Error("failed to cache similar items", zap.Error(err))
	}
}

func dotString(a, b []string) float32 {
	i, j, sum := 0, 0, float32(0)
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			i++
			j++
			sum++
		} else if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		}
	}
	return sum
}

func dotInt(a, b []int) float32 {
	i, j, sum := 0, 0, float32(0)
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			i++
			j++
			sum++
		} else if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		}
	}
	return sum
}

func (m *Master) fitRankingModel(dataSet *ranking.DataSet, prModel ranking.Model) {
	base.Logger().Info("fit personal ranking model", zap.Int("n_jobs", m.GorseConfig.Master.FitJobs))
	// training model
	trainSet, testSet := dataSet.Split(0, 0)
	score := prModel.Fit(trainSet, testSet, nil)
	// update match model
	m.rankingModelMutex.Lock()
	m.rankingModel = prModel
	m.rankingModelVersion++
	m.rankingScore = score
	m.rankingModelMutex.Unlock()
	base.Logger().Info("fit personal ranking model complete",
		zap.String("version", fmt.Sprintf("%x", m.rankingModelVersion)))
	if err := m.DataClient.InsertMeasurement(data.Measurement{Name: "NDCG@10", Value: score.NDCG, Timestamp: time.Now()}); err != nil {
		base.Logger().Error("failed to insert measurement", zap.Error(err))
	}
	if err := m.DataClient.InsertMeasurement(data.Measurement{Name: "Recall@10", Value: score.Recall, Timestamp: time.Now()}); err != nil {
		base.Logger().Error("failed to insert measurement", zap.Error(err))
	}
	if err := m.DataClient.InsertMeasurement(data.Measurement{Name: "Precision@10", Value: score.Precision, Timestamp: time.Now()}); err != nil {
		base.Logger().Error("failed to insert measurement", zap.Error(err))
	}
	if err := m.CacheClient.SetString(cache.GlobalMeta, cache.LastFitRankingModelTime, base.Now()); err != nil {
		base.Logger().Error("failed to write meta", zap.Error(err))
	}
	if err := m.CacheClient.SetString(cache.GlobalMeta, cache.LastRankingModelVersion, fmt.Sprintf("%x", m.rankingModelVersion)); err != nil {
		base.Logger().Error("failed to write meta", zap.Error(err))
	}
	// caching model
	m.localCache.RankingModelName = m.rankingModelName
	m.localCache.RankingModelVersion = m.rankingModelVersion
	m.localCache.RankingModel = prModel
	m.localCache.RankingScore = score
	m.localCache.UserIndex = m.userIndex
	if err := m.localCache.WriteLocalCache(); err != nil {
		base.Logger().Error("failed to write local cache", zap.Error(err))
	} else {
		base.Logger().Info("write model to local cache",
			zap.String("model_name", m.localCache.RankingModelName),
			zap.String("model_version", base.Hex(m.localCache.RankingModelVersion)),
			zap.Float32("model_score", m.localCache.RankingScore.NDCG),
			zap.Any("params", m.localCache.RankingModel.GetParams()))
	}
}
