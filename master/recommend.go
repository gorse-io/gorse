package master

import (
	"fmt"
	"github.com/chewxy/math32"
	"github.com/scylladb/go-set"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/pr"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"sort"
	"time"
)

// popItem updates popular items for the database.
func (m *Master) popItem(items []data.Item, feedback []data.Feedback) {
	base.Logger().Info("collect popular items", zap.Int("n_cache", m.cfg.Database.CacheSize))
	// create item mapping
	itemMap := make(map[string]data.Item)
	for _, item := range items {
		itemMap[item.ItemId] = item
	}
	// count feedback
	timeWindowLimit := time.Now().AddDate(0, 0, -m.cfg.Recommend.PopularWindow)
	count := make(map[string]int)
	for _, fb := range feedback {
		if fb.Timestamp.After(timeWindowLimit) {
			count[fb.ItemId]++
		}
	}
	// collect pop items
	popItems := make(map[string]*base.TopKStringFilter)
	popItems[""] = base.NewTopKStringFilter(m.cfg.Database.CacheSize)
	for itemId, f := range count {
		popItems[""].Push(itemId, float32(f))
		item := itemMap[itemId]
		for _, label := range item.Labels {
			if _, exists := popItems[label]; !exists {
				popItems[label] = base.NewTopKStringFilter(m.cfg.Database.CacheSize)
			}
			popItems[label].Push(itemId, float32(f))
		}
	}
	// write back
	for label, topItems := range popItems {
		result, _ := topItems.PopAll()
		if err := m.cacheStore.SetList(cache.PopularItems, label, result); err != nil {
			base.Logger().Error("failed to cache popular items", zap.Error(err))
		}
	}
	if err := m.cacheStore.SetString(cache.GlobalMeta, cache.CollectPopularTime, base.Now()); err != nil {
		base.Logger().Error("failed to cache popular items", zap.Error(err))
	}
}

// latest updates latest items.
func (m *Master) latest(items []data.Item) {
	base.Logger().Info("collect latest items", zap.Int("n_cache", m.cfg.Database.CacheSize))
	var err error
	latestItems := make(map[string]*base.TopKStringFilter)
	latestItems[""] = base.NewTopKStringFilter(m.cfg.Database.CacheSize)
	// find latest items
	for _, item := range items {
		latestItems[""].Push(item.ItemId, float32(item.Timestamp.Unix()))
		for _, label := range item.Labels {
			if _, exist := latestItems[label]; !exist {
				latestItems[label] = base.NewTopKStringFilter(m.cfg.Database.CacheSize)
			}
			latestItems[label].Push(item.ItemId, float32(item.Timestamp.Unix()))
		}
	}
	for label, topItems := range latestItems {
		result, _ := topItems.PopAll()
		if err = m.cacheStore.SetList(cache.LatestItems, label, result); err != nil {
			base.Logger().Error("failed to cache latest items", zap.Error(err))
		}
	}
	if err = m.cacheStore.SetString(cache.GlobalMeta, cache.CollectLatestTime, base.Now()); err != nil {
		base.Logger().Error("failed to cache latest items time", zap.Error(err))
	}
}

// similar updates neighbors for the database.
func (m *Master) similar(items []data.Item, dataset *pr.DataSet, source, similarity string) {
	base.Logger().Info("collect similar items", zap.Int("n_cache", m.cfg.Database.CacheSize))
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
	switch source {
	case model.SimilarityCollaborative:
		for _, feedbacks := range dataset.ItemFeedback {
			sort.Ints(feedbacks)
		}
	case model.SimilarityFeature:
		for _, item := range items {
			sort.Strings(item.Labels)
		}
	default:
		base.Logger().Error("unknown similarity source", zap.String("source", source))
	}

	if err := base.Parallel(dataset.ItemCount(), m.cfg.Master.FitJobs, func(workerId, jobId int) error {
		users := dataset.ItemFeedback[jobId]
		// Collect candidates
		itemSet := set.NewIntSet()
		for _, u := range users {
			itemSet.Add(dataset.UserFeedback[u]...)
		}
		// Ranking
		nearItems := base.NewTopKFilter(m.cfg.Database.CacheSize)
		for j := range itemSet.List() {
			if j != jobId {
				var score float32
				switch source {
				case model.SimilarityCollaborative:
					score = dotInt(dataset.ItemFeedback[jobId], dataset.ItemFeedback[j])
					if similarity == model.SimilarityCosine {
						score /= math32.Sqrt(float32(len(dataset.ItemFeedback[jobId])))
						score /= math32.Sqrt(float32(len(dataset.ItemFeedback[j])))
					}
				case model.SimilarityFeature:
					score = dotString(items[jobId].Labels, items[j].Labels)
					if similarity == model.SimilarityCosine {
						score /= math32.Sqrt(float32(len(items[jobId].Labels)))
						score /= math32.Sqrt(float32(len(items[j].Labels)))
					}
				}
				nearItems.Push(j, score)
			}
		}
		elem, _ := nearItems.PopAll()
		recommends := make([]string, len(elem))
		for i := range recommends {
			recommends[i] = dataset.ItemIndex.ToName(elem[i])
		}
		if err := m.cacheStore.SetList(cache.SimilarItems, dataset.ItemIndex.ToName(jobId), recommends); err != nil {
			return err
		}
		completed <- nil
		return nil
	}); err != nil {
		base.Logger().Error("failed to cache similar items", zap.Error(err))
	}
	close(completed)
	if err := m.cacheStore.SetString(cache.GlobalMeta, cache.CollectSimilarTime, base.Now()); err != nil {
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

func (m *Master) fitPRModel(dataSet *pr.DataSet, prModel pr.Model) {
	base.Logger().Info("fit personal ranking model", zap.Int("n_jobs", m.cfg.Master.FitJobs))
	// training model
	trainSet, testSet := dataSet.Split(0, 0)
	score := prModel.Fit(trainSet, testSet, nil)
	// update match model
	m.prMutex.Lock()
	m.prModel = prModel
	m.prVersion++
	m.prMutex.Unlock()
	base.Logger().Info("fit personal ranking model complete",
		zap.String("version", fmt.Sprintf("%x", m.prVersion)))
	if err := m.dataStore.InsertMeasurement(data.Measurement{Name: "NDCG@10", Value: score.NDCG, Timestamp: time.Now()}); err != nil {
		base.Logger().Error("failed to insert measurement", zap.Error(err))
	}
	if err := m.dataStore.InsertMeasurement(data.Measurement{Name: "Recall@10", Value: score.Recall, Timestamp: time.Now()}); err != nil {
		base.Logger().Error("failed to insert measurement", zap.Error(err))
	}
	if err := m.dataStore.InsertMeasurement(data.Measurement{Name: "Precision@10", Value: score.Precision, Timestamp: time.Now()}); err != nil {
		base.Logger().Error("failed to insert measurement", zap.Error(err))
	}
	if err := m.cacheStore.SetString(cache.GlobalMeta, cache.FitMatrixFactorizationTime, base.Now()); err != nil {
		base.Logger().Error("failed to write meta", zap.Error(err))
	}
	if err := m.cacheStore.SetString(cache.GlobalMeta, cache.MatrixFactorizationVersion, fmt.Sprintf("%x", m.prVersion)); err != nil {
		base.Logger().Error("failed to write meta", zap.Error(err))
	}
}
