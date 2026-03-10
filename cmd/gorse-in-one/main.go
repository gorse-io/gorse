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

	"github.com/gorse-io/gorse/client"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/klauspost/cpuid/v2"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

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
		var configPath string
		cachePath, _ := cmd.PersistentFlags().GetString("cache-path")
		playground, _ := cmd.PersistentFlags().GetString("playground")
		if playground != "" {
			userHomeDir, err := os.UserHomeDir()
			if err != nil {
				log.Logger().Fatal("failed to get user home directory", zap.Error(err))
			}
			etcDir := filepath.Join(userHomeDir, ".gorse", "etc")
			if err = os.MkdirAll(etcDir, os.ModePerm); err != nil {
				log.Logger().Fatal("failed to create config directory", zap.Error(err))
			}
			configPath = filepath.Join(etcDir, "config.toml")
			if playground == "ml-100k" {
				err = os.WriteFile(configPath, []byte(client.ConfigTOML), 0644)
			} else {
				err = os.WriteFile(configPath, []byte(config.ConfigTOML), 0644)
			}
			if err != nil {
				log.Logger().Fatal("failed to write playground config", zap.Error(err))
			}
			fmt.Println("Generated config file:", configPath)
			fmt.Println("Using cache directory:", cachePath)
		} else {
			configPath, _ = cmd.PersistentFlags().GetString("config")
			log.Logger().Info("load config", zap.String("config", configPath))
		}

		// load config
		conf, err := config.LoadConfig(configPath)
		if err != nil {
			log.Logger().Fatal("failed to load config", zap.Error(err))
		}

		// create master
		m := master.NewMaster(conf, cachePath, true, configPath)

		if playground != "" {
			setup(m, playground)
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
	oneCommand.PersistentFlags().BoolP("version", "v", false, "gorse version")
	oneCommand.PersistentFlags().String("playground", "", "playground mode (setup a recommender system for GitHub repositories)")
	oneCommand.PersistentFlags().Lookup("playground").NoOptDefVal = "default"
	oneCommand.PersistentFlags().StringP("config", "c", "", "configuration file path")
	oneCommand.PersistentFlags().String("cache-path", config.MkDir("master"), "path of cache folder")
}

func main() {
	if err := oneCommand.Execute(); err != nil {
		log.Logger().Fatal("failed to execute", zap.Error(err))
	}
}

func setup(m *master.Master, playground string) {
	// set database to user home directory
	m.Config.Database.DataStore = config.GetDefaultConfig().Database.DataStore
	fmt.Println("Using database:", m.Config.Database.DataStore)
	m.Config.Database.CacheStore = config.GetDefaultConfig().Database.CacheStore
	fmt.Println("Using cache:", m.Config.Database.CacheStore)
	m.Config.Master.NumJobs = runtime.NumCPU()
	fmt.Printf("Using %d CPU cores: %s\n", m.Config.Master.NumJobs, cpuid.CPU.BrandName)

	// connect database
	var err error
	dataOpts := m.Config.Database.StorageOptions(m.Config.Database.DataStore)
	m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.DataTablePrefix, dataOpts...)
	if err != nil {
		log.Logger().Fatal("failed to connect data database", zap.Error(err),
			zap.String("database", log.RedactDBURL(m.Config.Database.DataStore)))
	}
	defer m.DataClient.Close()
	if err = m.DataClient.Init(); err != nil {
		log.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// import playground data
	var resp *http.Response
	if playground == "ml-100k" {
		resp, err = http.Get("https://cdn.gorse.io/example/ml-100k.bin.gz")
	} else {
		resp, err = http.Get("https://cdn.gorse.io/example/github.bin.gz")
	}
	if err != nil {
		log.Logger().Fatal("failed to download playground data", zap.Error(err))
	}
	defer resp.Body.Close()
	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"Importing data",
	)
	p := progressbar.NewReader(resp.Body, bar)
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
