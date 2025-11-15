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
	"bufio"
	"compress/gzip"
	"database/sql"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/juju/errors"
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

			if err = initializeDatabase("data.db"); err != nil {
				log.Logger().Fatal("failed to initialize database", zap.Error(err))
			}

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

func initializeDatabase(path string) error {
	// skip initialization if file exists
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	// init database
	databaseClient, err := data.Open(storage.SQLitePrefix+path, "")
	if err != nil {
		return errors.Trace(err)
	}
	if err = databaseClient.Init(); err != nil {
		return errors.Trace(err)
	}
	if err = databaseClient.Close(); err != nil {
		return errors.Trace(err)
	}

	// open database
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return errors.Trace(err)
	}
	defer db.Close()

	// download mysqldump file
	resp, err := http.Get(playgroundDataFile)
	if err != nil {
		return errors.Trace(err)
	}

	// load mysqldump file
	pbReader := progressbar.NewReader(resp.Body, progressbar.DefaultBytes(
		resp.ContentLength,
		"Downloading playground dataset",
	))
	gzipReader, err := gzip.NewReader(&pbReader)
	if err != nil {
		return errors.Trace(err)
	}
	bufio.NewReader(gzipReader)

	return nil
}
