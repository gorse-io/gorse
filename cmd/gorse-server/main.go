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
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/storage"
	"log"
)

var serverCommand = &cobra.Command{
	Use:   "gorse-server",
	Short: "The server node of gorse recommender system",
	Run: func(cmd *cobra.Command, args []string) {
		// Load config
		configPath, _ := cmd.PersistentFlags().GetString("config")
		conf, meta, err := config.LoadConfig(configPath)
		if err != nil {
			log.Fatal(err)
		}
		if cmd.PersistentFlags().Changed("port") {
			conf.Server.Port, _ = cmd.PersistentFlags().GetInt("port")
		}
		if cmd.PersistentFlags().Changed("host") {
			conf.Server.Host, _ = cmd.PersistentFlags().GetString("host")
		}
		if cmd.PersistentFlags().Changed("database") {
			conf.Database.URL, _ = cmd.PersistentFlags().GetString("database")
		}
		// Start server
		s := server.Server{
			Config:   conf,
			MetaData: &meta,
		}
		if s.DB, err = storage.Open(conf.Database.URL); err != nil {
			log.Fatal(err)
		}
		s.Serve()
	},
}

func init() {
	serverCommand.PersistentFlags().StringP("config", "c", "/etc/gorse/config.toml", "Configuration file path.")
	serverCommand.PersistentFlags().Int("port", 8081, "Server port")
	serverCommand.PersistentFlags().String("host", "127.0.0.1", "Server host")
	serverCommand.PersistentFlags().String("database", "", "Database address")
}

func main() {
	if err := serverCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}
