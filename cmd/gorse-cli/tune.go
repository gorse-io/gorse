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
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model/ctr"
	"github.com/zhenghaoz/gorse/model/pr"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"os"
	"runtime"
	"time"
)

func init() {
	cliCommand.AddCommand(tuneCommand)
	// test match model
	tuneCommand.AddCommand(tuneCFCommand)
	tuneCFCommand.PersistentFlags().StringArray("feedback-type", nil, "Set feedback type.")
	tuneCFCommand.PersistentFlags().String("load-builtin", "", "load data from built-in")
	tuneCFCommand.PersistentFlags().String("load-csv", "", "load data from CSV file")
	tuneCFCommand.PersistentFlags().String("load-database", "", "load data from database")
	tuneCFCommand.PersistentFlags().String("csv-sep", "\t", "load CSV file with separator")
	tuneCFCommand.PersistentFlags().String("csv-format", "", "load CSV file with header")
	tuneCFCommand.PersistentFlags().Bool("csv-header", false, "load CSV file with header")
	tuneCFCommand.PersistentFlags().Int("verbose", 1, "Verbose period")
	tuneCFCommand.PersistentFlags().Int("jobs", runtime.NumCPU(), "Number of jobs for model fitting")
	tuneCFCommand.PersistentFlags().Int("top-k", 10, "Length of recommendation list")
	tuneCFCommand.PersistentFlags().Int("n-negatives", 100, "Number of users for sampled test set")
	tuneCFCommand.PersistentFlags().Int("n-test-users", 0, "Number of users for sampled test set")
	tuneCFCommand.PersistentFlags().IntP("n-trials", "t", 10, "Number of trials")
	for _, paramFlag := range matchParamFlags {
		tuneCFCommand.PersistentFlags().String(paramFlag.Name, "", paramFlag.Help)
	}
	// test rank model
	tuneCommand.AddCommand(tuneFMCommand)
	tuneFMCommand.PersistentFlags().String("load-builtin", "", "load data from built-in")
	tuneFMCommand.PersistentFlags().String("load-database", "", "load data from database")
	tuneFMCommand.PersistentFlags().Float32("test-ratio", 0.2, "Test ratio.")
	tuneFMCommand.PersistentFlags().String("task", "r", "Task for ranking (c - classification, r - regression)")
	tuneFMCommand.PersistentFlags().Int("verbose", 1, "Verbose period")
	tuneFMCommand.PersistentFlags().Int("jobs", runtime.NumCPU(), "Number of jobs for model fitting")
	tuneFMCommand.PersistentFlags().IntP("n-trials", "t", 10, "Number of trials")
	for _, paramFlag := range rankParamFlags {
		tuneFMCommand.PersistentFlags().String(paramFlag.Name, "", paramFlag.Help)
	}
}

var tuneCommand = &cobra.Command{
	Use:   "tune",
	Short: "tune recommendation model by random search",
}

var tuneCFCommand = &cobra.Command{
	Use:   "cf",
	Short: "Tune match model by random search",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modelName := args[0]
		m, err := pr.NewModel(modelName, nil)
		if err != nil {
			base.Logger().Fatal("failed to create model", zap.Error(err))
		}
		// Load data
		var trainSet, testSet *pr.DataSet
		if cmd.PersistentFlags().Changed("load-builtin") {
			name, _ := cmd.PersistentFlags().GetString("load-builtin")
			trainSet, testSet, err = pr.LoadDataFromBuiltIn(name)
			if err != nil {
				base.Logger().Fatal("failed to load built-in dataset", zap.Error(err),
					zap.String("name", name))
			}
			base.Logger().Info("load built-in dataset", zap.String("name", name))
		} else if cmd.PersistentFlags().Changed("load-csv") {
			name, _ := cmd.PersistentFlags().GetString("load-csv")
			sep, _ := cmd.PersistentFlags().GetString("csv-sep")
			header, _ := cmd.PersistentFlags().GetBool("csv-header")
			numTestUsers, _ := cmd.PersistentFlags().GetInt("n-test-users")
			seed, _ := cmd.PersistentFlags().GetInt("random-state")
			data := pr.LoadDataFromCSV(name, sep, header)
			trainSet, testSet = data.Split(numTestUsers, int64(seed))
		} else {
			feedbackTypes, _ := cmd.PersistentFlags().GetStringArray("feedback-type")
			numTestUsers, _ := cmd.PersistentFlags().GetInt("n-test-users")
			seed, _ := cmd.PersistentFlags().GetInt("random-state")
			// Open database
			database, err := data.Open(globalConfig.Database.DataStore)
			if err != nil {
				base.Logger().Fatal("failed to connect database", zap.Error(err))
			}
			defer database.Close()
			// Load data
			data, _, _, err := pr.LoadDataFromDatabase(database, feedbackTypes)
			if err != nil {
				base.Logger().Fatal("failed to load data from database", zap.Error(err))
			}
			if data.Count() == 0 {
				base.Logger().Warn("empty dataset")
			}
			base.Logger().Info("load dataset from database",
				zap.Int("n_users", data.UserCount()),
				zap.Int("n_items", data.ItemCount()),
				zap.Int("n_feedbacks", data.Count()))
			trainSet, testSet = data.Split(numTestUsers, int64(seed))
		}
		// Load hyper-parameters
		grid := parseParamFlags(cmd)
		base.Logger().Info("load hyper-parameters grid", zap.Any("grid", grid))
		// Load runtime options
		fitConfig := &pr.FitConfig{}
		fitConfig.Verbose, _ = cmd.PersistentFlags().GetInt("verbose")
		fitConfig.Jobs, _ = cmd.PersistentFlags().GetInt("jobs")
		fitConfig.TopK, _ = cmd.PersistentFlags().GetInt("top-k")
		fitConfig.Candidates, _ = cmd.PersistentFlags().GetInt("n-negatives")
		// Cross validation
		start := time.Now()
		grid.Fill(m.GetParamsGrid())
		base.Logger().Info("tune hyper-parameters", zap.Any("grid", grid))
		numTrials, _ := cmd.PersistentFlags().GetInt("n-trials")
		result := pr.RandomSearchCV(m, trainSet, testSet, grid, numTrials, 0, fitConfig)
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
		base.Logger().Info("complete cross validation", zap.String("time", elapsed.String()))
	},
}

