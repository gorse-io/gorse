// Copyright 2026 gorse Project Authors
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
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/model/cf"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/olekukonko/tablewriter"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:   "gorse-cli",
	Short: "Gorse command line tool",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var benchLLMCmd = &cobra.Command{
	Use:   "bench-llm",
	Short: "Benchmark LLM models for ranking",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			log.Logger().Fatal("failed to load config", zap.Error(err))
		}
		shots, _ := cmd.Flags().GetInt("shots")

		// Load dataset
		m := master.NewMaster(cfg, os.TempDir(), false, configPath)
		m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.DataTablePrefix,
			storage.WithIsolationLevel(m.Config.Database.MySQL.IsolationLevel))
		if err != nil {
			log.Logger().Fatal("failed to open data client", zap.Error(err))
		}
		evaluator := master.NewOnlineEvaluator(
			m.Config.Recommend.DataSource.PositiveFeedbackTypes,
			m.Config.Recommend.DataSource.ReadFeedbackTypes)
		ctrDataset, dataset, err := m.LoadDataFromDatabase(context.Background(), m.DataClient,
			m.Config.Recommend.DataSource.PositiveFeedbackTypes,
			m.Config.Recommend.DataSource.ReadFeedbackTypes,
			m.Config.Recommend.DataSource.ItemTTL,
			m.Config.Recommend.DataSource.PositiveFeedbackTTL,
			evaluator,
			nil)
		if err != nil {
			log.Logger().Fatal("failed to load dataset", zap.Error(err))
		}

		// Split dataset
		var scores sync.Map
		train, test := dataset.SplitLatest(shots)
		test.SampleUserNegatives(dataset, 99)

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"", "#users", "#items", "#interactions"})
		lo.Must0(table.Bulk([][]string{
			{"total", strconv.Itoa(dataset.CountUsers()), strconv.Itoa(dataset.CountItems()), strconv.Itoa(dataset.CountFeedback())},
			{"train", strconv.Itoa(train.CountUsers()), strconv.Itoa(train.CountItems()), strconv.Itoa(train.CountFeedback())},
			{"test", strconv.Itoa(test.CountUsers()), strconv.Itoa(test.CountItems()), strconv.Itoa(test.CountFeedback())},
		}))
		lo.Must0(table.Render())

		go EvaluateCF(train, test, &scores)
		go EvaluateAFM(ctrDataset, train, test, &scores)
		EvaluateLLM(cfg, train, test, &scores)
		data := [][]string{{"Model", "NDCG"}}
		scores.Range(func(key, value any) bool {
			score := value.(cf.Score)
			data = append(data, []string{key.(string), fmt.Sprintf("%.4f", score.NDCG)})
			return true
		})
		table = tablewriter.NewWriter(os.Stdout)
		table.Header(data[0])
		lo.Must0(table.Bulk(data[1:]))
		lo.Must0(table.Render())
	},
}

func EvaluateCF(train, test dataset.CFSplit, scores *sync.Map) {
	for name, model := range map[string]cf.MatrixFactorization{
		"ALS": cf.NewALS(nil),
		"BPR": cf.NewBPR(nil),
	} {
		score := model.Fit(context.Background(), train, test,
			cf.NewFitConfig().
				SetVerbose(10).
				SetJobs(runtime.NumCPU()).
				SetPatience(10))
		scores.Store(name, score)
	}
}

