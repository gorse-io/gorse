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
	"github.com/zhenghaoz/gorse/base"
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
		debugMode, _ := cmd.PersistentFlags().GetBool("debug")
		if debugMode {
			base.SetDevelopmentLogger()
		}
		// Start master
		configPath, _ := cmd.PersistentFlags().GetString("config")
		base.Logger().Info("load config", zap.String("config", configPath))
		conf, _, err := config.LoadConfig(configPath)
		if err != nil {
			base.Logger().Fatal("failed to load config", zap.Error(err))
		}
		l := master.NewMaster(conf)
		l.Serve()
	},
}

func init() {
	masterCommand.PersistentFlags().Bool("debug", false, "use debug log mode")
	masterCommand.PersistentFlags().StringP("config", "c", "/etc/gorse.toml", "configuration file path")
	masterCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
}

func main() {
	if err := masterCommand.Execute(); err != nil {
		base.Logger().Fatal("failed to execute", zap.Error(err))
	}
}