var tuneFMCommand = &cobra.Command{
	Use:   "fm",
	Short: "Tune rank model by random search.",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		// Load data
		var trainSet, testSet *ctr.Dataset
		if cmd.PersistentFlags().Changed("load-builtin") {
			name, _ := cmd.PersistentFlags().GetString("load-builtin")
			base.Logger().Info("load built-in dataset", zap.String("name", name))
			trainSet, testSet, err = ctr.LoadDataFromBuiltIn(name)
			if err != nil {
				base.Logger().Fatal("failed to load built-in dataset", zap.Error(err))
			}
		} else {
			// load dataset
			feedbackTypes, _ := cmd.PersistentFlags().GetStringArray("feedback-type")
			// Open database
			database, err := data.Open(globalConfig.Database.DataStore)
			if err != nil {
				base.Logger().Fatal("failed to connect database", zap.Error(err))
			}
			defer database.Close()
			seed, _ := cmd.PersistentFlags().GetInt64("seed")
			testRatio, _ := cmd.PersistentFlags().GetFloat32("test-ratio")
			dataSet, err := ctr.LoadDataFromDatabase(database, feedbackTypes)
			if err != nil {
				base.Logger().Fatal("failed to load data from database", zap.Error(err))
			}
			base.Logger().Info("load data from database",
				zap.Int("n_users", dataSet.UserCount()),
				zap.Int("n_items", dataSet.ItemCount()),
				zap.Int("n_positives", dataSet.PositiveCount))
			if dataSet.PositiveCount == 0 {
				base.Logger().Fatal("empty dataset")
			}
			trainSet, testSet = dataSet.Split(testRatio, seed)
			testSet.NegativeSample(1, trainSet, 0)
		}
		// Load hyper-parameters
		grid := parseParamFlags(cmd)
		base.Logger().Info("load hyper-parameters grid", zap.Any("grid", grid))
		// Load runtime options
		fitConfig := &ctr.FitConfig{}
		fitConfig.Verbose, _ = cmd.PersistentFlags().GetInt("verbose")
		fitConfig.Jobs, _ = cmd.PersistentFlags().GetInt("jobs")
		// Cross validation
		task, _ := cmd.PersistentFlags().GetString("task")
		m := ctr.NewFM(ctr.FMTask(task), nil)
		start := time.Now()
		grid.Fill(m.GetParamsGrid())
		base.Logger().Info("tune hyper-parameters", zap.Any("grid", grid))
		numTrials, _ := cmd.PersistentFlags().GetInt("n-trials")
		result := ctr.RandomSearchCV(m, trainSet, testSet, grid, numTrials, 0, fitConfig)
		elapsed := time.Since(start)
		// Render table
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"#", result.BestScore.GetName(), "Params"})
		for i := range result.Params {
			score := result.Scores[i]
			table.Append([]string{
				fmt.Sprintf("%v", i),
				fmt.Sprintf("%v", score.GetValue()),
				fmt.Sprintf("%v", result.Params[i].ToString()),
			})
		}
		table.Render()
		base.Logger().Info("complete cross validation", zap.String("time", elapsed.String()))
	},
}
