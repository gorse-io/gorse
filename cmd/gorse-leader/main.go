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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/leader"
	"github.com/zhenghaoz/gorse/storage"
)

var leaderCommand = &cobra.Command{
	Use:   "gorse-leader",
	Short: "The leader node of gorse recommender system",
	Run: func(cmd *cobra.Command, args []string) {
		// Load config
		configPath, _ := cmd.PersistentFlags().GetString("config")
		log.Infof("Leader: load config from %v", configPath)
		conf, meta, err := config.LoadConfig(configPath)
		if err != nil {
			log.Fatal(err)
		}
		if cmd.PersistentFlags().Changed("port") {
			conf.Leader.Port, _ = cmd.PersistentFlags().GetInt("port")
		}
		if cmd.PersistentFlags().Changed("host") {
			conf.Leader.Host, _ = cmd.PersistentFlags().GetString("host")
		}
		if cmd.PersistentFlags().Changed("database") {
			conf.Database.Path, _ = cmd.PersistentFlags().GetString("database")
		}
		// Start server
		db, err := storage.Open(conf.Database.Path)
		if err != nil {
			log.Fatal(err)
		}
		l := leader.NewLeader(db, conf, meta)
		l.Serve()
	},
}

func init() {
	leaderCommand.PersistentFlags().StringP("config", "c", "/etc/leader.toml", "Configuration file path.")
	leaderCommand.PersistentFlags().Int("port", 8081, "Server port")
	leaderCommand.PersistentFlags().String("host", "127.0.0.1", "Server host")
	leaderCommand.PersistentFlags().String("database", "", "Database address")
}

func main() {
	if err := leaderCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}
