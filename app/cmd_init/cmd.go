package cmd_init

import (
	"database/sql"
	"fmt"
	"github.com/spf13/cobra"
	"log"
)

var CmdInit = &cobra.Command{
	Use:   "init [data source]",
	Short: "Initialize database for gorse",
	Long:  "Initialize database for gorse.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dataSource := args[0]
		databaseDriver, _ := cmd.PersistentFlags().GetString("driver")
		// Connect database
		db, err := sql.Open(databaseDriver, dataSource)
		if err != nil {
			log.Fatal(err)
		}
		// Create ratings table
		query := "CREATE TABLE ratings (" +
			"user_id int NOT NULL, " +
			"item_id int NOT NULL, " +
			"rating int NOT NULL, " +
			"UNIQUE KEY unique_index (user_id,item_id)" +
			") ENGINE=InnoDB DEFAULT CHARSET=latin1"
		_, err = db.Exec(query)
		if err != nil {
			log.Fatal(err)
		}
		// Create recommends table
		query = "CREATE TABLE recommends (" +
			"user_id int NOT NULL, " +
			"item_id int NOT NULL, " +
			"rating double NOT NULL, " +
			"era int NOT NULL, " +
			"UNIQUE KEY unique_index (user_id,item_id)" +
			") ENGINE=InnoDB DEFAULT CHARSET=latin1"
		_, err = db.Exec(query)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Database was initialized successfully.")
	},
}

func init() {
	CmdInit.PersistentFlags().String("driver", "mysql", "Database driver")
}
