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
	"github.com/zhenghaoz/gorse/worker"
	"runtime"
)

var workerCommand = &cobra.Command{
	Use:   "gorse-worker",
	Short: "The worker node of gorse recommender system",
	Run: func(cmd *cobra.Command, args []string) {
		masterHost, _ := cmd.PersistentFlags().GetString("master-host")
		masterPort, _ := cmd.PersistentFlags().GetInt("master-port")
		workingJobs, _ := cmd.PersistentFlags().GetInt("jobs")
		// create worker
		w := worker.NewWorker(masterHost, masterPort, workingJobs)
		w.Serve()
	},
}

func init() {
	workerCommand.PersistentFlags().String("master-host", "127.0.0.1", "Master host.")
	workerCommand.PersistentFlags().Int("master-port", 6384, "Master port.")
	workerCommand.PersistentFlags().IntP("jobs", "j", runtime.NumCPU(), "Number of working jobs.")
}

func main() {
	if err := workerCommand.Execute(); err != nil {
		log.Error(err)
	}
}
