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
	_ "net/http/pprof"

	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/common/util"
	"github.com/zhenghaoz/gorse/worker"
	"go.uber.org/zap"
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
		workingJobs, _ := cmd.PersistentFlags().GetInt("jobs")
		// setup logger
		debug, _ := cmd.PersistentFlags().GetBool("debug")
		log.SetLogger(cmd.PersistentFlags(), debug)
		// create worker
		cachePath, _ := cmd.PersistentFlags().GetString("cache-path")
		caFile, _ := cmd.PersistentFlags().GetString("ssl-ca")
		certFile, _ := cmd.PersistentFlags().GetString("ssl-cert")
		keyFile, _ := cmd.PersistentFlags().GetString("ssl-key")
		var tlsConfig *util.TLSConfig
		if caFile != "" && certFile != "" && keyFile != "" {
			tlsConfig = &util.TLSConfig{
				SSLCA:   caFile,
				SSLCert: certFile,
				SSLKey:  keyFile,
			}
		} else if caFile == "" && certFile == "" && keyFile == "" {
			tlsConfig = nil
		} else {
			log.Logger().Fatal("incomplete SSL configuration",
				zap.String("ssl_ca", caFile),
				zap.String("ssl_cert", certFile),
				zap.String("ssl_key", keyFile))
		}
		w := worker.NewWorker(masterHost, masterPort, httpHost, httpPort, workingJobs, cachePath, tlsConfig)
		w.Serve()
	},
}

func init() {
	log.AddFlags(workerCommand.PersistentFlags())
	workerCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	workerCommand.PersistentFlags().String("master-host", "127.0.0.1", "host of master node")
	workerCommand.PersistentFlags().Int("master-port", 8086, "port of master node")
	workerCommand.PersistentFlags().String("http-host", "127.0.0.1", "host for Prometheus metrics export")
	workerCommand.PersistentFlags().Int("http-port", 8089, "port for Prometheus metrics export")
	workerCommand.PersistentFlags().Bool("debug", false, "use debug log mode")
	workerCommand.PersistentFlags().Bool("managed", false, "enable managed mode")
	workerCommand.PersistentFlags().IntP("jobs", "j", 1, "number of working jobs.")
	workerCommand.PersistentFlags().String("cache-path", "worker_cache.data", "path of cache file")
	workerCommand.PersistentFlags().String("ssl-ca", "", "path of SSL CA")
	workerCommand.PersistentFlags().String("ssl-cert", "", "path to SSL certificate")
	workerCommand.PersistentFlags().String("ssl-key", "", "path to SSL key")
}

func main() {
	if err := workerCommand.Execute(); err != nil {
		log.Logger().Fatal("failed to execute", zap.Error(err))
	}
}
