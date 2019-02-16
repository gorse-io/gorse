package main

import (
	"fmt"
	"github.com/spf13/cobra"
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

var cmdTest = &cobra.Command{
	Use:   "test",
	Short: "Test a model with cross validation",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Hugo Static Site Generator v0.9 -- HEAD")
	},
}

func main() {
	rootCmd.AddCommand(cmdTest)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
