package cmd

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/database"
	"github.com/zhenghaoz/gorse/engine"
	"github.com/zhenghaoz/gorse/server"
	"log"
)

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
		db, err := database.Open(conf.Database.Path)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("connect to database: %v", conf.Database.Path)
		// Start model daemon
		instance := server.Instance{DB: db, Config: &conf, MetaData: &metaData}
		go engine.Main(&instance)
		server.Main(&instance)
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

func loadConfig(cmd *cobra.Command) (config.Config, toml.MetaData) {
	if !cmd.PersistentFlags().Changed("config") {
		log.Fatal("please use specify a configuration")
	}
	configFile, _ := cmd.PersistentFlags().GetString("config")
	log.Printf("load configuration from %v", configFile)
	return config.LoadConfig(configFile)
}
