package cmd

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/database"
	"github.com/zhenghaoz/gorse/engine"
	"log"
	"strconv"
	"time"
)

var db *database.DB

var commandServe = &cobra.Command{
	Use:   "serve",
	Short: "Start a recommender sever",
	Run: func(cmd *cobra.Command, args []string) {
		// Print welcome
		welcome()
		// Load configuration
		conf, metaData := loadConfig(cmd)
		// Connect database
		var err error
		db, err = database.Open(conf.Database.File)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("connect to database: %v", conf.Database.File)
		// Start model daemon
		go watch(conf, metaData)
		serve(conf)
	},
}

func init() {
	commandServe.PersistentFlags().StringP("config", "c", "", "Configure file")
}

func welcome() {
	// Print Header
	fmt.Println("     __ _  ___  _ __ ___  ___      ")
	fmt.Println("    / _` |/ _ \\| '__/ __|/ _ \\   ")
	fmt.Println("   | (_| | (_) | |  \\__ \\  __/   ")
	fmt.Println("    \\__, |\\___/|_|  |___/\\___|  ")
	fmt.Println("     __/ |                         ")
	fmt.Println("    |___/                          ")
	fmt.Println("                                   ")
	fmt.Println("gorse: Go Recommender System Engine")
	fmt.Println("-----------------------------------")
}

func loadConfig(cmd *cobra.Command) (engine.TomlConfig, toml.MetaData) {
	if !cmd.PersistentFlags().Changed("config") {
		log.Fatal("please use specify a configuration")
	}
	configFile, _ := cmd.PersistentFlags().GetString("config")
	return engine.LoadConfig(configFile)
}

// watch is watching database and calls UpdateRecommends when necessary.
func watch(config engine.TomlConfig, metaData toml.MetaData) {
	log.Println("start model updater")
	for {
		// Get status
		stat, err := status()
		if err != nil {
			log.Fatal(err)
		}
		// Compare
		if stat.FeedbackCount-stat.FeedbackCommit > config.Recommend.UpdateThreshold ||
			stat.IgnoreCount-stat.IgnoreCommit > config.Recommend.CacheSize/2 {
			log.Printf("current count (%v) - commit (%v) > threshold (%v), start to update recommends\n",
				stat.FeedbackCount, stat.FeedbackCommit, config.Recommend.UpdateThreshold)
			if err = engine.Update(config, metaData, db); err != nil {
				log.Fatal(err)
			}
			if err = db.SetMeta("feedback_commit", strconv.Itoa(stat.FeedbackCount)); err != nil {
				log.Fatal(err)
			}
			if err = db.SetMeta("ignore_commit", strconv.Itoa(stat.IgnoreCount)); err != nil {
				log.Fatal(err)
			}
			t := time.Now()
			if err = db.SetMeta("commit_time", t.String()); err != nil {
				log.Fatal(err)
			}
			log.Printf("recommends update-to-date, commit = %v", stat.FeedbackCount)
		}
		// Sleep
		time.Sleep(time.Duration(config.Recommend.CheckPeriod) * time.Minute)
	}
}
