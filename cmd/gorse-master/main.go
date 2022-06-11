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
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/master"
	"go.uber.org/zap"
	_ "net/http/pprof"
)

var masterCommand = &cobra.Command{
	Use:   "gorse-master",
	Short: "The master node of gorse recommender system.",
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
		// Start master
		configPath, _ := cmd.PersistentFlags().GetString("config")
		log.Logger().Info("load config", zap.String("config", configPath))
		conf, err := config.LoadConfig(configPath, false)
		if err != nil {
			log.Logger().Fatal("failed to load config", zap.Error(err))
		}
		cachePath, _ := cmd.PersistentFlags().GetString("cache-path")
		l := master.NewMaster(conf, cachePath)
		l.Serve()
	},
}

func init() {
	masterCommand.PersistentFlags().Bool("debug", false, "use debug log mode")
	masterCommand.PersistentFlags().StringP("config", "c", "", "configuration file path")
	masterCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	masterCommand.PersistentFlags().String("log-path", "", "path of log file")
	masterCommand.PersistentFlags().String("cache-path", "master_cache.data", "path of cache file")
}

func main() {
	if err := masterCommand.Execute(); err != nil {
		log.Logger().Fatal("failed to execute", zap.Error(err))
	}
}
