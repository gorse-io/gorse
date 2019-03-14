package main

import (
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/app/cv"
	"github.com/zhenghaoz/gorse/app/data"
	"github.com/zhenghaoz/gorse/app/init_tool"
	"github.com/zhenghaoz/gorse/app/serve"
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
	rootCmd.AddCommand(cv.CmdTest)
	rootCmd.AddCommand(init_tool.CmdInit)
	rootCmd.AddCommand(data.CmdDump)
	rootCmd.AddCommand(serve.CmdServer)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
