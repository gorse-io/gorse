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
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/protocol"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"log"
)

var configPath = model.GorseDir + "/cli.toml"
var masterClient protocol.MasterClient
var globalConfig config.Config

func init() {
	cliCommand.AddCommand(versionCommand)

	// load cli config
	cliConfig, _, err := config.LoadConfig(configPath)
	if err != nil {
		base.Logger().Fatal("failed to load config", zap.Error(err),
			zap.String("path", configPath))
	}

	// create connection
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", cliConfig.Master.Host, cliConfig.Master.Port), grpc.WithInsecure())
	if err != nil {
		base.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	masterClient = protocol.NewMasterClient(conn)

	// load master config
	masterCfgJson, err := masterClient.GetMeta(context.Background(), &protocol.RequestInfo{})
	if err != nil {
		base.Logger().Fatal("failed to load master config", zap.Error(err))
	}
	err = json.Unmarshal([]byte(masterCfgJson.Config), &globalConfig)
	if err != nil {
		base.Logger().Fatal("failed to parse master config", zap.Error(err))
	}
}

func main() {
	if err := cliCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}

var cliCommand = &cobra.Command{
	Use:   "gorse-cli",
	Short: "CLI for gorse recommender system.",
}

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "gorse version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Name)
	},
}
