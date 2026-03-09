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
	"encoding/csv"
	"fmt"
	"maps"
	"math"
	"os"
	"runtime"
	"slices"
	"strconv"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/common/heap"
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
		dataset, cfDataset, err := m.LoadDataFromDatabase(context.Background(), m.DataClient,
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
		train, test := dataset.Split(0.2, 0)

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"", "#interactions", "#positive", "#negative"})
		lo.Must0(table.Bulk([][]string{
			{"total", strconv.Itoa(dataset.Count()), strconv.Itoa(dataset.PositiveCount), strconv.Itoa(dataset.NegativeCount)},
			{"train", strconv.Itoa(train.Count()), strconv.Itoa(train.PositiveCount), strconv.Itoa(train.NegativeCount)},
			{"test", strconv.Itoa(test.Count()), strconv.Itoa(test.PositiveCount), strconv.Itoa(test.NegativeCount)},
		}))
		lo.Must0(table.Render())

		exportUserAUC, _ := cmd.Flags().GetBool("user-auc")
		go EvaluateAFM(cfg, train, test, exportUserAUC, &scores)
		EvaluateLLM(cfg, train, test, cfDataset.GetItems(), exportUserAUC, &scores)
		data := [][]string{{"Ranker", "GAUC"}}
		scores.Range(func(key, value any) bool {
			data = append(data, []string{
				key.(string),
				strconv.FormatFloat(value.(float64), 'f', 4, 32),
			})
			return true
		})
		table = tablewriter.NewWriter(os.Stdout)
		table.Header(data[0])
		lo.Must0(table.Bulk(data[1:]))
		lo.Must0(table.Render())
	},
}

func EvaluateAFM(cfg *config.Config, train, test *ctr.Dataset, exportUserAUC bool, scores *sync.Map) {
	ml := ctr.NewAFM(nil)
	ml.Fit(context.Background(), train, test,
		ctr.NewFitConfig().
			SetVerbose(10).
			SetJobs(runtime.NumCPU()).
			SetPatience(10))

	feedbackCount := make(map[int32]int)
	for i := 0; i < train.Count(); i++ {
		indices, _, _, target := train.Get(i)
		if target > 0 {
			userIndex := indices[0]
			feedbackCount[userIndex]++
		}
	}

	var features []lo.Tuple2[[]int32, []float32]
	var embeddings [][][]float32
	positives := make(map[int32][]int)
	negatives := make(map[int32][]int)
	for i := 0; i < test.Count(); i++ {
		indices, values, embedding, target := test.Get(i)
		features = append(features, lo.Tuple2[[]int32, []float32]{A: indices, B: values})
		embeddings = append(embeddings, embedding)
		userIndex := indices[0]
		if target > 0 {
			positives[userIndex] = append(positives[userIndex], i)
		} else {
			negatives[userIndex] = append(negatives[userIndex], i)
		}
	}
	predictions := ml.BatchInternalPredict(features, embeddings, runtime.NumCPU())

	var csvFile *os.File
	var csvWriter *csv.Writer
	if exportUserAUC {
		var err error
		csvFile, err = os.Create("AFM.csv")
		if err != nil {
			log.Logger().Error("failed to create AFM.csv", zap.Error(err))
			exportUserAUC = false
		} else {
			defer csvFile.Close()
			csvWriter = csv.NewWriter(csvFile)
			defer csvWriter.Flush()
			_ = csvWriter.Write([]string{"Feedback", "Candidates", "AUC"})
		}
	}

	var sum float32
	var count float32
	for userIndex, posIndices := range positives {
		negIndices := negatives[userIndex]
		if len(negIndices) == 0 || feedbackCount[userIndex] == 0 || feedbackCount[userIndex] > cfg.Recommend.ContextSize {
			continue
		}
		var posPredictions, negPredictions []float32
		for _, posIndex := range posIndices {
			posPredictions = append(posPredictions, predictions[posIndex])
		}
		for _, negIndex := range negIndices {
			negPredictions = append(negPredictions, predictions[negIndex])
		}
		userAUC := ctr.AUC(posPredictions, negPredictions)
		if exportUserAUC {
			_ = csvWriter.Write([]string{
				strconv.Itoa(feedbackCount[userIndex]),
				strconv.Itoa(len(posIndices) + len(negIndices)),
				fmt.Sprintf("%.4f", userAUC),
			})
		}
		userCount := float32(len(posIndices) + len(negIndices))
		sum += userAUC * userCount
		count += userCount
	}

	var score float64
	if count > 0 {
		score = float64(sum / count)
	}
	scores.Store("AFM", score)
}

