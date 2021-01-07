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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/match"
	"github.com/zhenghaoz/gorse/storage"
	"os"
	"strconv"
	"strings"
	"time"
)

/* Models */

/* Flags for parameters */

const (
	intFlag     = 0
	float64Flag = 1
)

type paramFlag struct {
	Type int
	Key  model.ParamName
	Name string
	Help string
}

var testParamFlags = []paramFlag{
	{float64Flag, model.Lr, "lr", "Learning rate"},
	{float64Flag, model.Reg, "reg", "Regularization strength"},
	{intFlag, model.NEpochs, "n-epochs", "Number of epochs"},
	{intFlag, model.NFactors, "n-factors", "Number of factors"},
	{float64Flag, model.InitMean, "init-mean", "Mean of gaussian initial parameters"},
	{float64Flag, model.InitStdDev, "init-std", "Standard deviation of gaussian initial parameters"},
	{float64Flag, model.NegWeight, "neg-weight", "Weight of negative samples in ALS."},
}

func parseParamFlags(cmd *cobra.Command) model.ParamsGrid {
	grid := make(model.ParamsGrid)
	for _, paramFlag := range testParamFlags {
		if cmd.PersistentFlags().Changed(paramFlag.Name) {
			text, err := cmd.PersistentFlags().GetString(paramFlag.Name)
			if err != nil {
				log.Fatal(err)
			}
			grid[paramFlag.Key] = parseParamList(text, paramFlag.Type)
		}
	}
	return grid
}

func parseParamList(text string, tp int) []interface{} {
	if len(text) == 0 {
		log.Fatal("empty string")
	}
	if text[0] == '[' && text[len(text)-1] == ']' {
		text = text[1 : len(text)-1]
	}
	paramTexts := strings.Split(text, ",")
	params := make([]interface{}, len(paramTexts))
	for i, paramText := range paramTexts {
		params[i] = parseParam(paramText, tp)
	}
	return params
}

func parseParam(text string, tp int) interface{} {
	switch tp {
	case intFlag:
		i, err := strconv.Atoi(text)
		if err != nil {
			log.Fatal(err)
		}
		return i
	case float64Flag:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			log.Fatal(err)
		}
		return f
	default:
		log.Fatal("unknown parameter type", tp)
		return nil
	}
}

func test(cmd *cobra.Command, args []string) {
	modelName := args[0]
	m, err := match.NewModel(modelName, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Load data
	var trainSet, testSet *match.DataSet
	if cmd.PersistentFlags().Changed("load-builtin") {
		name, _ := cmd.PersistentFlags().GetString("load-builtin")
		log.Infof("Load built-in dataset %s\n", name)
		trainSet, testSet = match.LoadDataFromBuiltIn(name)
	} else if cmd.PersistentFlags().Changed("load-csv") {
		name, _ := cmd.PersistentFlags().GetString("load-csv")
		sep, _ := cmd.PersistentFlags().GetString("csv-sep")
		header, _ := cmd.PersistentFlags().GetBool("csv-header")
		numTestUsers, _ := cmd.PersistentFlags().GetInt("n-test-users")
		seed, _ := cmd.PersistentFlags().GetInt("random-state")
		log.Infof("Load csv file %v", name)
		data := match.LoadDataFromCSV(name, sep, header)
		trainSet, testSet = data.Split(numTestUsers, int64(seed))
	} else {
		// Load config
		cfg, _, err := config.LoadConfig(match.GorseDir + "/cli.toml")
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
		log.Infof("Load data from %v", cfg.Database.Path)
		data, _, err := match.LoadDataFromDatabase(database)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("data set: #user = %v, #item = %v, #feedback = %v", data.UserCount(), data.ItemCount(), data.Count())
		numTestUsers, _ := cmd.PersistentFlags().GetInt("n-test-users")
		seed, _ := cmd.PersistentFlags().GetInt("random-state")
		trainSet, testSet = data.Split(numTestUsers, int64(seed))
	}
	log.Infof("train set: #user = %v, #item = %v, #feedback = %v", trainSet.UserCount(), trainSet.ItemCount(), trainSet.Count())
	log.Infof("test set: #user = %v, #item = %v, #feedback = %v", testSet.UserCount(), testSet.ItemCount(), testSet.Count())
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
	var result *match.ParamsSearchResult
	if grid.Len() == 0 {
		result = match.NewParamsSearchResult()
		score := m.Fit(trainSet, testSet, fitConfig)
		result.AddScore(nil, score)
	} else {
		a := match.GridSearchCV(m, trainSet, testSet, grid, 0)
		result = &a
	}
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
			fmt.Sprintf("%v", result.Params[i].ToString()),
		})
	}
	table.Render()
	log.Printf("Complete cross validation (%v)\n", elapsed)
}
