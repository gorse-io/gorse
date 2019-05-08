package serve

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/engine"
	"log"
	"strconv"
	"time"
)

var db *engine.DB

var CmdServer = &cobra.Command{
	Use:   "serve",
	Short: "Start a recommender sever",
	Run: func(cmd *cobra.Command, args []string) {
		// Print welcome
		Welcome()
		// Load configuration
		conf, metaData := LoadConfig(cmd)
		// Connect database
		var err error
		db, err = engine.Open(conf.Database.File)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("connect to database: %v", conf.Database.File)
		// Start model daemon
		go Watcher(conf, metaData)
		Server(conf.Server)
	},
}

func init() {
	CmdServer.PersistentFlags().StringP("config", "c", "", "Configure file")
}

func Welcome() {
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

func LoadConfig(cmd *cobra.Command) (engine.TomlConfig, toml.MetaData) {
	if !cmd.PersistentFlags().Changed("config") {
		log.Fatal("please use specify a configuration")
	}
	configFile, _ := cmd.PersistentFlags().GetString("config")
	return engine.LoadConfig(configFile)
}

// Watcher is watching database and calls UpdateRecommends when necessary.
func Watcher(config engine.TomlConfig, metaData toml.MetaData) {
	log.Println("start model updater")
	for {
		// Count ratings
		count, err := db.CountFeedback()
		if err != nil {
			log.Fatal(err)
		}
		// Get commit ratings
		lastCountString, err := db.GetMeta("commit")
		if err != nil {
			log.Fatal(err)
		}
		lastCount, err := strconv.Atoi(lastCountString)
		if lastCountString == "" {
			lastCount = 0
		} else if err != nil {
			log.Fatal(err)
		}
		log.Printf("current number of feedback: %v, commit: %v\n", count, lastCount)
		// Compare
		if count-lastCount > config.Recommend.UpdateThreshold {
			log.Printf("current count (%v) - commit (%v) > threshold (%v), start to update recommends\n",
				count, lastCount, config.Recommend.UpdateThreshold)
			if err = engine.Update(config, metaData, db); err != nil {
				log.Fatal(err)
			}
			if err = db.SetMeta("commit", strconv.Itoa(count)); err != nil {
				log.Fatal(err)
			}
			log.Printf("recommends update-to-date, commit = %v", count)
		}
		// Sleep
		time.Sleep(time.Duration(config.Recommend.CheckPeriod) * time.Minute)
	}
}
