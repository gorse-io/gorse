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
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"os"
)

func init() {
	cliCommand.AddCommand(clusterCommand)
}

var clusterCommand = &cobra.Command{
	Use:   "cluster",
	Short: "Get cluster information.",
	Run: func(cmd *cobra.Command, args []string) {
		// show cluster
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"role", "address"})
		table.Append([]string{
			fmt.Sprint("master"),
			fmt.Sprintf("%v:%v", globalConfig.Master.Host, globalConfig.Master.Port),
		})
		table.Render()
	},
}
