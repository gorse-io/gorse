package engine

import (
	"github.com/BurntSushi/toml"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"fmt"
	"log"
)

// UpdatePopularity updates popularity for all items.
func UpdatePopularity(dataSet core.DataSetInterface, db *DB) error {
	log.Printf("update popularity")
	itemId, popularity := core.Popularity(dataSet)
	return db.UpdatePopularity(itemId, popularity)
}

// UpdatePopItem updates popular items for the database.
func UpdatePopItem(cacheSize int, db *DB) error {
	log.Printf("update popular items")
	items, err := db.GetItems()
	if err != nil {
		return err
	}
	timestamp := make([]float64, len(items))
	for i, item := range items {
		timestamp[i] = item.Popularity
	}
	items, scores := TopItems(items, timestamp, cacheSize)
	recommends := make([]RecommendedItem, len(items))
	for i := range recommends {
		recommends[i].Item = items[i]
		recommends[i].Score = scores[i]
	}
	return db.PutList(ListPop, recommends)
}

// UpdateLatest updates latest items.
func UpdateLatest(cacheSize int, db *DB) error {
	log.Printf("update latest items")
	// update latest items
	items, err := db.GetItems()
	if err != nil {
		return err
	}
	timestamp := make([]float64, len(items))
	for i, item := range items {
		timestamp[i] = float64(item.Timestamp.Unix())
	}
	recommendItems, scores := TopItems(items, timestamp, cacheSize)
	recommends := make([]RecommendedItem, len(recommendItems))
	for i := range recommends {
		recommends[i].Item = recommendItems[i]
		recommends[i].Score = scores[i]
	}
	return db.PutList(ListLatest, recommends)
}

// UpdateNeighbors updates neighbors for the database.
func UpdateNeighbors(name string, cacheSize int, dataSet core.DataSetInterface, db *DB) error {
	log.Printf("update neighbors by %v", name)
	similarity := LoadSimilarity(name)
	for denseItemId := 0; denseItemId < dataSet.ItemCount(); denseItemId++ {
		itemId := dataSet.ItemIndexer().ToID(denseItemId)
		neighbors, similarities := core.Neighbors(dataSet, itemId, cacheSize, similarity)
		recommends := make([]RecommendedItem, len(neighbors))
		items, err := db.GetItemsByID(neighbors)
		if err != nil {
			return err
		}
		for i := range recommends {
			recommends[i].Item = items[i]
			recommends[i].Score = similarities[i]
		}
		if err := db.PutIdentList(BucketNeighbors, itemId, recommends); err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

// UpdateRecommends updates personalized recommendations for the database.
func UpdateRecommends(name string, params base.Params, cacheSize int, fitJobs int, dataSet core.DataSetInterface, db *DB) error {
	// Create model
	log.Printf("create model %v with params = %v\n", name, params)
	model := LoadModel(name, params)
	if model == nil {
		log.Printf("invalid model %v, aborting\n", name)
		return fmt.Errorf("invalid model %v", name)
	}
	// Training model
	log.Println("training model")
	model.Fit(dataSet, &base.RuntimeOptions{Verbose: true, FitJobs: fitJobs})
	// Generate recommends
	log.Println("update recommends")
	items := core.Items(dataSet)
	for userIndex := 0; userIndex < dataSet.UserCount(); userIndex++ {
		userId := dataSet.UserIndexer().ToID(userIndex)
		exclude := dataSet.UserByIndex(userIndex)
		recommendItems, ratings := core.Top(items, userId, cacheSize, exclude, model)
		recommends := make([]RecommendedItem, len(recommendItems))
		items, err := db.GetItemsByID(recommendItems)
		if err != nil {
			return err
		}
		for i := range recommends {
			recommends[i].Item = items[i]
			recommends[i].Score = ratings[i]
		}
		if err := db.PutIdentList(BucketRecommends, userId, recommends); err != nil {
			return err
		}
	}
	return nil
}

// Update all kinds recommendations for the database.
func Update(config TomlConfig, metaData toml.MetaData, db *DB) error {
	// Load data
	log.Println("load data from database")
	dataSet, err := db.ToDataSet()
	if err != nil {
		return err
	}
	// Update popularity
	if err = UpdatePopularity(dataSet, db); err != nil {
		return err
	}
	// Update popular items
	if err = UpdatePopItem(config.Recommend.CacheSize, db); err != nil {
		return err
	}
	// Update latest items
	if err = UpdateLatest(config.Recommend.CacheSize, db); err != nil {
		return err
	}
	// Generate recommends
	params := config.Params.ToParams(metaData)
	if err = UpdateRecommends(config.Recommend.Model, params, config.Recommend.CacheSize, config.Recommend.FitJobs,
		dataSet, db); err != nil {
		return err
	}
	// Generate neighbors
	if err = UpdateNeighbors(config.Recommend.Similarity, config.Recommend.CacheSize, dataSet, db); err != nil {
		return err
	}
	return nil
}

// TopItems finds top items by weights.
func TopItems(itemId []Item, weight []float64, n int) (topItemId []Item, topWeight []float64) {
	popItems := base.NewMaxHeap(n)
	for i := range itemId {
		popItems.Add(itemId[i], weight[i])
	}
	elem, scores := popItems.ToSorted()
	recommends := make([]Item, len(elem))
	for i := range recommends {
		recommends[i] = elem[i].(Item)
	}
	return recommends, scores
}