func EvaluateLLM(cfg *config.Config, train, test *ctr.Dataset, items []data.Item, exportUserAUC bool, scores *sync.Map) {
	chat, err := logics.NewChatReranker(
		cfg.Recommend.Ranker.RerankerAPI,
		cfg.Recommend.Ranker.QueryTemplate,
		cfg.Recommend.Ranker.DocumentTemplate,
	)
	if err != nil {
		log.Logger().Fatal("failed to create chat ranker", zap.Error(err))
	}

	feedbacks := make(map[int32][]*logics.FeedbackItem)
	for i := 0; i < train.Count(); i++ {
		indices, _, _, target := train.Get(i)
		if target <= 0 {
			continue
		}
		userIndex := indices[0]
		itemIndex := indices[1] - int32(train.CountUsers())
		feedbacks[userIndex] = append(feedbacks[userIndex], &logics.FeedbackItem{
			Item: items[itemIndex],
		})
	}

	positives := make(map[int32][]int32)
	negatives := make(map[int32][]int32)
	for i := 0; i < test.Count(); i++ {
		indices, _, _, target := test.Get(i)
		userIndex := indices[0]
		itemIndex := indices[1] - int32(test.CountUsers())
		if target > 0 {
			positives[userIndex] = append(positives[userIndex], itemIndex)
		} else {
			negatives[userIndex] = append(negatives[userIndex], itemIndex)
		}
	}

	var csvFile *os.File
	var csvWriter *csv.Writer
	var csvMu sync.Mutex
	if exportUserAUC {
		var err error
		csvFile, err = os.Create(fmt.Sprintf("%s.csv", cfg.Recommend.Ranker.RerankerAPI.Model))
		if err != nil {
			log.Logger().Error("failed to create LLM.csv", zap.Error(err))
			exportUserAUC = false
		} else {
			defer csvFile.Close()
			csvWriter = csv.NewWriter(csvFile)
			defer csvWriter.Flush()
			_ = csvWriter.Write([]string{"Feedback", "Candidates", "AUC"})
		}
	}

	var sum atomic.Float32
	var count atomic.Float32
	lo.Must0(parallel.ForEach(context.Background(), slices.Collect(maps.Keys(positives)), runtime.NumCPU(), func(_ int, userIndex int32) {
		posIndices := positives[userIndex]
		negIndices := negatives[userIndex]
		if len(negIndices) == 0 {
			return
		}
		candidates := make([]*data.Item, 0, len(posIndices)+len(negIndices))
		positiveItems := mapset.NewSet[string]()
		negativeItems := mapset.NewSet[string]()
		for _, negIndex := range negIndices {
			item := items[negIndex]
			candidates = append(candidates, &item)
			negativeItems.Add(item.ItemId)
		}
		for _, posIndex := range posIndices {
			item := items[posIndex]
			candidates = append(candidates, &item)
			positiveItems.Add(item.ItemId)
		}
		feedback := feedbacks[int32(userIndex)]
		if len(feedback) == 0 || len(feedback) > cfg.Recommend.ContextSize {
			return
		}
		result, err := chat.Rank(context.Background(), &data.User{}, feedback, candidates)
		if err != nil {
			log.Logger().Error("failed to rank items for user", zap.Int32("user_index", userIndex), zap.Error(err))
			return
		}
		var posPredictions, negPredictions []float32
		for _, item := range result {
			if positiveItems.Contains(item.Id) {
				posPredictions = append(posPredictions, float32(item.Score))
			} else if negativeItems.Contains(item.Id) {
				negPredictions = append(negPredictions, float32(item.Score))
			}
		}
		userAUC := ctr.AUC(posPredictions, negPredictions)
		if exportUserAUC {
			csvMu.Lock()
			_ = csvWriter.Write([]string{
				strconv.Itoa(len(feedback)),
				strconv.Itoa(len(posIndices) + len(negIndices)),
				fmt.Sprintf("%.4f", userAUC),
			})
			csvMu.Unlock()
		}
		userCount := float32(len(posIndices) + len(negIndices))
		sum.Add(userAUC * userCount)
		count.Add(userCount)
	}))

	var score float64
	if count.Load() > 0 {
		score = float64(sum.Load() / count.Load())
	}
	scores.Store(cfg.Recommend.Ranker.RerankerAPI.Model, score)
}

