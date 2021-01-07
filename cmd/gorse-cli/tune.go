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
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/match"
	"github.com/zhenghaoz/gorse/storage"
	"log"
	"os"
	"time"
)

func tune(cmd *cobra.Command, args []string) {
	modelName := args[0]
	m, err := model.NewModel(modelName, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Load data
	var trainSet, testSet *match.DataSet
	if cmd.PersistentFlags().Changed("load-builtin") {
		name, _ := cmd.PersistentFlags().GetString("load-builtin")
		trainSet, testSet = match.LoadDataFromBuiltIn(name)
		log.Printf("Load built-in dataset %s\n", name)
	} else if cmd.PersistentFlags().Changed("load-csv") {
		name, _ := cmd.PersistentFlags().GetString("load-csv")
		sep, _ := cmd.PersistentFlags().GetString("csv-sep")
		header, _ := cmd.PersistentFlags().GetBool("csv-header")
		numTestUsers, _ := cmd.PersistentFlags().GetInt("n-test-users")
		seed, _ := cmd.PersistentFlags().GetInt("random-state")
		data := match.LoadDataFromCSV(name, sep, header)
		trainSet, testSet = data.Split(numTestUsers, int64(seed))
	} else {
		log.Println("Load default dataset ml-100k")
		// Load config
		cfg, _, err := config.LoadConfig("/home/zhenghaoz/.gorse/cli.toml")
		if err != nil {
			log.Fatal(err)
		}
		// Open database
		database, err := storage.Open(cfg.Database.Path)
		if err != nil {
			log.Fatal(err)
		}
		defer database.Close()
		// Load data
		data, _, err := match.LoadDataFromDatabase(database)
		if err != nil {
			log.Fatal(err)
		}
		numTestUsers, _ := cmd.PersistentFlags().GetInt("n-test-users")
		seed, _ := cmd.PersistentFlags().GetInt("random-state")
		trainSet, testSet = data.Split(numTestUsers, int64(seed))
	}
	// Load hyper-parameters
	grid := parseParamFlags(cmd)
	log.Printf("Load hyper-parameters grid: %v\n", grid)
	// Load runtime options
	fitConfig := &config.FitConfig{}
	fitConfig.Verbose, _ = cmd.PersistentFlags().GetInt("verbose")
	fitConfig.Jobs, _ = cmd.PersistentFlags().GetInt("jobs")
	fitConfig.TopK, _ = cmd.PersistentFlags().GetInt("top-k")
	fitConfig.Candidates, _ = cmd.PersistentFlags().GetInt("n-negatives")
	// Cross validation
	start := time.Now()
	grid.FillIfNotExist(m.GetParamsGrid())
	log.Printf("Tune hyper-parameters on: %v\n", grid)
	result := match.RandomSearchCV(m, trainSet, testSet, grid, 10, 0)
	elapsed := time.Since(start)
	// Render table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"#", "NDCG@10", "Precision@10", "Recall@10", "Params"})
	for i := range result.Params {
		score := result.Scores[i]
		table.Append([]string{
			fmt.Sprintf("%v", i),
			fmt.Sprintf("%v", score.NDCG),
			fmt.Sprintf("%v", score.Precision),
			fmt.Sprintf("%v", score.Recall),
			fmt.Sprintf("%v", result.Params[i]),
		})
	}
	table.Render()
	log.Printf("Complete cross validation (%v)\n", elapsed)
}
