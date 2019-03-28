package cmd_serve

import (
	"github.com/BurntSushi/toml"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zhenghaoz/gorse/app/engine"
	"github.com/zhenghaoz/gorse/core"
	"log"
	"time"
)

// ModelDaemon is watching database and calls UpdateModel when necessary.
func ModelDaemon(config TomlConfig, metaData toml.MetaData) {
	log.Println("start model daemon")
	for {
		// Count ratings
		count, err := db.RatingCount()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("current number of ratings: %v\n", count)
		// Get commit ratings
		lastCount, err := db.GetMeta(engine.LastCount)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("last number of ratings: %v\n", lastCount)
		// Compare
		if count-lastCount > config.Recommend.UpdateThreshold {
			log.Printf("current count (%v) - last count (%v) > threshold (%v), start to update recommends\n",
				count, lastCount, config.Recommend.UpdateThreshold)
			UpdateModel(config, metaData)
		}
		// Sleep
		time.Sleep(time.Duration(config.Recommend.CheckPeriod) * time.Minute)
	}
}

// UpdateModel trains a new model and updates cached recommendations.
func UpdateModel(config TomlConfig, metaData toml.MetaData) {
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
	// Generate list
	log.Println("generate list")
	items := core.Items(dataSet)
	for denseUserId := 0; denseUserId < dataSet.UserCount(); denseUserId++ {
		userId := dataSet.UserIdSet.ToSparseId(denseUserId)
		exclude := dataSet.GetUserRatingsSet(userId)
		recommendItems, ratings := core.Top(items, userId, config.Recommend.CacheSize, exclude, model)
		if err = db.UpdateRecommends(userId, recommendItems, ratings); err != nil {
			log.Fatal(err)
		}
	}
	log.Println("recommends update-to-date")
}