func EvaluateEmbedding(cfg *config.Config, train, test dataset.CFSplit, embeddingExpr, textExpr string, topK, jobs int, scores *sync.Map) {
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
	var dimensions atomic.Int64
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
			if dimensions.Load() == 0 {
				dimensions.Store(int64(len(resp.Data[0].Embedding)))
			}
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
					if dim := dimensions.Swap(int64(len(e))); dim == 0 {
						dimensions.Store(int64(len(e)))
					} else if dim != int64(len(e)) {
						log.Logger().Fatal("inconsistent embedding dimensions",
							zap.Int64("expected", dim),
							zap.Int64("got", int64(len(e))))
					}
				}
			}
		}))
	}

	var (
		ndcg      atomic.Float32
		precision atomic.Float32
		recall    atomic.Float32
		count     atomic.Float32
	)
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

		h := heap.NewTopKFilter[int32, float32](topK)
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
				totalDistance -= floats.Euclidean(candidateEmbedding, shotEmbedding)
				validShots++
			}
			if validShots > 0 {
				h.Push(candidateIdx, totalDistance)
			}
		}

		if h.Len() == 0 {
			return
		}

		rankList := h.PopAllValues()
		ndcg.Add(cf.NDCG(targetSet, rankList))
		precision.Add(cf.Precision(targetSet, rankList))
		recall.Add(cf.Recall(targetSet, rankList))
		count.Add(1)
	}))

	var score cf.Score
	if count.Load() > 0 {
		score = cf.Score{
			NDCG:      ndcg.Load() / count.Load(),
			Precision: precision.Load() / count.Load(),
			Recall:    recall.Load() / count.Load(),
		}
	}
	scores.Store(fmt.Sprintf("%s (%d)", cfg.OpenAI.EmbeddingModel, dimensions.Load()), score)
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
		dimensions, _ := cmd.Flags().GetInt("embedding-dimensions")
		cfg.OpenAI.EmbeddingDimensions = dimensions

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

		topK, _ := cmd.Flags().GetInt("top")
		jobs, _ := cmd.Flags().GetInt("jobs")
		EvaluateEmbedding(cfg, train, test, embeddingColumn, textColumn, topK, jobs, &scores)
		data := [][]string{{
			"Embedding Model",
			fmt.Sprintf("NDCG@%d", topK),
			fmt.Sprintf("Precision@%d", topK),
			fmt.Sprintf("Recall@%d", topK),
		}}
		scores.Range(func(key, value any) bool {
			score := value.(cf.Score)
			data = append(data, []string{
				key.(string),
				fmt.Sprintf("%.4f", score.NDCG),
				fmt.Sprintf("%.4f", score.Precision),
				fmt.Sprintf("%.4f", score.Recall),
			})
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
	benchLLMCmd.PersistentFlags().Bool("user-auc", false, "Export user-level AUC scores to CSV file")
	benchEmbeddingCmd.PersistentFlags().IntP("top", "k", 10, "Number of top items to evaluate for each user")
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
