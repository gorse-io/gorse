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
	"bufio"
	"fmt"
	"github.com/araddon/dateparse"
	"github.com/cheggaaa/pb/v3"
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/storage/data"
	"os"
	"strings"
)

func init() {
	cliCommand.AddCommand(importCommand)
	// import items
	importCommand.AddCommand(importFeedbackCommand)
	importItemCommand.PersistentFlags().BoolP("yes", "y", false, "Skip import preview.")
	importItemCommand.PersistentFlags().StringP("sep", "s", ",", "Separator for csv file.")
	importItemCommand.PersistentFlags().BoolP("header", "H", false, "Skip first line of csv file.")
	importItemCommand.PersistentFlags().StringP("label-sep", "l", "|", "Separator for labels")
	importItemCommand.PersistentFlags().StringP("format", "f", "itu", "Columns of csv file "+
		"(u - user, i - item, t - timestamp, l - labels, _ - meaningless).")
	// import feedback
	importCommand.AddCommand(importItemCommand)
	importFeedbackCommand.PersistentFlags().BoolP("yes", "y", false, "Skip import preview.")
	importFeedbackCommand.PersistentFlags().StringP("type", "t", "", "Set feedback type.")
	importFeedbackCommand.PersistentFlags().StringP("sep", "s", ",", "Separator for csv file.")
	importFeedbackCommand.PersistentFlags().BoolP("header", "H", false, "Skip first line of csv file.")
	importFeedbackCommand.PersistentFlags().StringP("format", "f", "uit", "Columns of csv file "+
		"(u - user, i - item, t - timestamp, _ - meaningless).")
}

var importCommand = &cobra.Command{
	Use:   "import",
	Short: "Import data into database.",
}

var importItemCommand = &cobra.Command{
	Use:   "items CSV_FILE",
	Short: "Import items from csv file into gorse",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		csvFile := args[0]
		skipPreview, _ := cmd.PersistentFlags().GetBool("yes")
		sep, _ := cmd.PersistentFlags().GetString("sep")
		header, _ := cmd.PersistentFlags().GetBool("header")
		formatString, _ := cmd.PersistentFlags().GetString("format")
		labelSep, _ := cmd.PersistentFlags().GetString("label-sep")
		if !skipPreview {
			if ok := previewImportItems(csvFile, sep, labelSep, header, formatString); !ok {
				return
			}
		}
		importItems(csvFile, sep, labelSep, header, formatString)
	},
}

func previewImportItems(csvFile string, sep string, labelSep string, hasHeader bool, fmtString string) bool {
	// Open file
	file, err := os.Open(csvFile)
	if err != nil {
		log.Fatalf("cli: failed to open file (%v)", err)
	}
	defer file.Close()
	// read lines
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"item_id", "timestamp", "label"})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// skip header
		if hasHeader {
			continue
		}
		// split fields
		splits := strings.Split(line, sep)
		splits = format(fmtString, "itl", splits)
		// parse item id
		if splits[0] == "" {
			log.Fatal("cli: invalid item id")
		}
		item := data.Item{ItemId: splits[0]}
		if splits[1] != "" {
			item.Timestamp, err = dateparse.ParseAny(splits[1])
			if err != nil {
				log.Fatalf("cli: failed to parse datetime at line %v (%v)", table.NumLines(), err)
			}
		}
		if splits[2] != "" {
			item.Labels = strings.Split(splits[2], labelSep)
		}
		if err != nil {
			log.Fatal(err)
		}
		// preview first 5 lines
		table.Append([]string{
			item.ItemId,
			fmt.Sprintf("%v", item.Timestamp),
			fmt.Sprintf("%v", item.Labels),
		})
		if table.NumLines() > 5 {
			break
		}
	}
	table.Render()
	fmt.Print("Import items to database? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	if strings.HasPrefix(strings.ToUpper(text), "N") {
		return false
	}
	return true
}

