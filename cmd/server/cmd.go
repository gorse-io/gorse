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
	Use:   "server",
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
		// Start model daemon
		go Watcher(conf, metaData)
		Server(conf.Server)
	},
}

func init() {
	CmdServer.PersistentFlags().BoolP("init", "i", false, "Initialize empty database")
	CmdServer.PersistentFlags().StringP("config", "c", "", "Configure file")
	CmdServer.PersistentFlags().String("import-ratings-csv", "", "import ratings from CSV file")
	CmdServer.PersistentFlags().String("ratings-csv-sep", "\t", "import ratings from CSV file with separator")
	CmdServer.PersistentFlags().Bool("ratings-csv-header", false, "import ratings from CSV file with header")
	CmdServer.PersistentFlags().String("import-items-csv", "", "import items from CSV file")
	CmdServer.PersistentFlags().String("items-csv-sep", "\t", "import items from CSV file with separator")
	CmdServer.PersistentFlags().Bool("items-csv-header", false, "import items from CSV file with header")
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

func ImportData(cmd *cobra.Command, db *engine.DB) {
	// Import ratings
	if cmd.PersistentFlags().Changed("import-feedback") {
		name, _ := cmd.PersistentFlags().GetString("import-feedback-csv")
		sep, _ := cmd.PersistentFlags().GetString("feedback-csv-sep")
		header, _ := cmd.PersistentFlags().GetBool("feedback-csv-header")
		log.Printf("Import feedback from %s\n", name)
		if err := db.LoadFeedbackFromCSV(name, sep, header); err != nil {
			log.Fatal(err)
		}
		log.Println("Feedback are imported successfully!")
	}
	// Import items
	if cmd.PersistentFlags().Changed("import-items") {
		name, _ := cmd.PersistentFlags().GetString("import-items-csv")
		sep, _ := cmd.PersistentFlags().GetString("items-csv-sep")
		header, _ := cmd.PersistentFlags().GetBool("items-csv-header")
		log.Printf("Import items from %s\n", name)
		if err := db.LoadItemsFromCSV(name, sep, header); err != nil {
			log.Fatal(err)
		}
		log.Println("Items are imported successfully!")
	}
}

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
			engine.Update(config, metaData, db)
			if err = db.SetMeta("last_count", strconv.Itoa(count)); err != nil {
				log.Println(err)
			}
			log.Printf("recommends update-to-date, last_count = %v", count)
		}
		// Sleep
		time.Sleep(time.Duration(config.Recommend.CheckPeriod) * time.Minute)
	}
}
