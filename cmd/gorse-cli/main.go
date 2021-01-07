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
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/match"
	"log"
)

const versionName = "0.2"

func init() {
	cliCommand.AddCommand(importCommand)
	cliCommand.AddCommand(exportCommand)
	cliCommand.AddCommand(testCommand)
	cliCommand.AddCommand(tuneCommand)
	cliCommand.AddCommand(versionCommand)

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

	// export feedback
	exportCommand.AddCommand(exportFeedbackCommand)
	exportFeedbackCommand.PersistentFlags().String("sep", ",", "Separator for csv file.")
	exportFeedbackCommand.PersistentFlags().Bool("header", false, "Skip first line of csv file.")

	// export items
	exportCommand.AddCommand(exportItemCommand)
	exportItemCommand.PersistentFlags().String("sep", ",", "Separator for csv file.")
	exportItemCommand.PersistentFlags().Bool("header", false, "Skip first line of csv file.")
	exportItemCommand.PersistentFlags().String("label-sep", "|", "Separator for labels")

	// test model
	testCommand.PersistentFlags().String("load-builtin", "", "load data from built-in")
	testCommand.PersistentFlags().String("load-csv", "", "load data from CSV file")
	testCommand.PersistentFlags().String("load-database", "", "load data from built-in")
	testCommand.PersistentFlags().String("csv-sep", "\t", "load CSV file with separator")
	testCommand.PersistentFlags().String("csv-format", "", "load CSV file with header")
	testCommand.PersistentFlags().Bool("csv-header", false, "load CSV file with header")
	for _, paramFlag := range testParamFlags {
		testCommand.PersistentFlags().String(paramFlag.Name, "", paramFlag.Help)
	}
	testCommand.PersistentFlags().Int("verbose", 1, "Verbose period")
	testCommand.PersistentFlags().Int("jobs", 1, "Number of jobs for model fitting")
	testCommand.PersistentFlags().Int("top-k", 10, "Length of recommendation list")
	testCommand.PersistentFlags().Int("n-negatives", 100, "Number of negative samples")
	testCommand.PersistentFlags().Int("n-test-users", 0, "Number of users for sampled test set")

	// tune model
	tuneCommand.PersistentFlags().String("load-builtin", "", "load data from built-in")
	tuneCommand.PersistentFlags().String("load-csv", "", "load data from CSV file")
	tuneCommand.PersistentFlags().String("load-database", "", "load data from built-in")
	tuneCommand.PersistentFlags().String("csv-sep", "\t", "load CSV file with separator")
	tuneCommand.PersistentFlags().String("csv-format", "", "load CSV file with header")
	tuneCommand.PersistentFlags().Bool("csv-header", false, "load CSV file with header")
	for _, paramFlag := range testParamFlags {
		tuneCommand.PersistentFlags().String(paramFlag.Name, "", paramFlag.Help)
	}
	tuneCommand.PersistentFlags().Int("verbose", 1, "Verbose period")
	tuneCommand.PersistentFlags().Int("jobs", 1, "Number of jobs for model fitting")
	tuneCommand.PersistentFlags().Int("top-k", 10, "Length of recommendation list")
	tuneCommand.PersistentFlags().Int("n-negatives", 0, "Number of users for sampled test set")
	tuneCommand.PersistentFlags().Int("n-test-users", 0, "Number of users for sampled test set")
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
	Use:   "import",
	Short: "Import data into gorse",
}

var exportCommand = &cobra.Command{
	Use:   "export",
	Short: "Export data from gorse",
}

var testCommand = &cobra.Command{
	Use:   "test",
	Short: "Test recommendation model by user-leave-one-out",
	Args:  cobra.ExactArgs(1),
	Run:   test,
}

var tuneCommand = &cobra.Command{
	Use:   "tune",
	Short: "Tune recommendation model by random search",
	Args:  cobra.ExactArgs(1),
	Run:   tune,
}

var importItemCommand = &cobra.Command{
	Use:   "items CSV_FILE",
	Short: "Import items from csv file into gorse",
	Args:  cobra.ExactArgs(1),
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

var exportItemCommand = &cobra.Command{
	Use:   "items CSV_FILE",
	Short: "Export items from gorse into csv file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		csvFile := args[0]
		sep, _ := cmd.PersistentFlags().GetString("sep")
		header, _ := cmd.PersistentFlags().GetBool("header")
		labelSep, _ := cmd.PersistentFlags().GetString("label-sep")
		config, _, err := config.LoadConfig("/home/zhenghaoz/.gorse/cli.toml")
		if err != nil {
			log.Fatal(err)
		}
		exportItems(csvFile, sep, labelSep, header, config)
	},
}

var importFeedbackCommand = &cobra.Command{
	Use:   "feedback CSV_FILE",
	Short: "Import feedback from csv file into gorse",
	Args:  cobra.ExactArgs(1),
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

var exportFeedbackCommand = &cobra.Command{
	Use:   "feedback CSV_FILE",
	Short: "Export feedback from gorse into csv file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Read ags and flags
		csvFile := args[0]
		sep, _ := cmd.PersistentFlags().GetString("sep")
		header, _ := cmd.PersistentFlags().GetBool("header")
		config, _, err := config.LoadConfig(match.GorseDir + "/cli.toml")
		if err != nil {
			log.Fatal(err)
		}
		exportFeedback(csvFile, sep, header, config)
	},
}

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Check the version of gorse",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(versionName)
	},
}