func EvaluateAFM(ctrDataset *ctr.Dataset, train, test dataset.CFSplit, scores *sync.Map) {
	ctrTrain, ctrTest := SplitCTRDataset(ctrDataset, train, test)
	ml := ctr.NewAFM(nil)
	ml.Fit(context.Background(), ctrTrain, ctrTest,
		ctr.NewFitConfig().
			SetVerbose(10).
			SetJobs(runtime.NumCPU()).
			SetPatience(10))

	buildCTRInput := func(user, item int32) ([]int32, []float32, [][]float32) {
		var (
			indices   []int32
			values    []float32
			embedding [][]float32
			position  int32
		)
		if ctrDataset.CountUsers() > 0 {
			indices = append(indices, user)
			values = append(values, 1)
			position += int32(ctrDataset.CountUsers())
		}
		if ctrDataset.CountItems() > 0 {
			indices = append(indices, position+item)
			values = append(values, 1)
			position += int32(ctrDataset.CountItems())
			if len(ctrDataset.ItemEmbeddings) > 0 && int(item) < len(ctrDataset.ItemEmbeddings) {
				embedding = ctrDataset.ItemEmbeddings[item]
			}
		}
		if ctrDataset.CountUsers() > 0 {
			if int(user) < len(ctrDataset.UserLabels) {
				for _, feature := range ctrDataset.UserLabels[user] {
					indices = append(indices, position+feature.A)
					values = append(values, feature.B)
				}
			}
			position += int32(ctrDataset.Index.CountUserLabels())
		}
		if ctrDataset.CountItems() > 0 {
			if int(item) < len(ctrDataset.ItemLabels) {
				for _, feature := range ctrDataset.ItemLabels[item] {
					indices = append(indices, position+feature.A)
					values = append(values, feature.B)
				}
			}
		}
		return indices, values, embedding
	}

	negatives := test.SampleUserNegatives(train, 99)
	userFeedback := test.GetUserFeedback()

	var sumNDCG float32
	var ndcgUsers float32
	for userIdx := 0; userIdx < test.CountUsers(); userIdx++ {
		positives := userFeedback[userIdx]
		if len(positives) == 0 {
			continue
		}
		targetSet := mapset.NewSet(positives...)
		candidatesSet := mapset.NewSet(positives...)
		for _, item := range negatives[userIdx] {
			candidatesSet.Add(item)
		}
		candidates := candidatesSet.ToSlice()
		if len(candidates) == 0 {
			continue
		}
		features := make([]lo.Tuple2[[]int32, []float32], len(candidates))
		embeddings := make([][][]float32, len(candidates))
		for i, item := range candidates {
			indices, values, embedding := buildCTRInput(int32(userIdx), item)
			features[i] = lo.Tuple2[[]int32, []float32]{A: indices, B: values}
			embeddings[i] = embedding
		}
		predictions := ml.BatchInternalPredict(features, embeddings, runtime.NumCPU())

		type scoredItem struct {
			item  int32
			score float32
		}
		scored := make([]scoredItem, 0, len(candidates))
		for i, item := range candidates {
			scored = append(scored, scoredItem{item: item, score: predictions[i]})
		}
		sort.Slice(scored, func(i, j int) bool {
			return scored[i].score > scored[j].score
		})
		rankList := make([]int32, 0, len(scored))
		for _, s := range scored {
			rankList = append(rankList, s.item)
		}
		sumNDCG += cf.NDCG(targetSet, rankList)
		ndcgUsers++
	}

	ndcg := float32(0)
	if ndcgUsers > 0 {
		ndcg = sumNDCG / ndcgUsers
	}
	scores.Store("AFM", cf.Score{NDCG: ndcg})
}

func SplitCTRDataset(ctrDataset *ctr.Dataset, train, test dataset.CFSplit) (*ctr.Dataset, *ctr.Dataset) {
	makeKey := func(user, item int32) uint64 {
		return (uint64(uint32(user)) << 32) | uint64(uint32(item))
	}
	newSubset := func(capacity int) *ctr.Dataset {
		return &ctr.Dataset{
			Index:                  ctrDataset.Index,
			UserLabels:             ctrDataset.UserLabels,
			ItemLabels:             ctrDataset.ItemLabels,
			ItemEmbeddings:         ctrDataset.ItemEmbeddings,
			ItemEmbeddingIndex:     ctrDataset.ItemEmbeddingIndex,
			ItemEmbeddingDimension: ctrDataset.ItemEmbeddingDimension,
			Users:                  make([]int32, 0, capacity),
			Items:                  make([]int32, 0, capacity),
			Target:                 make([]float32, 0, capacity),
		}
	}
	appendSample := func(dataSet *ctr.Dataset, user, item int32, target float32) {
		dataSet.Users = append(dataSet.Users, user)
		dataSet.Items = append(dataSet.Items, item)
		dataSet.Target = append(dataSet.Target, target)
		if target > 0 {
			dataSet.PositiveCount++
		} else {
			dataSet.NegativeCount++
		}
	}

	trainSet := newSubset(ctrDataset.Count())
	testSet := newSubset(test.CountFeedback() + test.CountUsers()*100)

	testPositive := mapset.NewSet[uint64]()
	for userIdx, items := range test.GetUserFeedback() {
		for _, itemIdx := range items {
			testPositive.Add(makeKey(int32(userIdx), itemIdx))
		}
	}
	negatives := test.SampleUserNegatives(train, 99)
	testNegative := mapset.NewSet[uint64]()
	for userIdx, items := range negatives {
		for _, itemIdx := range items {
			testNegative.Add(makeKey(int32(userIdx), itemIdx))
		}
	}
	addedNegative := make(map[int32]bool)
	for i := 0; i < ctrDataset.Count(); i++ {
		user := ctrDataset.Users[i]
		item := ctrDataset.Items[i]
		target := ctrDataset.Target[i]
		key := makeKey(user, item)
		if target > 0 && testPositive.Contains(key) {
			appendSample(testSet, user, item, target)
		} else if target <= 0 && !addedNegative[user] {
			appendSample(testSet, user, item, target)
			addedNegative[user] = true
		} else if !testPositive.Contains(key) && !testNegative.Contains(key) {
			appendSample(trainSet, user, item, target)
		}
	}
	return trainSet, testSet
}

