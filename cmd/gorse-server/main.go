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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/server"
)

var serverCommand = &cobra.Command{
	Use:   "gorse-server",
	Short: "The server node of gorse recommender system.",
	Run: func(cmd *cobra.Command, args []string) {
		// show version
		showVersion, _ := cmd.PersistentFlags().GetBool("version")
		if showVersion {
			fmt.Println(version.VersionName)
			return
		}
		// start server
		masterPort, _ := cmd.PersistentFlags().GetInt("master-port")
		masterHost, _ := cmd.PersistentFlags().GetString("master-host")
		port, _ := cmd.PersistentFlags().GetInt("port")
		host, _ := cmd.PersistentFlags().GetString("host")
		s := server.NewServer(masterHost, masterPort, host, port)
		s.Serve()
	},
}

func init() {
	serverCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	serverCommand.PersistentFlags().Int("master-port", 8086, "port of master node")
	serverCommand.PersistentFlags().String("master--host", "127.0.0.1", "host of master node")
	serverCommand.PersistentFlags().Int("port", 8087, "port of server node")
	serverCommand.PersistentFlags().String("host", "127.0.0.1", "host of server node")
}

func main() {
	if err := serverCommand.Execute(); err != nil {
		log.Fatal("server: ", err)
	}
}
