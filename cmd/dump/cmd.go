package dump

import (
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/engine"
	"log"
)

var CmdDump = &cobra.Command{
	Use:   "dump [database]",
	Short: "Import/export data from database",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		databaseFile := args[0]
		// Connect database
		db, err := engine.Open(databaseFile)
		if err != nil {
			log.Fatal(err)
		}
		// Import ratings
		if cmd.PersistentFlags().Changed("import-feedback-csv") {
			name, _ := cmd.PersistentFlags().GetString("import-feedback-csv")
			sep, _ := cmd.PersistentFlags().GetString("feedback-csv-sep")
			header, _ := cmd.PersistentFlags().GetBool("feedback-csv-header")
			log.Printf("Import feedback from %s\n", name)
			if err = db.LoadFeedbackFromCSV(name, sep, header); err != nil {
				log.Fatal(err)
			}
			log.Println("Feedback are imported successfully!")
		}
		// Import items
		if cmd.PersistentFlags().Changed("import-items-csv") {
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
	CmdDump.PersistentFlags().String("import-feedback-csv", "", "import feedback from CSV file")
	CmdDump.PersistentFlags().String("feedback-csv-sep", "\t", "import feedback from CSV file with separator")
	CmdDump.PersistentFlags().Bool("feedback-csv-header", false, "import feedback from CSV file with header")
	CmdDump.PersistentFlags().String("import-items-csv", "", "import items from CSV file")
	CmdDump.PersistentFlags().String("items-csv-sep", "\t", "import items from CSV file with separator")
	CmdDump.PersistentFlags().Bool("items-csv-header", false, "import items from CSV file with header")
}
