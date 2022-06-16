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
	"bytes"
	"database/sql"
	_ "embed"
	"fmt"
	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
	"github.com/juju/errors"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/master"
	"github.com/zhenghaoz/gorse/storage"
	"github.com/zhenghaoz/gorse/storage/data"
	"github.com/zhenghaoz/gorse/worker"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"
	"strings"
)

//go:embed mysql2sqlite
var mysql2SQLite string

const playgroundDataFile = "https://cdn.gorse.io/example/github.sql"

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
		var conf *config.Config
		var err error
		playgroundMode, _ := cmd.PersistentFlags().GetBool("playground")
		if playgroundMode {
			// load config for playground
			conf = config.GetDefaultConfig()
			conf.Database.DataStore = "sqlite://data.db"
			conf.Database.CacheStore = "sqlite://cache.db"
			conf.Recommend.DataSource.PositiveFeedbackTypes = []string{"star", "like"}
			conf.Recommend.DataSource.ReadFeedbackTypes = []string{"read"}
			if err := conf.Validate(true); err != nil {
				log.Logger().Fatal("invalid config", zap.Error(err))
			}

			// print prologue
			fmt.Printf("Welcome to Gorse %s Playground\n", version.Version)
			fmt.Println()
			fmt.Printf("    Dashboard:     http://%s:%d/overview\n", conf.Master.HttpHost, conf.Master.HttpPort)
			fmt.Printf("    RESTful APIs:  http://%s:%d/apidocs\n", conf.Master.HttpHost, conf.Master.HttpPort)
			fmt.Printf("    Documentation: https://docs.gorse.io/\n")
			fmt.Println()

			if err = initializeDatabase("data.db"); err != nil {
				log.Logger().Fatal("failed to initialize database", zap.Error(err))
			}
		} else {
			configPath, _ := cmd.PersistentFlags().GetString("config")
			log.Logger().Info("load config", zap.String("config", configPath))
			conf, err = config.LoadConfig(configPath, true)
			if err != nil {
				log.Logger().Fatal("failed to load config", zap.Error(err))
			}
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
		// Start master
		l.Serve()
	},
}

func init() {
	oneCommand.PersistentFlags().Bool("debug", false, "use debug log mode")
	oneCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	oneCommand.PersistentFlags().String("log-path", "", "path of log file")
	oneCommand.PersistentFlags().Bool("playground", false, "playground mode (setup a recommender system for GitHub repositories)")
	// master node commands
	oneCommand.PersistentFlags().StringP("config", "c", "", "configuration file path")
	oneCommand.PersistentFlags().String("master-cache-path", "master_cache.data", "path of cache file for the master node")
	// worker node commands
	oneCommand.PersistentFlags().Int("worker-jobs", 1, "number of working jobs for the worker node")
	oneCommand.PersistentFlags().String("worker-cache-path", "worker_cache.data", "path of cache file for the worker node")
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
	databaseClient, err := data.Open(storage.SQLitePrefix + path)
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

	// create converter
	converter, err := NewMySQLToSQLiteConverter(mysql2SQLite)
	if err != nil {
		return errors.Trace(err)
	}

	// load mysqldump file
	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"Downloading playground dataset",
	)
	reader := bufio.NewReader(resp.Body)
	var builder strings.Builder
	for {
		line, isPrefix, err := reader.ReadLine()
		_ = bar.Add(len(line))
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Trace(err)
		}
		builder.Write(line)
		if !isPrefix {
			_ = bar.Add(1)
			text := builder.String()
			if strings.HasPrefix(text, "INSERT INTO ") {
				// convert to SQLite sql
				sqliteSQL, err := converter.Convert(text)
				if err != nil {
					return errors.Trace(err)
				}
				if _, err = db.Exec(sqliteSQL); err != nil {
					return errors.Trace(err)
				}
			}
			builder.Reset()
		}
	}
	return nil
}

type MySQLToSQLiteConverter struct {
	interpreter *interp.Interpreter
}

func NewMySQLToSQLiteConverter(source string) (*MySQLToSQLiteConverter, error) {
	converter := &MySQLToSQLiteConverter{}
	program, err := parser.ParseProgram([]byte(source), nil)
	if err != nil {
		return nil, errors.Trace(err)
	}
	converter.interpreter, err = interp.New(program)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return converter, nil
}

func (converter *MySQLToSQLiteConverter) Convert(sql string) (string, error) {
	input := strings.NewReader(sql)
	output := bytes.NewBuffer(nil)
	if _, err := converter.interpreter.Execute(&interp.Config{
		Stdin: input, Output: output, Args: []string{"-"},
	}); err != nil {
		return "", errors.Trace(err)
	}
	return output.String(), nil
}
