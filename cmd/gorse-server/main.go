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

	"github.com/gorse-io/gorse/base/log"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var serverCommand = &cobra.Command{
	Use:   "gorse-server",
	Short: "The server node of gorse recommender system.",
	Run: func(cmd *cobra.Command, args []string) {
		// show version
		showVersion, _ := cmd.PersistentFlags().GetBool("version")
		if showVersion {
			fmt.Println(version.BuildInfo())
			return
		}

		// setup logger
		debug, _ := cmd.PersistentFlags().GetBool("debug")
		log.SetLogger(cmd.PersistentFlags(), debug)

		// create server
		masterPort, _ := cmd.PersistentFlags().GetInt("master-port")
		masterHost, _ := cmd.PersistentFlags().GetString("master-host")
		httpPort, _ := cmd.PersistentFlags().GetInt("http-port")
		httpHost, _ := cmd.PersistentFlags().GetString("http-host")
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
		s := server.NewServer(masterHost, masterPort, httpHost, httpPort, cachePath, tlsConfig)

		// stop server
		done := make(chan struct{})
		go func() {
			sigint := make(chan os.Signal, 1)
			signal.Notify(sigint, os.Interrupt)
			<-sigint
			s.Shutdown()
			close(done)
		}()

		// start server
		s.Serve()
		<-done
		log.Logger().Info("stop gorse server successfully")
	},
}

func init() {
	log.AddFlags(serverCommand.PersistentFlags())
	serverCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	serverCommand.PersistentFlags().Int("master-port", 8086, "port of master node")
	serverCommand.PersistentFlags().String("master-host", "127.0.0.1", "host of master node")
	serverCommand.PersistentFlags().Int("http-port", 8087, "host for RESTful APIs and Prometheus metrics export")
	serverCommand.PersistentFlags().String("http-host", "127.0.0.1", "port for RESTful APIs and Prometheus metrics export")
	serverCommand.PersistentFlags().Bool("debug", false, "use debug log mode")
	serverCommand.PersistentFlags().String("cache-path", "server_cache.data", "path of cache file")
	serverCommand.PersistentFlags().String("ssl-ca", "", "path of SSL CA")
	serverCommand.PersistentFlags().String("ssl-cert", "", "path of SSL certificate")
	serverCommand.PersistentFlags().String("ssl-key", "", "path of SSL key")
}

func main() {
	if err := serverCommand.Execute(); err != nil {
		log.Logger().Fatal("failed to execute", zap.Error(err))
	}
}
