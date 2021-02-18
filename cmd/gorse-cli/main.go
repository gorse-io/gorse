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
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/protocol"
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
		logrus.Fatalf("cli: failed to load config from %v", configPath)
	}

	// create connection
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", cliConfig.Master.Host, cliConfig.Master.Port), grpc.WithInsecure())
	if err != nil {
		logrus.Fatalf("cli: failed to connect master (%v)", err)
	}
	masterClient = protocol.NewMasterClient(conn)

	// load master config
	masterCfgJson, err := masterClient.GetConfig(context.Background(), &protocol.Void{})
	if err != nil {
		logrus.Fatalf("cli: failed to load master config (%v)", err)
	}
	err = json.Unmarshal([]byte(masterCfgJson.Json), &globalConfig)
	if err != nil {
		logrus.Fatalf("cli: failed to parse master config (%v)", err)
	}
}

func main() {
	if err := cliCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}

var cliCommand = &cobra.Command{
	Use:   "gorse-cli",
	Short: "CLI for gorse recommender system",
}

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Check the version of gorse",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.VersionName)
	},
}
