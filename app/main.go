package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/app/cmd_cv"
	"github.com/zhenghaoz/gorse/app/cmd_data"
	"github.com/zhenghaoz/gorse/app/cmd_init"
	"github.com/zhenghaoz/gorse/app/cmd_serve"
	"github.com/zhenghaoz/gorse/app/engine"
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
	rootCmd.AddCommand(cmd_cv.CmdTest)
	rootCmd.AddCommand(cmd_init.CmdInit)
	rootCmd.AddCommand(cmd_data.CmdData)
	rootCmd.AddCommand(cmd_serve.CmdServer)
	rootCmd.AddCommand(VersionCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
