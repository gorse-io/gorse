package server

import (
	"database/sql"
	"github.com/BurntSushi/toml"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zhenghaoz/gorse/core"
	"log"
)

func modelKeeper(config TomlConfig, metaData toml.MetaData) {
	// Connect to database
	log.Println("Connect to database")
	db, err := sql.Open(config.Database.Driver, config.Database.Access)
	if err != nil {
		log.Fatal(err)
	}
	// Load data from database
	log.Println("Load data from database")
	dataSet, err := core.LoadDataFromSQL(db,
		"ratings", "user_id", "item_id", "rating")
	if err != nil {
		log.Fatal(err)
	}
	// Create model
	params := config.Params.ToParams(metaData)
	log.Printf("Create model %v with params = %v\n", config.Recommend.Model, params)
	model := CreateModelFromName(config.Recommend.Model, params)
	// Training model
	log.Printf("Training model\n")
	model.Fit(dataSet)
	// Prepare SQL
	statement, err := db.Prepare("INSERT INTO recommends VALUES(?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	// Generate list
	log.Println("Generate list")
	items := core.Items(dataSet)
	for denseUserId := 0; denseUserId < dataSet.UserCount(); denseUserId++ {
		userId := dataSet.UserIdSet.ToSparseId(denseUserId)
		exclude := dataSet.GetUserRatingsSet(userId)
		recommendItems := core.Top(items, userId, config.Recommend.CacheSize, exclude, model)
		for i, itemId := range recommendItems {
			_, err = statement.Exec(userId, itemId, i)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
