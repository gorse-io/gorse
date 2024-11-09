package main

import (
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/base/log"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:   "gorse-cli",
	Short: "Gorse command line tool",
}

var dumpCmd = &cobra.Command{
	Use: "dump",
}

var restoreCmd = &cobra.Command{
	Use: "restore",
}

func init() {
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(restoreCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Logger().Fatal("failed to execute command", zap.Error(err))
	}
}