func EvaluateLLM(cfg *config.Config, train, test dataset.CFSplit, scores *sync.Map) {
	chat, err := logics.NewChatRanker(cfg.OpenAI, cfg.Recommend.Ranker.Prompt)
	if err != nil {
		log.Logger().Fatal("failed to create chat ranker", zap.Error(err))
	}

	var sum atomic.Float32
	var count atomic.Float32
	negatives := test.SampleUserNegatives(train, 99)
	lo.Must0(parallel.Detachable(context.Background(), test.CountUsers(), runtime.NumCPU(), 10, func(pCtx *parallel.Context, userIdx int) {
		targetSet := mapset.NewSet(test.GetUserFeedback()[userIdx]...)
		negativeSample := negatives[userIdx]
		candidates := make([]*data.Item, 0, targetSet.Cardinality()+len(negativeSample))
		for _, itemIdx := range negativeSample {
			candidates = append(candidates, &test.GetItems()[itemIdx])
		}
		if len(test.GetUserFeedback()[userIdx]) == 0 {
			return
		}
		for _, itemIdx := range test.GetUserFeedback()[userIdx] {
			candidates = append(candidates, &test.GetItems()[itemIdx])
		}
		feedback := make([]*logics.FeedbackItem, 0, len(train.GetUserFeedback()[int32(userIdx)]))
		for _, itemIdx := range train.GetUserFeedback()[int32(userIdx)] {
			feedback = append(feedback, &logics.FeedbackItem{
				Item: train.GetItems()[itemIdx],
			})
		}
		pCtx.Detach()
		start := time.Now()
		result, err := chat.Rank(context.Background(), &data.User{}, feedback, candidates)
		if err != nil {
			if apiError, ok := err.(*openai.APIError); ok && apiError.HTTPStatusCode == 421 {
				return
			}
			log.Logger().Fatal("failed to rank items for user", zap.Int("user", userIdx), zap.Error(err))
		}
		duration := time.Since(start)
		pCtx.Attach()
		var score float32
		if len(result) > 0 {
			score = cf.NDCG(targetSet, lo.Map(result, func(itemId string, _ int) int32 {
				return train.GetItemDict().Id(itemId)
			}))
		} else {
			score = 0
		}
		sum.Add(score)
		count.Add(1)
		log.Logger().Info("LLM ranking result",
			zap.Int("user", userIdx),
			zap.Int("feedback", len(feedback)),
			zap.Int("candidates", len(candidates)),
			zap.Int("results", len(result)),
			zap.Float32("user_NDCG", score),
			zap.Float32("avg_NDCG", sum.Load()/count.Load()),
			zap.Duration("duration", duration),
		)
	}))

	score := sum.Load() / count.Load()
	scores.Store(cfg.OpenAI.ChatCompletionModel, cf.Score{NDCG: score})
}

