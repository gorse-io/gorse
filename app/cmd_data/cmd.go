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
		sep, _ := cmd.PersistentFlags().GetString("csv-sep")
		header, _ := cmd.PersistentFlags().GetBool("csv-header")
		// Load ratings
		if cmd.PersistentFlags().Changed("import-ratings") {
			name, _ := cmd.PersistentFlags().GetString("import-ratings")
			log.Printf("Import ratings from %s\n", name)
			if err = db.LoadRatingsFromCSV(name, sep, header); err != nil {
				log.Fatal(err)
			}
			log.Println("Ratings are imported successfully!")
		}
		// Load items
		if cmd.PersistentFlags().Changed("import-items") {
			name, _ := cmd.PersistentFlags().GetString("import-items")
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
	CmdData.PersistentFlags().String("import-ratings", "", "import ratings from CSV file")
	CmdData.PersistentFlags().String("import-items", "", "import items from CSV file")
	CmdData.PersistentFlags().String("csv-sep", "\t", "import CSV file with separator")
	CmdData.PersistentFlags().Bool("csv-header", false, "import CSV file with header")
}
