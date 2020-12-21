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
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage"
	"log"
	"os"
	"strings"
)

const batchSize = 1000

func exportFeedback(csvFile string, sep string, header bool, config *config.Config) {
	// Open database
	database, err := storage.Open(config.Database.Path)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	// Open file
	file, err := os.Create(csvFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// Export feedbacks
	if header {
		if _, err := file.WriteString(fmt.Sprintf("user_id%vitem_id\n", sep)); err != nil {
			log.Fatal(err)
		}
	}
	cursor := ""
	count := 0
	for {
		var feedback []storage.Feedback
		var err error
		cursor, feedback, err = database.GetFeedback(cursor, batchSize)
		if err != nil {
			log.Fatal(err)
		}
		for _, v := range feedback {
			if _, err = file.WriteString(fmt.Sprintf("%v%v%v\n", v.UserId, sep, v.ItemId)); err != nil {
				log.Fatal(err)
			}
		}
		count += len(feedback)
		fmt.Printf("\rexport feedback %v", count)
		if cursor == "" {
			fmt.Println()
			break
		}
	}
}

func exportItems(csvFile string, sep string, labelSep string, header bool, config *config.Config) {
	// Open database
	database, err := storage.Open(config.Database.Path)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	// Open file
	file, err := os.Create(csvFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// Export items
	cursor := ""
	for {
		cursor, items, err := database.GetItems("", cursor, batchSize)
		if err != nil {
			log.Fatal(err)
		}
		for _, item := range items {
			if _, err = file.WriteString(fmt.Sprintf("%v%v%v%v%v\n",
				item.ItemId, sep, item.Timestamp, sep, strings.Join(item.Labels, labelSep))); err != nil {
				log.Fatal(err)
			}
		}
		if cursor == "" {
			break
		}
	}
}