func EvaluateEmbedding(cfg *config.Config, train, test dataset.CFSplit, embeddingExpr, textExpr string, jobs int, scores *sync.Map) {
	// Compile expression
	var embeddingProgram, textProgram *vm.Program
	var err error
	if embeddingExpr != "" {
		embeddingProgram, err = expr.Compile(embeddingExpr, expr.Env(map[string]any{
			"item": data.Item{},
		}))
		if err != nil {
			log.Logger().Fatal("failed to compile embedding expression", zap.Error(err))
		}
	} else if textExpr != "" {
		textProgram, err = expr.Compile(textExpr, expr.Env(map[string]any{
			"item": data.Item{},
		}))
		if err != nil {
			log.Logger().Fatal("failed to compile text expression", zap.Error(err))
		}
	} else {
		log.Logger().Fatal("one of embedding-column or text-column is required")
	}

	// Extract embeddings
	embeddings := make([][]float32, test.CountItems())
	if textExpr != "" {
		clientConfig := openai.DefaultConfig(cfg.OpenAI.AuthToken)
		clientConfig.BaseURL = cfg.OpenAI.BaseURL
		client := openai.NewClientWithConfig(clientConfig)
		// Generate embeddings
		bar := progressbar.Default(int64(test.CountItems()))
		lo.Must0(parallel.For(context.Background(), test.CountItems(), jobs, func(i int) {
			_ = bar.Add(1)
			item := &test.GetItems()[i]
			result, err := expr.Run(textProgram, map[string]any{
				"item": *item,
			})
			if err != nil {
				return
			}
			text, ok := result.(string)
			if !ok {
				return
			}

			// Generate embedding
			resp, err := client.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
				Input:      text,
				Model:      openai.EmbeddingModel(cfg.OpenAI.EmbeddingModel),
				Dimensions: cfg.OpenAI.EmbeddingDimensions,
			})
			if err != nil {
				log.Logger().Error("failed to create embeddings", zap.String("item_id", item.ItemId), zap.Error(err))
				return
			}
			embeddings[i] = resp.Data[0].Embedding
		}))
	} else {
		lo.Must0(parallel.For(context.Background(), test.CountItems(), jobs, func(i int) {
			item := test.GetItems()[i]
			result, err := expr.Run(embeddingProgram, map[string]any{
				"item": item,
			})
			if err == nil {
				if e, ok := result.([]float32); ok {
					embeddings[i] = e
				}
			}
		}))
	}

	var sum atomic.Float32
	var count atomic.Float32
	negatives := test.SampleUserNegatives(train, 99)
	lo.Must0(parallel.For(context.Background(), test.CountUsers(), jobs, func(userIdx int) {
		targetSet := mapset.NewSet(test.GetUserFeedback()[userIdx]...)
		negativeSample := negatives[userIdx]
		if len(test.GetUserFeedback()[userIdx]) == 0 {
			return
		}

		candidates := make([]int32, 0, targetSet.Cardinality()+len(negativeSample))
		candidates = append(candidates, test.GetUserFeedback()[userIdx]...)
		candidates = append(candidates, negativeSample...)

		feedback := train.GetUserFeedback()[userIdx]
		if len(feedback) == 0 {
			return
		}

		type scoredItem struct {
			item  int32
			score float32
		}
		scored := make([]scoredItem, 0, len(candidates))
		for _, candidateIdx := range candidates {
			candidateEmbedding := embeddings[candidateIdx]
			if candidateEmbedding == nil {
				continue
			}
			var totalDistance float32
			var validShots int
			for _, shotIdx := range feedback {
				shotEmbedding := embeddings[shotIdx]
				if shotEmbedding == nil {
					continue
				}
				totalDistance += floats.Euclidean(candidateEmbedding, shotEmbedding)
				validShots++
			}
			if validShots > 0 {
				scored = append(scored, scoredItem{item: candidateIdx, score: totalDistance})
			}
		}

		if len(scored) == 0 {
			return
		}

		sort.Slice(scored, func(i, j int) bool {
			return scored[i].score < scored[j].score // Smaller distance is better
		})

		rankList := make([]int32, 0, len(scored))
		for _, s := range scored {
			rankList = append(rankList, s.item)
		}

		sum.Add(cf.NDCG(targetSet, rankList))
		count.Add(1)
	}))

	score := float32(0)
	if count.Load() > 0 {
		score = sum.Load() / count.Load()
	}
	scores.Store(cfg.OpenAI.EmbeddingModel, cf.Score{NDCG: score})
}

