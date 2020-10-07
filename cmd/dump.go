package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/database"
	"log"
	"time"
)

var commandImportFeedback = &cobra.Command{
	Use:   "import-feedback [database_file] [csv_file]",
	Short: "Import feedback from CSV",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		databaseFile := args[0]
		csvFile := args[1]
		sep, _ := cmd.PersistentFlags().GetString("sep")
		header, _ := cmd.PersistentFlags().GetBool("header")
		// Connect database
		db, err := database.Open(databaseFile)
		if err != nil {
			log.Fatal(err)
		}
		// Import feedback
		printCount(db)
		log.Printf("import feedback from %s\n", csvFile)
		start := time.Now()
		if err = db.LoadFeedbackFromCSV(csvFile, sep, header); err != nil {
			log.Fatal(err)
		}
		elapsed := time.Since(start)
		printCount(db)
		log.Printf("feedback are imported successfully! (%v)\n", elapsed)
	},
}

var commandImportItems = &cobra.Command{
	Use:   "import-items [database_file] [csv_file]",
	Short: "Import items from CSV",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		databaseFile := args[0]
		csvFile := args[1]
		sep, _ := cmd.PersistentFlags().GetString("sep")
		header, _ := cmd.PersistentFlags().GetBool("header")
		timestampColumn, _ := cmd.PersistentFlags().GetInt("timestamp")
		labelColumn, _ := cmd.PersistentFlags().GetInt("label")
		labelSep, _ := cmd.PersistentFlags().GetString("label-sep")
		// Connect database
		db, err := database.Open(databaseFile)
		if err != nil {
			log.Fatal(err)
		}
		// Import feedback
		printCount(db)
		log.Printf("import items from %s\n", csvFile)
		start := time.Now()
		if err = db.LoadItemsFromCSV(csvFile, sep, header, timestampColumn, labelSep, labelColumn); err != nil {
			log.Fatal(err)
		}
		elapsed := time.Since(start)
		printCount(db)
		log.Printf("items are imported successfully! (%v)\n", elapsed)
	},
}

var commandExportFeedback = &cobra.Command{
	Use:   "export-feedback [database_file] [csv_file]",
	Short: "Export feedback to CSV",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		databaseFile := args[0]
		csvFile := args[1]
		sep, _ := cmd.PersistentFlags().GetString("sep")
		header, _ := cmd.PersistentFlags().GetBool("header")
		// Connect database
		db, err := database.Open(databaseFile)
		if err != nil {
			log.Fatal(err)
		}
		// Import feedback
		log.Printf("export feedback to %s\n", csvFile)
		if err = db.SaveFeedbackToCSV(csvFile, sep, header); err != nil {
			log.Fatal(err)
		}
		log.Println("feedback are exported successfully!")
	},
}

var commandExportItems = &cobra.Command{
	Use:   "export-items [database_file] [csv_file]",
	Short: "Export items to CSV",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		databaseFile := args[0]
		csvFile := args[1]
		sep, _ := cmd.PersistentFlags().GetString("sep")
		header, _ := cmd.PersistentFlags().GetBool("header")
		timestamp, _ := cmd.PersistentFlags().GetBool("timestamp")
		labelColumn, _ := cmd.PersistentFlags().GetBool("label")
		labelSep, _ := cmd.PersistentFlags().GetString("label-sep")
		// Connect database
		db, err := database.Open(databaseFile)
		if err != nil {
			log.Fatal(err)
		}
		// Import feedback
		log.Printf("export items to %s\n", csvFile)
		if err = db.SaveItemsToCSV(csvFile, sep, header, timestamp, labelSep, labelColumn); err != nil {
			log.Fatal(err)
		}
		log.Println("items are exported successfully!")
	},
}

func init() {
	commands := []*cobra.Command{commandImportFeedback, commandImportItems, commandExportFeedback, commandExportItems}
	commandImportItems.PersistentFlags().IntP("timestamp", "t", 0, "specify the timestamp column")
	commandImportItems.PersistentFlags().IntP("label", "l", 0, "specify the label column")
	commandExportItems.PersistentFlags().BoolP("timestamp", "t", false, "export with timestamp")
	commandExportItems.PersistentFlags().BoolP("label", "l", false, "export with labels")
	for _, command := range commands {
		command.PersistentFlags().String("sep", ",", "set the separator for CSV files")
		command.PersistentFlags().String("label-sep", "|", "set the separator for labels")
		command.PersistentFlags().Bool("header", false, "set the header for CSV files")
		command.PersistentFlags().StringP("config", "c", "", "configure file")
	}
}

func printCount(db *database.Database) {
	// Count feedback
	nFeedback, err := db.CountFeedback()
	if err != nil {
		log.Fatal(err)
	}
	// Count items
	nItems, err := db.CountItems()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("database status: %v feedback, %v items", nFeedback, nItems)
}
