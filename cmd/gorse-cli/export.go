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
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"os"
	"strings"
)

func init() {
	cliCommand.AddCommand(exportCommand)
	// export feedback
	exportCommand.AddCommand(exportFeedbackCommand)
	exportFeedbackCommand.PersistentFlags().StringP("type", "t", "", "Set feedback type.")
	exportFeedbackCommand.PersistentFlags().IntP("batch-size", "b", 1024, "Batch size of reading data.")
	exportFeedbackCommand.PersistentFlags().StringP("sep", "s", ",", "Separator for csv file.")
	exportFeedbackCommand.PersistentFlags().BoolP("header", "H", false, "Print header.")
	// export items
	exportCommand.AddCommand(exportItemCommand)
	exportItemCommand.PersistentFlags().IntP("batch-size", "b", 1024, "Batch size of reading data.")
	exportItemCommand.PersistentFlags().StringP("sep", "s", ",", "Separator for csv file.")
	exportItemCommand.PersistentFlags().BoolP("header", "H", false, "Print header.")
	exportItemCommand.PersistentFlags().StringP("label-sep", "l", "|", "Separator for labels")
}

var exportCommand = &cobra.Command{
	Use:   "export",
	Short: "export data",
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
		batchSize, _ := cmd.PersistentFlags().GetInt("batch-size")
		exportItems(csvFile, sep, labelSep, header, batchSize)
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
		batchSize, _ := cmd.PersistentFlags().GetInt("batch-size")
		var feedbackType *string
		if cmd.PersistentFlags().Changed("type") {
			temp, _ := cmd.PersistentFlags().GetString("type")
			feedbackType = &temp
		}
		exportFeedback(csvFile, feedbackType, sep, header, batchSize)
	},
}

func exportFeedback(csvFile string, feedbackType *string, sep string, printHeader bool, batchSize int) {
	// Open database
	database, err := data.Open(globalConfig.Database.DataStore)
	if err != nil {
		base.Logger().Fatal("failed to connect database", zap.Error(err))
	}
	defer database.Close()
	// Open file
	file, err := os.Create(csvFile)
	if err != nil {
		base.Logger().Fatal("failed to create file", zap.Error(err))
	}
	defer file.Close()
	// Export feedbacks
	if printHeader {
		if _, err = file.WriteString(fmt.Sprintf("feedback_type%vuser_id%vitem_id%vtime_stamp\n",
			sep, sep, sep)); err != nil {
			base.Logger().Fatal("failed to write file", zap.Error(err))
		}
	}
	cursor := ""
	for {
		var feedback []data.Feedback
		cursor, feedback, err = database.GetFeedback(cursor, batchSize, feedbackType)
		if err != nil {
			base.Logger().Fatal("failed to get feedback", zap.Error(err))
		}
		for _, v := range feedback {
			if _, err = file.WriteString(fmt.Sprintf("%v%v%v%v%v%v%v\n",
				v.FeedbackType, sep, v.UserId, sep, v.ItemId, sep, v.Timestamp)); err != nil {
				base.Logger().Fatal("failed to write line", zap.Error(err))
			}
		}
		if cursor == "" {
			break
		}
	}
}

func exportItems(csvFile string, sep string, labelSep string, printHeader bool, batchSize int) {
	// Open database
	database, err := data.Open(globalConfig.Database.DataStore)
	if err != nil {
		base.Logger().Fatal("failed to connect database", zap.Error(err))
	}
	defer database.Close()
	// Open file
	file, err := os.Create(csvFile)
	if err != nil {
		base.Logger().Fatal("failed to create file", zap.Error(err))
	}
	defer file.Close()
	// Print header
	if printHeader {
		if _, err = file.WriteString(fmt.Sprintf("item_id%vtime_stamp%vlabels", sep, sep)); err != nil {
			base.Logger().Fatal("failed to write file", zap.Error(err))
		}
	}
	// Export items
	cursor := ""
	for {
		var items []data.Item
		cursor, items, err = database.GetItems(cursor, batchSize)
		if err != nil {
			base.Logger().Fatal("failed to get items", zap.Error(err))
		}
		for _, item := range items {
			if _, err = file.WriteString(fmt.Sprintf("%v%v%v%v%v\n",
				item.ItemId, sep, item.Timestamp, sep, strings.Join(item.Labels, labelSep))); err != nil {
				base.Logger().Fatal("failed to write line", zap.Error(err))
			}
		}
		if cursor == "" {
			break
		}
	}
}