var benchEmbeddingCmd = &cobra.Command{
	Use:   "bench-embedding",
	Short: "Benchmark embedding models for item-to-item",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			log.Logger().Fatal("failed to load config", zap.Error(err))
		}
		shots, _ := cmd.Flags().GetInt("shots")
		embeddingColumn, _ := cmd.Flags().GetString("embedding-column")
		textColumn, _ := cmd.Flags().GetString("text-column")
		if embeddingColumn == "" && textColumn == "" {
			log.Logger().Fatal("one of embedding-column or text-column is required")
		}

		// Load dataset
		m := master.NewMaster(cfg, os.TempDir(), false, configPath)
		m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.DataTablePrefix,
			storage.WithIsolationLevel(m.Config.Database.MySQL.IsolationLevel))
		if err != nil {
			log.Logger().Fatal("failed to open data client", zap.Error(err))
		}
		evaluator := master.NewOnlineEvaluator(
			m.Config.Recommend.DataSource.PositiveFeedbackTypes,
			m.Config.Recommend.DataSource.ReadFeedbackTypes)
		_, dataset, err := m.LoadDataFromDatabase(context.Background(), m.DataClient,
			m.Config.Recommend.DataSource.PositiveFeedbackTypes,
			m.Config.Recommend.DataSource.ReadFeedbackTypes,
			m.Config.Recommend.DataSource.ItemTTL,
			m.Config.Recommend.DataSource.PositiveFeedbackTTL,
			evaluator,
			nil)
		if err != nil {
			log.Logger().Fatal("failed to load dataset", zap.Error(err))
		}

		// Override config
		if cmd.Flags().Changed("embedding-model") {
			cfg.OpenAI.EmbeddingModel = cmd.Flag("embedding-model").Value.String()
		}
		if cmd.Flags().Changed("embedding-dimensions") {
			dimensions, _ := cmd.Flags().GetInt("embedding-dimensions")
			cfg.OpenAI.EmbeddingDimensions = dimensions
		}

		// Split dataset
		var scores sync.Map
		train, test := dataset.SplitLatest(shots)
		test.SampleUserNegatives(dataset, 99)

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"", "#users", "#items", "#interactions"})
		lo.Must0(table.Bulk([][]string{
			{"total", strconv.Itoa(dataset.CountUsers()), strconv.Itoa(dataset.CountItems()), strconv.Itoa(dataset.CountFeedback())},
			{"train", strconv.Itoa(train.CountUsers()), strconv.Itoa(train.CountItems()), strconv.Itoa(train.CountFeedback())},
			{"test", strconv.Itoa(test.CountUsers()), strconv.Itoa(test.CountItems()), strconv.Itoa(test.CountFeedback())},
		}))
		lo.Must0(table.Render())

		jobs, _ := cmd.Flags().GetInt("jobs")
		EvaluateEmbedding(cfg, train, test, embeddingColumn, textColumn, jobs, &scores)
		data := [][]string{{"Model", "NDCG"}}
		scores.Range(func(key, value any) bool {
			score := value.(cf.Score)
			data = append(data, []string{key.(string), fmt.Sprintf("%.4f", score.NDCG)})
			return true
		})
		table = tablewriter.NewWriter(os.Stdout)
		table.Header(data[0])
		lo.Must0(table.Bulk(data[1:]))
		lo.Must0(table.Render())
	},
}

func init() {
	rootCmd.PersistentFlags().StringP("config", "c", "", "Path to configuration file")
	rootCmd.PersistentFlags().IntP("jobs", "j", runtime.NumCPU(), "Number of jobs to run in parallel")
	rootCmd.AddCommand(benchLLMCmd)
	rootCmd.AddCommand(benchEmbeddingCmd)
	benchLLMCmd.PersistentFlags().IntP("shots", "s", math.MaxInt, "Number of shots for each user")
	benchEmbeddingCmd.PersistentFlags().IntP("shots", "s", math.MaxInt, "Number of shots for each user")
	benchEmbeddingCmd.PersistentFlags().Int("embedding-dimensions", 0, "Embedding dimensions")
	benchEmbeddingCmd.PersistentFlags().String("embedding-model", "", "Embedding model")
	benchEmbeddingCmd.PersistentFlags().String("embedding-column", "", "Column name of embedding in item label")
	benchEmbeddingCmd.PersistentFlags().String("text-column", "", "Column name of text in item label")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Logger().Fatal("failed to execute command", zap.Error(err))
	}
}
