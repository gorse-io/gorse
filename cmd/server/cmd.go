package serve

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/cmd/engine"
	"log"
)

var db engine.Database

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
		db, err = engine.NewDatabaseConnection(conf.Database.Driver, conf.Database.Access)
		if err != nil {
			log.Fatal(err)
		}
		// Start model daemon
		go ModelDaemon(conf, metaData)
		//go ModelDaemon(conf, metaData)
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

func LoadConfig(cmd *cobra.Command) (TomlConfig, toml.MetaData) {
	if !cmd.PersistentFlags().Changed("config") {
		log.Fatal("please use specify a configuration")
	}
	configFile, _ := cmd.PersistentFlags().GetString("config")
	var conf TomlConfig
	metaData, err := toml.DecodeFile(configFile, &conf)
	if err != nil {
		log.Fatal(err)
	}
	return conf, metaData
}

func InitDB(cmd *cobra.Command, db *engine.Database) {
	val, err := cmd.PersistentFlags().GetBool("init")
	if err != nil {
		log.Fatal(err)
	}
	if val {
		if err := db.Init(); err != nil {
			log.Fatal(err)
		}
		log.Println("Database was initialized successfully.")
	}
}

func ImportData(cmd *cobra.Command, db *engine.Database) {
	// Import ratings
	if cmd.PersistentFlags().Changed("import-ratings") {
		name, _ := cmd.PersistentFlags().GetString("import-ratings-csv")
		sep, _ := cmd.PersistentFlags().GetString("ratings-csv-sep")
		header, _ := cmd.PersistentFlags().GetBool("ratings-csv-header")
		log.Printf("Import ratings from %s\n", name)
		if err := db.LoadRatingsFromCSV(name, sep, header); err != nil {
			log.Fatal(err)
		}
		log.Println("Ratings are imported successfully!")
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
