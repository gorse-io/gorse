// Copyright 2020 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/config"
	"log"
)

func init() {
	cliCommand.AddCommand(importCommand)
	cliCommand.AddCommand(testCommand)
	cliCommand.AddCommand(tuneCommand)
	// import feedback
	importCommand.AddCommand(importFeedbackCommand)
	importFeedbackCommand.PersistentFlags().String("sep", ",", "Separator for csv file.")
	importFeedbackCommand.PersistentFlags().Bool("header", false, "Skip first line of csv file.")
	importFeedbackCommand.PersistentFlags().String("format", "ui", "Columns of csv file "+
		"(u - user, i - item).")
	// import items
	importCommand.AddCommand(importItemCommand)
	importItemCommand.PersistentFlags().String("sep", ",", "Separator for csv file.")
	importItemCommand.PersistentFlags().Bool("header", false, "Skip first line of csv file.")
	importItemCommand.PersistentFlags().String("label-sep", "|", "Separator for labels")
	importItemCommand.PersistentFlags().String("format", "itu", "Columns of csv file "+
		"(u - user, i - item, t - timestamp, l - labels, _ - meaningless).")
}

func main() {
	if err := cliCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}

var cliCommand = &cobra.Command{
	Use:   "gorse-cli",
	Short: "CLI for gorse recommender system",
}

var importCommand = &cobra.Command{
	Use: "import",
}

var testCommand = &cobra.Command{
	Use:  "test",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var tuneCommand = &cobra.Command{
	Use:  "tune",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var importItemCommand = &cobra.Command{
	Use:  "items CSV_FILE",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		csvFile := args[0]
		sep, _ := cmd.PersistentFlags().GetString("sep")
		header, _ := cmd.PersistentFlags().GetBool("header")
		format, _ := cmd.PersistentFlags().GetString("format")
		labelSep, _ := cmd.PersistentFlags().GetString("label-sep")
		config, _, err := config.LoadConfig("/home/zhenghaoz/.gorse/cli.toml")
		if err != nil {
			log.Fatal(err)
		}
		importItems(csvFile, sep, labelSep, header, format, config)
	},
}

var importFeedbackCommand = &cobra.Command{
	Use:  "feedback CSV_FILE",
	Long: "Import feedback from csv file.",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Read ags and flags
		csvFile := args[0]
		sep, _ := cmd.PersistentFlags().GetString("sep")
		header, _ := cmd.PersistentFlags().GetBool("header")
		format, _ := cmd.PersistentFlags().GetString("format")
		config, _, err := config.LoadConfig("/home/zhenghaoz/.gorse/cli.toml")
		if err != nil {
			log.Fatal(err)
		}
		importFeedback(csvFile, sep, header, format, config)
	},
}
