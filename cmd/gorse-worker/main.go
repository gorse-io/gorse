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
	"github.com/zhenghaoz/gorse/storage"
	"github.com/zhenghaoz/gorse/worker"
)

var workerCommand = &cobra.Command{
	Use:   "gorse-worker",
	Short: "The worker node of gorse recommender system",
	Run: func(cmd *cobra.Command, args []string) {
		// Load config
		configPath, _ := cmd.PersistentFlags().GetString("config")
		log.Infof("Leader: load config from %v", configPath)
		conf, _, err := config.LoadConfig(configPath)
		if err != nil {
			log.Fatal(err)
		}
		if cmd.PersistentFlags().Changed("leader-addr") {
			conf.Worker.LeaderAddr, _ = cmd.PersistentFlags().GetString("leader-addr")
		}
		if cmd.PersistentFlags().Changed("rpc-port") {
			conf.Worker.RPCPort, _ = cmd.PersistentFlags().GetInt("rpc-port")
		}
		if cmd.PersistentFlags().Changed("gossip-port") {
			conf.Worker.GossipPort, _ = cmd.PersistentFlags().GetInt("gossip-port")
		}
		if cmd.PersistentFlags().Changed("host") {
			conf.Worker.Host, _ = cmd.PersistentFlags().GetString("host")
		}
		if cmd.PersistentFlags().Changed("database") {
			conf.Database.Path, _ = cmd.PersistentFlags().GetString("database")
		}
		// Start server
		db, err := storage.Open(conf.Database.Path)
		if err != nil {
			log.Fatal(err)
		}
		// create worker
		w := worker.NewWorker(db, conf)
		w.Serve()
	},
}

func init() {
	workerCommand.PersistentFlags().StringP("config", "c", "/etc/leader.toml", "Configuration file path.")
	workerCommand.PersistentFlags().String("leader-addr", "", "Leader address")
	workerCommand.PersistentFlags().Int("gossip-port", 8081, "gossip port")
	workerCommand.PersistentFlags().Int("rpc-port", 8081, "RPC port")
	workerCommand.PersistentFlags().String("host", "127.0.0.1", "Server host")
	workerCommand.PersistentFlags().String("database", "", "Database address")
}

func main() {
	if err := workerCommand.Execute(); err != nil {
		log.Error(err)
	}
}
