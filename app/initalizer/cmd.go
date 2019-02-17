package initalizer

import "github.com/spf13/cobra"

var CmdInit = &cobra.Command{
	Use:   "init",
	Short: "Initialize database for gorse",
	Run: func(cmd *cobra.Command, args []string) {

	},
}
