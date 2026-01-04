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
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"

	"github.com/fsnotify/fsnotify"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/klauspost/cpuid/v2"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const dumpFile = "https://cdn.gorse.io/example/github.bin.gz"

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

		// locate config file
		var (
			configPath string
			cachePath  string
		)
		playground, _ := cmd.PersistentFlags().GetBool("playground")
		if playground {
			userHomeDir, err := os.UserHomeDir()
			if err != nil {
				log.Logger().Fatal("failed to get user home directory", zap.Error(err))
			}
			etcDir := filepath.Join(userHomeDir, ".gorse", "etc")
			if err = os.MkdirAll(etcDir, os.ModePerm); err != nil {
				log.Logger().Fatal("failed to create config directory", zap.Error(err))
			}
			configPath = filepath.Join(etcDir, "config.toml")
			if err = os.WriteFile(configPath, []byte(config.ConfigTOML), 0644); err != nil {
				log.Logger().Fatal("failed to write playground config", zap.Error(err))
			}
			fmt.Println("Generated config file:", configPath)
			cachePath = filepath.Join(userHomeDir, ".gorse", "tmp")
			if err = os.MkdirAll(cachePath, os.ModePerm); err != nil {
				log.Logger().Fatal("failed to create tmp directory", zap.Error(err))
			}
			fmt.Println("Using cache directory:", cachePath)
		} else {
			configPath, _ = cmd.PersistentFlags().GetString("config")
			cachePath, _ = cmd.PersistentFlags().GetString("cache-path")
			log.Logger().Info("load config", zap.String("config", configPath))
		}

		// load config
		conf, err := config.LoadConfig(configPath)
		if err != nil {
			log.Logger().Fatal("failed to load config", zap.Error(err))
		}

		// create master
		m := master.NewMaster(conf, cachePath, true)

		if playground {
			setup(m)
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

		// Watch config file change
		viper.WatchConfig()
		viper.OnConfigChange(func(e fsnotify.Event) {
			log.Logger().Info("reload config", zap.String("config", e.Name))
			cfg, err := config.LoadConfig(e.Name)
			if err != nil {
				log.Logger().Error("failed to reload config", zap.Error(err))
				return
			}
			m.Reload(cfg)
		})

		// Start master
		m.Serve()
		<-done
		log.Logger().Info("stop gorse-in-one successfully")
	},
}

func init() {
	log.AddFlags(oneCommand.PersistentFlags())
	oneCommand.PersistentFlags().Bool("debug", false, "use debug log mode")
	oneCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	oneCommand.PersistentFlags().Bool("playground", false, "playground mode (setup a recommender system for GitHub repositories)")
	oneCommand.PersistentFlags().StringP("config", "c", "", "configuration file path")
	oneCommand.PersistentFlags().String("cache-path", "one_cache.data", "path of cache file")
}

func main() {
	if err := oneCommand.Execute(); err != nil {
		log.Logger().Fatal("failed to execute", zap.Error(err))
	}
}

func setup(m *master.Master) {
	// set database to user home directory
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		log.Logger().Fatal("failed to get user home directory", zap.Error(err))
	}
	libDir := filepath.Join(userHomeDir, ".gorse", "lib")
	if err = os.MkdirAll(libDir, os.ModePerm); err != nil {
		log.Logger().Fatal("failed to create lib directory", zap.Error(err))
	}
	m.Config.Database.DataStore = "sqlite://" + filepath.Join(libDir, "data.db")
	fmt.Println("Using database:", m.Config.Database.DataStore)
	m.Config.Database.CacheStore = "sqlite://" + filepath.Join(libDir, "cache.db")
	fmt.Println("Using cache:", m.Config.Database.CacheStore)
	m.Config.Master.NumJobs = runtime.NumCPU()
	fmt.Printf("Using %d CPU cores: %s\n", m.Config.Master.NumJobs, cpuid.CPU.BrandName)

	// connect database
	m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.DataTablePrefix,
		storage.WithIsolationLevel(m.Config.Database.MySQL.IsolationLevel))
	if err != nil {
		log.Logger().Fatal("failed to connect data database", zap.Error(err),
			zap.String("database", log.RedactDBURL(m.Config.Database.DataStore)))
	}
	defer m.DataClient.Close()
	if err = m.DataClient.Init(); err != nil {
		log.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// import playground data
	req, err := http.Get(dumpFile)
	if err != nil {
		log.Logger().Fatal("failed to download playground data", zap.Error(err))
	}
	defer req.Body.Close()
	bar := progressbar.DefaultBytes(
		req.ContentLength,
		"Importing data",
	)
	p := progressbar.NewReader(req.Body, bar)
	d, err := gzip.NewReader(&p)
	if err != nil {
		log.Logger().Fatal("failed to decompress playground data", zap.Error(err))
	}
	_, err = m.Restore(d)
	if err != nil {
		log.Logger().Fatal("failed to import playground data", zap.Error(err))
	}

	// show info
	fmt.Printf("Welcome to Gorse Playground\n")
	fmt.Println()
	fmt.Printf("    Dashboard:     http://127.0.0.1:%d/overview\n", m.Config.Master.HttpPort)
	fmt.Printf("    RESTful APIs:  http://127.0.0.1:%d/apidocs\n", m.Config.Master.HttpPort)
	fmt.Printf("    Documentation: https://gorse.io\n")
	fmt.Println()
}
