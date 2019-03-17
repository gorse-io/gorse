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

		fmt.Println("Database was initialized successfully.")
	},
}

func init() {
	CmdInit.PersistentFlags().String("driver", "mysql", "Database driver")
}
