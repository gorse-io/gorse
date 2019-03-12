package server

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zhenghaoz/gorse/core"
	"io"
	"log"
)

func modelKeeper(config TomlConfig, metaData toml.MetaData) {
	// Connect to database
	log.Printf("Connect to database (%s)\n", config.Database.Driver)
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
	// Generate list
	log.Println("Generate list")
	buf := bytes.NewBuffer(nil)
	items := core.Items(dataSet)
	for denseUserId := 0; denseUserId < dataSet.UserCount(); denseUserId++ {
		userId := dataSet.UserIdSet.ToSparseId(denseUserId)
		exclude := dataSet.GetUserRatingsSet(userId)
		recommendItems := core.Top(items, userId, config.Recommend.CacheSize, exclude, model)
		for i, itemId := range recommendItems {
			buf.WriteString(fmt.Sprintf("%d\t%d\t%d\n", userId, itemId, i))
		}
	}
	// Save list
	mysql.RegisterReaderHandler("data", func() io.Reader {
		return bytes.NewReader(buf.Bytes())
	})
	_, err = db.Exec("LOAD DATA LOCAL INFILE 'Reader::data' INTO TABLE recommends")
	if err != nil {
		log.Fatal(err)
	}
}
