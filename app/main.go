package main

import (
	"github.com/spf13/cobra"
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
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
