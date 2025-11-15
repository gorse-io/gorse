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
	"compress/gzip"
	"fmt"
	"os"
	"os/signal"

	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const playgroundDataFile = "https://cdn.gorse.io/example/github.sql.gz"

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
		debug, _ := cmd.PersistentFlags().GetBool("debug")
		log.SetLogger(cmd.PersistentFlags(), debug)

		// load config
		var conf *config.Config
		var err error
		playgroundMode, _ := cmd.PersistentFlags().GetBool("playground")
		if playgroundMode {
			// load config for playground
			conf = config.GetDefaultConfig()
			conf.Database.DataStore = "sqlite://data.db"
			conf.Database.CacheStore = "sqlite://cache.db"
			conf.Recommend.DataSource.PositiveFeedbackTypes = []expression.FeedbackTypeExpression{
				expression.MustParseFeedbackTypeExpression("star"),
				expression.MustParseFeedbackTypeExpression("like"),
				expression.MustParseFeedbackTypeExpression("read>=3"),
			}
			conf.Recommend.DataSource.ReadFeedbackTypes = []expression.FeedbackTypeExpression{
				expression.MustParseFeedbackTypeExpression("read"),
			}
			if err := conf.Validate(); err != nil {
				log.Logger().Fatal("invalid config", zap.Error(err))
			}

			fmt.Printf("Welcome to Gorse %s Playground\n", version.Version)

			fmt.Println()
			fmt.Printf("    Dashboard:     http://127.0.0.1:%d/overview\n", conf.Master.HttpPort)
			fmt.Printf("    RESTful APIs:  http://127.0.0.1:%d/apidocs\n", conf.Master.HttpPort)
			fmt.Printf("    Documentation: https://gorse.io/docs\n")
			fmt.Println()
		} else {
			configPath, _ := cmd.PersistentFlags().GetString("config")
			log.Logger().Info("load config", zap.String("config", configPath))
			conf, err = config.LoadConfig(configPath)
			if err != nil {
				log.Logger().Fatal("failed to load config", zap.Error(err))
			}
		}

		// create master
		cachePath, _ := cmd.PersistentFlags().GetString("cache-path")
		m := master.NewMaster(conf, cachePath, true)

		if playgroundMode {
			// connect data database
			m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.DataTablePrefix,
				storage.WithIsolationLevel(m.Config.Database.MySQL.IsolationLevel))
			if err != nil {
				log.Logger().Fatal("failed to connect data database", zap.Error(err),
					zap.String("database", log.RedactDBURL(m.Config.Database.DataStore)))
			}
			if err = m.DataClient.Init(); err != nil {
				log.Logger().Fatal("failed to init database", zap.Error(err))
			}

			// import playground data
			fileInfo, err := os.Stat("C:\\Users\\zhang\\Downloads\\github.bin.gz")
			if err != nil {
				log.Logger().Fatal("failed to stat dump file", zap.Error(err))
			}
			bar := progressbar.DefaultBytes(
				fileInfo.Size(),
				"Importing playground data",
			)
			f, err := os.Open("C:\\Users\\zhang\\Downloads\\github.bin.gz")
			if err != nil {
				log.Logger().Fatal("failed to open playground data", zap.Error(err))
			}
			p := progressbar.NewReader(f, bar)
			d, err := gzip.NewReader(&p)
			if err != nil {
				log.Logger().Fatal("failed to read playground data", zap.Error(err))
			}
			_, err = m.Restore(d)
			if err != nil {
				log.Logger().Fatal("failed to import playground data", zap.Error(err))
			}
			fmt.Println("setting up playground data...")
			os.Exit(0)
		}

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
		log.Logger().Info("stop gorse-in-one successfully")
	},
}

func init() {
	log.AddFlags(oneCommand.PersistentFlags())
	oneCommand.PersistentFlags().Bool("debug", false, "use debug log mode")
	oneCommand.PersistentFlags().Bool("managed", false, "enable managed mode")
	oneCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	oneCommand.PersistentFlags().Bool("playground", false, "playground mode (setup a recommender system for GitHub repositories)")
	oneCommand.PersistentFlags().StringP("config", "c", "", "configuration file path")
	oneCommand.PersistentFlags().String("cache-path", "one_cache.data", "path of cache file")
	oneCommand.PersistentFlags().Int("recommend-jobs", 1, "number of working jobs for recommendation tasks")
}

func main() {
	if err := oneCommand.Execute(); err != nil {
		log.Logger().Fatal("failed to execute", zap.Error(err))
	}
}
