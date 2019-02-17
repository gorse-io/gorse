package dump

import (
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
	"log"
)

var CmdDump = &cobra.Command{
	Use:   "dump [data source]",
	Short: "Import/export data from database",
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
			fmt.Printf("Import data from %s\n", name)
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
	CmdDump.PersistentFlags().String("driver", "mysql", "Database driver")
	CmdDump.PersistentFlags().String("import-csv", "", "import data from CSV file")
	//CmdDump.PersistentFlags().String("csv-sep", "\t", "import CSV file with separator")
	//CmdDump.PersistentFlags().Bool("csv-header", false, "import CSV file with header")
}