func importItems(csvFile string, sep string, labelSep string, hasHeader bool, fmt string) {
	// Get file size
	info, err := os.Stat(csvFile)
	if err != nil {
		log.Fatal(err)
	}
	length := info.Size()
	// Open file
	file, err := os.Open(csvFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// Open database
	database, err := data.Open(globalConfig.Database.DataStore)
	if err != nil {
		log.Fatalf("cli: failed to connect database (%v)", err)
	}
	err = database.Init()
	if err != nil {
		log.Fatalf("cli: failed to init database (%v)", err)
	}
	defer database.Close()
	// Read lines
	scanner := bufio.NewScanner(file)
	bar := pb.StartNew(int(length))
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		// skip header
		if hasHeader {
			continue
		}
		splits := strings.Split(line, sep)
		splits = format(fmt, "itl", splits)
		if splits[0] == "" {
			log.Fatalf("cli: failed to open file (%v)", err)
		}
		item := data.Item{ItemId: splits[0]}
		if splits[1] != "" {
			item.Timestamp, err = dateparse.ParseAny(splits[1])
			if err != nil {
				log.Fatalf("cli: failed to parse datetime at line %v (%v)", lineCount, err)
			}
		}
		if splits[2] != "" {
			item.Labels = strings.Split(splits[2], labelSep)
		}
		err := database.InsertItem(item)
		if err != nil {
			log.Fatal(err)
		}
		bar.Add(len(line) + 1)
		lineCount++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	bar.Finish()
}

var importFeedbackCommand = &cobra.Command{
	Use:   "feedback CSV_FILE",
	Short: "Import feedback from csv file into gorse",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Read ags and flags
		csvFile := args[0]
		skipPreview, _ := cmd.PersistentFlags().GetBool("yes")
		sep, _ := cmd.PersistentFlags().GetString("sep")
		header, _ := cmd.PersistentFlags().GetBool("header")
		fmtString, _ := cmd.PersistentFlags().GetString("format")
		feedbackType, _ := cmd.PersistentFlags().GetString("type")
		if !skipPreview {
			if ok := previewImportFeedback(csvFile, feedbackType, sep, header, fmtString); !ok {
				return
			}
		}
		importFeedback(csvFile, feedbackType, sep, header, fmtString)
	},
}

func previewImportFeedback(csvFile string, feedbackType string, sep string, hasHeader bool, fmtString string) bool {
	// Open file
	file, err := os.Open(csvFile)
	if err != nil {
		log.Fatalf("cli: failed to open file (%v)", err)
	}
	defer file.Close()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"type", "user_id", "item_id", "timestamp"})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if hasHeader {
			continue
		}
		splits := strings.Split(line, sep)
		splits = format(fmtString, "uit", splits)
		if splits[0] == "" {
			log.Fatalf("cli: invalid user id at line %v", table.NumLines())
		}
		if splits[1] == "" {
			log.Fatalf("cli: invalid item id at line %v", table.NumLines())
		}
		feedback := data.Feedback{FeedbackKey: data.FeedbackKey{FeedbackType: feedbackType, UserId: splits[0], ItemId: splits[1]}}
		feedback.Timestamp, err = dateparse.ParseAny(splits[2])
		if err != nil {
			log.Fatalf("cli: failed to parse datetime at line %v (%v)", table.NumLines(), err)
		}
		// preview first 5 lines
		table.Append([]string{
			feedback.FeedbackType,
			feedback.UserId,
			feedback.ItemId,
			fmt.Sprintf("%v", feedback.Timestamp),
		})
		if table.NumLines() > 5 {
			break
		}
	}
	table.Render()

	fmt.Printf("Import feedback into database (type = %v, auto_insert_user = %v, auto_insert_item = %v) [Y/n] ",
		feedbackType, globalConfig.Common.AutoInsertUser, globalConfig.Common.AutoInsertItem)
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	if strings.HasPrefix(strings.ToUpper(text), "N") {
		return false
	}
	return true
}

func importFeedback(csvFile, feedbackType string, sep string, hasHeader bool, fmtString string) {
	// Get file size
	info, err := os.Stat(csvFile)
	if err != nil {
		log.Fatal(err)
	}
	length := info.Size()
	// Open file
	file, err := os.Open(csvFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// Open database
	database, err := data.Open(globalConfig.Database.DataStore)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	// Read lines
	scanner := bufio.NewScanner(file)
	bar := pb.StartNew(int(length))
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if hasHeader {
			continue
		}
		splits := strings.Split(line, sep)
		splits = format(fmtString, "uit", splits)
		if splits[0] == "" {
			log.Fatalf("cli: invalid user id at line %v", lineCount)
		}
		if splits[1] == "" {
			log.Fatalf("cli: invalid item id at line %v", lineCount)
		}
		feedback := data.Feedback{FeedbackKey: data.FeedbackKey{FeedbackType: feedbackType, UserId: splits[0], ItemId: splits[1]}}
		feedback.Timestamp, err = dateparse.ParseAny(splits[2])
		if err != nil {
			log.Fatalf("cli: failed to parse datetime at line %v (%v)", lineCount, err)
		}
		err := database.InsertFeedback(feedback, globalConfig.Common.AutoInsertUser, globalConfig.Common.AutoInsertItem)
		if err != nil {
			log.Fatal(err)
		}
		bar.Add(len(line) + 1)
		lineCount++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	bar.Finish()
}

func format(inFmt string, outFmt string, s []string) []string {
	if len(s) < len(inFmt) {
		log.Fatalf("Expect %d fields, get %d", len(inFmt), len(s))
	}
	if inFmt == outFmt {
		return s
	}
	pool := make(map[uint8]string)
	for i := range inFmt {
		pool[inFmt[i]] = s[i]
	}
	out := make([]string, len(outFmt))
	for i, c := range outFmt {
		out[i] = pool[uint8(c)]
	}
	return out
}
