package serve

import (
	"github.com/BurntSushi/toml"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/engine"
	"log"
	"strconv"
	"time"
)

// Watcher is watching database and calls UpdateRecommends when necessary.
func Watcher(config engine.TomlConfig, metaData toml.MetaData) {
	log.Println("start model daemon")
	for {
		// Count ratings
		count, err := db.CountItems()
		if err != nil {
			log.Fatal(err)
		}
		// Get commit ratings
		lastCountString, err := db.GetMeta("last_count")
		if err != nil {
			log.Fatal(err)
		}
		lastCount, err := strconv.Atoi(lastCountString)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("current number of ratings: %v, last number of ratings: %v\n", count, lastCount)
		// Compare
		if count-lastCount > config.Recommend.UpdateThreshold {
			log.Printf("current count (%v) - last count (%v) > threshold (%v), start to update recommends\n",
				count, lastCount, config.Recommend.UpdateThreshold)
			UpdateRecommends(config, metaData)
			if err = db.SetMeta("last_count", strconv.Itoa(count)); err != nil {
				log.Println(err)
			}
			log.Printf("recommends update-to-date, last_count = %v", count)
		}
		// Sleep
		time.Sleep(time.Duration(config.Recommend.CheckPeriod) * time.Minute)
	}
}

// UpdateRecommends trains a new model and updates cached recommendations.
func UpdateRecommends(config engine.TomlConfig, metaData toml.MetaData) {
	// Load cmd_data from database
	log.Println("load data from database")
	dataSet, err := db.ToDataSet()
	if err != nil {
		log.Fatal(err)
	}
	// Create model
	params := config.Params.ToParams(metaData)
	log.Printf("create model %v with params = %v\n", config.Recommend.Model, params)
	model := engine.CreateModelFromName(config.Recommend.Model, params)
	// Training model
	log.Println("training model")
	model.Fit(dataSet, nil)
	// Generate recommends
	log.Println("generate recommends")
	items := core.Items(dataSet)
	for userIndex := 0; userIndex < dataSet.UserCount(); userIndex++ {
		userId := dataSet.UserIndexer().ToID(userIndex)
		exclude := dataSet.UserByIndex(userIndex)
		recommendItems, ratings := core.Top(items, userId, config.Recommend.CacheSize, exclude, model)
		recommends := make([]engine.RecommendedItem, len(recommendItems))
		for i := range recommends {
			recommends[i].ItemId = recommendItems[i]
			recommends[i].Score = ratings[i]
		}
		if err = db.SetRecommends(userId, recommends); err != nil {
			log.Fatal(err)
		}
	}
	// Generate neighbors
	log.Printf("generate neighbors by %v", config.Recommend.Similarity)
	similarity := engine.CreateSimilarityFromName(config.Recommend.Similarity)
	for denseItemId := 0; denseItemId < dataSet.ItemCount(); denseItemId++ {
		itemId := dataSet.ItemIndexer().ToID(denseItemId)
		neighbors, similarities := core.Neighbors(dataSet, itemId, config.Recommend.CacheSize, similarity)
		recommends := make([]engine.RecommendedItem, len(neighbors))
		for i := range recommends {
			recommends[i].ItemId = neighbors[i]
			recommends[i].Score = similarities[i]
		}
		if err = db.SetNeighbors(itemId, recommends); err != nil {
			log.Fatal(err)
		}
	}
}
