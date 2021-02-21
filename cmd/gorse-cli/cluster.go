// Copyright 2021 gorse Project Authors
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
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/protocol"
	"os"
)

func init() {
	cliCommand.AddCommand(clusterCommand)
}

var clusterCommand = &cobra.Command{
	Use:   "cluster",
	Short: "Get cluster information.",
	Run: func(cmd *cobra.Command, args []string) {
		cluster, err := masterClient.GetCluster(context.Background(), &protocol.Void{})
		if err != nil {
			log.Fatalf("cli: failed to get cluster information (%v)", err)
		}
		// show cluster
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"role", "address"})
		table.Append([]string{"master", cluster.Master})
		for _, addr := range cluster.Servers {
			table.Append([]string{"server", addr})
		}
		for _, addr := range cluster.Workers {
			table.Append([]string{"worker", addr})
		}
		table.Render()
	},
}
