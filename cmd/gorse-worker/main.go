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
	"github.com/zhenghaoz/gorse/worker"
	"go.uber.org/zap"
	_ "net/http/pprof"
)

var workerCommand = &cobra.Command{
	Use:   "gorse-worker",
	Short: "The worker node of gorse recommender system.",
	Run: func(cmd *cobra.Command, args []string) {
		// show version
		showVersion, _ := cmd.PersistentFlags().GetBool("version")
		if showVersion {
			fmt.Println(version.BuildInfo())
			return
		}
		masterHost, _ := cmd.PersistentFlags().GetString("master-host")
		masterPort, _ := cmd.PersistentFlags().GetInt("master-port")
		httpHost, _ := cmd.PersistentFlags().GetString("http-host")
		httpPort, _ := cmd.PersistentFlags().GetInt("http-port")
		debugMode, _ := cmd.PersistentFlags().GetBool("debug")
		workingJobs, _ := cmd.PersistentFlags().GetInt("jobs")
		// setup logger
		if debugMode {
			base.SetDevelopmentLogger()
		}
		// create worker
		w := worker.NewWorker(masterHost, masterPort, httpHost, httpPort, workingJobs)
		w.Serve()
	},
}

func init() {
	workerCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	workerCommand.PersistentFlags().String("master-host", "127.0.0.1", "host of master node")
	workerCommand.PersistentFlags().Int("master-port", 8086, "port of master node")
	workerCommand.PersistentFlags().String("http-host", "127.0.0.1", "host of status report")
	workerCommand.PersistentFlags().Int("http-port", 8089, "port of status report")
	workerCommand.PersistentFlags().Bool("debug", false, "use debug log mode")
	workerCommand.PersistentFlags().IntP("jobs", "j", 1, "number of working jobs.")
}

func main() {
	if err := workerCommand.Execute(); err != nil {
		base.Logger().Fatal("failed to execute", zap.Error(err))
	}
}
