package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/cmd/cv"
	"github.com/zhenghaoz/gorse/cmd/data"
	"github.com/zhenghaoz/gorse/cmd/engine"
	"github.com/zhenghaoz/gorse/cmd/init"
	"github.com/zhenghaoz/gorse/cmd/server"
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

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Check the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(engine.VersionName)
	},
}

func main() {
	rootCmd.AddCommand(cv.CmdTest)
	rootCmd.AddCommand(init.CmdInit)
	rootCmd.AddCommand(data.CmdData)
	rootCmd.AddCommand(serve.CmdServer)
	rootCmd.AddCommand(VersionCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
