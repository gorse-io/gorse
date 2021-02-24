// Copyright 2021 gorse Project Authors
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
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/storage/cache"
	"os"
)

func init() {
	cliCommand.AddCommand(statusCommand)
}

var statusCommand = &cobra.Command{
	Use:   "status",
	Short: "Check the status of recommender system",
	Run: func(cmd *cobra.Command, args []string) {
		// connect to cache store
		cacheStore, err := cache.Open(globalConfig.Database.CacheStore)
		if err != nil {
			log.Fatal("cli:", err)
		}
		// show status
		status := []string{
			cache.LastUpdatePopularTime,
			cache.LastUpdateLatestTime,
			cache.LastUpdateSimilarTime,
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"status", "value"})
		for _, stat := range status {
			val, err := cacheStore.GetString(cache.GlobalMeta, stat)
			if err != nil && err.Error() != "redis: nil" {
				log.Fatal("cli:", err)
			}
			table.Append([]string{stat, val})
		}
		table.Render()
	},
}
