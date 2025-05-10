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
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/master"
	"go.uber.org/zap"
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
		debug, _ := cmd.PersistentFlags().GetBool("debug")
		log.SetLogger(cmd.PersistentFlags(), debug)

		// Create master
		configPath, _ := cmd.PersistentFlags().GetString("config")
		log.Logger().Info("load config", zap.String("config", configPath))
		conf, err := config.LoadConfig(configPath, false)
		if err != nil {
			log.Logger().Fatal("failed to load config", zap.Error(err))
		}
		cachePath, _ := cmd.PersistentFlags().GetString("cache-path")
		m := master.NewMaster(conf, cachePath)
		// Stop master
		done := make(chan struct{})
		go func() {
			sigint := make(chan os.Signal, 1)
			signal.Notify(sigint, os.Interrupt)
			<-sigint
			m.Shutdown()
			close(done)
		}()
		// Start master
		m.Serve()
		<-done
		log.Logger().Info("stop gorse master successfully")
	},
}

func init() {
	log.AddFlags(masterCommand.PersistentFlags())
	masterCommand.PersistentFlags().Bool("debug", false, "use debug log mode")
	masterCommand.PersistentFlags().Bool("managed", false, "enable managed mode")
	masterCommand.PersistentFlags().StringP("config", "c", "", "configuration file path")
	masterCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	masterCommand.PersistentFlags().String("cache-path", "master_cache.data", "path of cache file")
}

func main() {
	if err := masterCommand.Execute(); err != nil {
		log.Logger().Fatal("failed to execute", zap.Error(err))
	}
}
