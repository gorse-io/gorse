package dump

import "github.com/spf13/cobra"

var CmdDump = &cobra.Command{
	Use:   "dump",
	Short: "Import/export data from database",
	Run: func(cmd *cobra.Command, args []string) {

	},
}
