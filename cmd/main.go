package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
)

var versionName = "0.1.3"

var rootCmd = &cobra.Command{
	Use:   "gorse",
	Short: "gorse: Go Recommender System Engine",
	Long:  "gorse is an offline recommender system backend based on collaborative filtering written in Go.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Check the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(versionName)
	},
}

// Main is the main entry of gorse.
func Main() {
	rootCmd.AddCommand(commandTest)
	rootCmd.AddCommand(commandImportFeedback)
	rootCmd.AddCommand(commandImportItems)
	rootCmd.AddCommand(commandExportFeedback)
	rootCmd.AddCommand(commandExportItems)
	rootCmd.AddCommand(commandServe)
	rootCmd.AddCommand(versionCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
