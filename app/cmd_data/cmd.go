package cmd_data

import (
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/app/engine"
	"log"
)

var CmdData = &cobra.Command{
	Use:   "data [cmd_data source]",
	Short: "Import/export data from database",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dataSource := args[0]
		databaseDriver, _ := cmd.PersistentFlags().GetString("driver")
		// Connect database
		db, err := engine.NewDatabaseConnection(databaseDriver, dataSource)
		if err != nil {
			log.Fatal(err)
		}
		// Import ratings
		if cmd.PersistentFlags().Changed("import-ratings") {
			name, _ := cmd.PersistentFlags().GetString("import-ratings-csv")
			sep, _ := cmd.PersistentFlags().GetString("ratings-csv-sep")
			header, _ := cmd.PersistentFlags().GetBool("ratings-csv-header")
			log.Printf("Import ratings from %s\n", name)
			if err = db.LoadRatingsFromCSV(name, sep, header); err != nil {
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
			if err = db.LoadItemsFromCSV(name, sep, header); err != nil {
				log.Fatal(err)
			}
			log.Println("Items are imported successfully!")
		}
	},
}

func init() {
	CmdData.PersistentFlags().String("driver", "mysql", "database driver")
	CmdData.PersistentFlags().String("import-ratings-csv", "", "import ratings from CSV file")
	CmdData.PersistentFlags().String("ratings-csv-sep", "\t", "import ratings from CSV file with separator")
	CmdData.PersistentFlags().Bool("ratings-csv-header", false, "import ratings from CSV file with header")
	CmdData.PersistentFlags().String("import-items-csv", "", "import items from CSV file")
	CmdData.PersistentFlags().String("items-csv-sep", "\t", "import items from CSV file with separator")
	CmdData.PersistentFlags().Bool("items-csv-header", false, "import items from CSV file with header")
}
