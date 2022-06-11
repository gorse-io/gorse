// Copyright 2022 gorse Project Authors
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
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/master"
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/worker"
	"go.uber.org/zap"
)

var oneCommand = &cobra.Command{
	Use:   "gorse-in-one",
	Short: "The all in one distribution of gorse recommender system.",
	Run: func(cmd *cobra.Command, args []string) {
		// Show version
		if showVersion, _ := cmd.PersistentFlags().GetBool("version"); showVersion {
			fmt.Println(version.BuildInfo())
			return
		}

		// setup logger
		var outputPaths []string
		if cmd.PersistentFlags().Changed("log-path") {
			outputPath, _ := cmd.PersistentFlags().GetString("log-path")
			outputPaths = append(outputPaths, outputPath)
		}
		debugMode, _ := cmd.PersistentFlags().GetBool("debug")
		if debugMode {
			log.SetDevelopmentLogger(outputPaths...)
		} else {
			log.SetProductionLogger(outputPaths...)
		}

		// load config
		configPath, _ := cmd.PersistentFlags().GetString("config")
		log.Logger().Info("load config", zap.String("config", configPath))
		conf, err := config.LoadConfig(configPath, true)
		if err != nil {
			log.Logger().Fatal("failed to load config", zap.Error(err))
		}

		// create master
		masterCachePath, _ := cmd.PersistentFlags().GetString("master-cache-path")
		l := master.NewMaster(conf, masterCachePath)
		// Start worker
		go func() {
			workerCachePath, _ := cmd.PersistentFlags().GetString("worker-cache-path")
			workerJobs, _ := cmd.PersistentFlags().GetInt("worker-jobs")
			w := worker.NewWorker(conf.Master.Host, conf.Master.Port, conf.Master.Host,
				0, workerJobs, workerCachePath)
			w.SetOneMode(l.Settings)
			w.Serve()
		}()
		// Start server
		go func() {
			serverCachePath, _ := cmd.PersistentFlags().GetString("server-cache-path")
			serverPort, _ := cmd.PersistentFlags().GetInt("server-port")
			s := server.NewServer(conf.Master.Host, conf.Master.Port, conf.Master.Host,
				serverPort, serverCachePath)
			s.SetOneMode(l.Settings)
			s.Serve()
		}()
		// Start master
		l.Serve()
	},
}

func init() {
	oneCommand.PersistentFlags().Bool("debug", false, "use debug log mode")
	oneCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	oneCommand.PersistentFlags().String("log-path", "", "path of log file")
	// master node commands
	oneCommand.PersistentFlags().StringP("config", "c", "", "configuration file path")
	oneCommand.PersistentFlags().String("master-cache-path", "master_cache.data", "path of cache file for the master node")
	// worker node commands
	oneCommand.PersistentFlags().Int("worker-jobs", 1, "number of working jobs for the worker node")
	oneCommand.PersistentFlags().String("worker-cache-path", "worker_cache.data", "path of cache file for the worker node")
	// server node commands
	oneCommand.PersistentFlags().Int("server-port", 8087, "port of RESTful APIs and Prometheus metrics for the server node")
	oneCommand.PersistentFlags().String("server-cache-path", "server_cache.data", "path of cache file for the server node")
}

func main() {
	if err := oneCommand.Execute(); err != nil {
		log.Logger().Fatal("failed to execute", zap.Error(err))
	}
}
