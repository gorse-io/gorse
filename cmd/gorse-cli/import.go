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
	"github.com/araddon/dateparse"
	"github.com/cheggaaa/pb/v3"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage"
	"log"
	"os"
	"strings"
)

func importFeedback(csvFile string, sep string, header bool, fmt string, config *config.Config) {
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
	database, err := storage.Open(config.Database.Path)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	// Read lines
	scanner := bufio.NewScanner(file)
	bar := pb.StartNew(int(length))
	for scanner.Scan() {
		line := scanner.Text()
		splits := strings.Split(line, sep)
		splits = format(fmt, "ui", splits)
		if splits[0] == "" {
			log.Fatal("invalid user id")
		}
		if splits[1] == "" {
			log.Fatal("invalid item id")
		}
		feedback := storage.Feedback{UserId: splits[0], ItemId: splits[1]}
		err := database.InsertFeedback(feedback)
		if err != nil {
			log.Fatal(err)
		}
		bar.Add(len(line) + 1)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	bar.Finish()
}

func importItems(csvFile string, sep string, labelSep string, header bool, fmt string, config *config.Config) {
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
	database, err := storage.Open(config.Database.Path)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	// Read lines
	scanner := bufio.NewScanner(file)
	bar := pb.StartNew(int(length))
	for scanner.Scan() {
		line := scanner.Text()
		splits := strings.Split(line, sep)
		splits = format(fmt, "itl", splits)
		if splits[0] == "" {
			log.Fatal("invalid item id")
		}
		item := storage.Item{ItemId: splits[0]}
		if splits[1] != "" {
			item.Timestamp, err = dateparse.ParseAny(splits[1])
			if err != nil {
				log.Fatal(err)
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
