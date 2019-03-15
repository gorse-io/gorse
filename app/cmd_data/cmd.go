package cmd_data

import (
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
	"log"
)

var CmdData = &cobra.Command{
	Use:   "dump [cmd_data source]",
	Short: "Import/export cmd_data from database",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dataSource := args[0]
		databaseDriver, _ := cmd.PersistentFlags().GetString("driver")
		// Connect database
		db, err := sql.Open(databaseDriver, dataSource)
		if err != nil {
			log.Fatal(err)
		}
		if cmd.PersistentFlags().Changed("import-csv") {
			name, _ := cmd.PersistentFlags().GetString("import-csv")
			//sep, _ := cmd.PersistentFlags().GetString("csv-sep")
			//header, _ := cmd.PersistentFlags().GetBool("csv-header")
			fmt.Printf("Import cmd_data from %s\n", name)
			mysql.RegisterLocalFile(name)
			_, err := db.Exec("LOAD DATA LOCAL INFILE '" + name + "' INTO TABLE ratings")
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("Data imported succesfully!")
		}
	},
}

func init() {
	CmdData.PersistentFlags().String("driver", "mysql", "Database driver")
	CmdData.PersistentFlags().String("import-csv", "", "import cmd_data from CSV file")
	//CmdData.PersistentFlags().String("csv-sep", "\t", "import CSV file with separator")
	//CmdData.PersistentFlags().Bool("csv-header", false, "import CSV file with header")
}
