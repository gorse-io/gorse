package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/cmd/dump"
	"github.com/zhenghaoz/gorse/cmd/serve"
	"github.com/zhenghaoz/gorse/cmd/test"
	"log"
)

var VersionName = 0.1

var rootCmd = &cobra.Command{
	Use:   "gorse",
	Short: "gorse: Go Recommender System Engine",
	Long:  "A high performance recommender system engine in Go",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
		}
	},
}

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Check the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(VersionName)
	},
}

func main() {
	rootCmd.AddCommand(test.CmdTest)
	rootCmd.AddCommand(dump.CmdImportFeedback)
	rootCmd.AddCommand(dump.CmdImportItems)
	rootCmd.AddCommand(dump.CmdExportFeedback)
	rootCmd.AddCommand(dump.CmdExportItems)
	rootCmd.AddCommand(serve.CmdServer)
	rootCmd.AddCommand(VersionCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
