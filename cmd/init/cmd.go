package init

import (
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/cmd/engine"
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
		db, err := engine.NewDatabaseConnection(databaseDriver, dataSource)
		if err != nil {
			log.Fatal(err)
		}
		if err = db.Init(); err != nil {
			log.Fatal(err)
		}
		log.Println("Database was initialized successfully.")
	},
}

func init() {
	CmdInit.PersistentFlags().String("driver", "mysql", "database driver")
}
