package serve

import (
	"github.com/BurntSushi/toml"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zhenghaoz/gorse/cmd/engine"
	"github.com/zhenghaoz/gorse/core"
	"log"
	"time"
)

// Watcher is watching database and calls UpdateRecommends when necessary.
func Watcher(config TomlConfig, metaData toml.MetaData) {
	log.Println("start model daemon")
	for {
		// Count ratings
		count, err := db.RatingCount()
		if err != nil {
			log.Fatal(err)
		}
		// Get commit ratings
		lastCount, err := db.GetMeta(engine.LastCount)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("current number of ratings: %v, last number of ratings: %v\n", count, lastCount)
		// Compare
		if count-lastCount > config.Recommend.UpdateThreshold {
			log.Printf("current count (%v) - last count (%v) > threshold (%v), start to update recommends\n",
				count, lastCount, config.Recommend.UpdateThreshold)
			UpdateRecommends(config, metaData)
			if err = db.SetMeta(engine.LastCount, count); err != nil {
				log.Println(err)
			}
			log.Printf("recommends update-to-date, last_count = %v", count)
		}
		// Sleep
		time.Sleep(time.Duration(config.Recommend.CheckPeriod) * time.Minute)
	}
}

// UpdateRecommends trains a new model and updates cached recommendations.
func UpdateRecommends(config TomlConfig, metaData toml.MetaData) {
	// Load cmd_data from database
	log.Println("load data from database")
	dataSet, err := db.LoadData()
	if err != nil {
		log.Fatal(err)
	}
	// Create model
	params := config.Params.ToParams(metaData)
	log.Printf("create model %v with params = %v\n", config.Recommend.Model, params)
	model := CreateModelFromName(config.Recommend.Model, params)
	// Training model
	log.Println("training model")
	model.Fit(dataSet)
	// Generate recommends
	log.Println("generate recommends")
	items := core.Items(dataSet)
	for denseUserId := 0; denseUserId < dataSet.UserCount(); denseUserId++ {
		userId := dataSet.UserIdSet.ToSparseId(denseUserId)
		exclude := dataSet.GetUserRatingsSet(userId)
		recommendItems, ratings := core.Top(items, userId, config.Recommend.CacheSize, exclude, model)
		if err = db.PutRecommends(userId, recommendItems, ratings); err != nil {
			log.Fatal(err)
		}
	}
	// Generate neighbors
	log.Printf("generate neighbors by %v", config.Recommend.Similarity)
	similarity := CreateSimilarityFromName(config.Recommend.Similarity)
	for denseItemId := 0; denseItemId < dataSet.ItemCount(); denseItemId++ {
		itemId := dataSet.ItemIdSet.ToSparseId(denseItemId)
		neighbors, similarities := core.Neighbors(dataSet, itemId, config.Recommend.CacheSize, similarity)
		if err = db.PutNeighbors(itemId, neighbors, similarities); err != nil {
			log.Fatal(err)
		}
	}
}
