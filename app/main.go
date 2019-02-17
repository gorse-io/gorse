package main

import (
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/app/dump"
	"github.com/zhenghaoz/gorse/app/initalizer"
	"github.com/zhenghaoz/gorse/app/server"
	"github.com/zhenghaoz/gorse/app/test"
	"log"
)

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

func main() {
	rootCmd.AddCommand(test.CmdTest)
	rootCmd.AddCommand(initalizer.CmdInit)
	rootCmd.AddCommand(dump.CmdDump)
	rootCmd.AddCommand(server.CmdServer)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
