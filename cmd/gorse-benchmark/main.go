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
	"os"
	"runtime"
	"sort"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
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
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:   "gorse-benchmark",
	Short: "Gorse Benchmarking Tool",
}

var llmCmd = &cobra.Command{
	Use:   "llm",
	Short: "Benchmark LLM models",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			log.Logger().Fatal("failed to load config", zap.Error(err))
		}
		// Load dataset
		m := master.NewMaster(cfg, os.TempDir(), false)
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
		fmt.Println("Dataset loaded:")
		fmt.Printf("  Users: %d\n", dataset.CountUsers())
		fmt.Printf("  Items: %d\n", dataset.CountItems())
		// fmt.Printf("  Positive Feedbacks: %d\n", dataset.CountPositive())
		// fmt.Printf("  Negative Feedbacks: %d\n", dataset.CountNegative())
		// Split dataset
		var scores sync.Map
		train, test := dataset.SplitLatest()
		test.SampleUserNegatives(dataset, 99)
		// go EvaluateCF(train, test, &scores)
		EvaluateAFM(ctrDataset, train, test)
		// EvaluateLLM(cfg, train, test)
		scores.Range(func(key, value any) bool {
			fmt.Printf("%s score: %v\n", key.(string), value)
			return true
		})
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

func EvaluateAFM(ctrDataset *ctr.Dataset, train, test dataset.CFSplit) float32 {
	ctrTrain, ctrTest := SplitCTRDataset(ctrDataset, train, test)
	fmt.Println(ctrTrain.PositiveCount, ctrTrain.NegativeCount)
	fmt.Println(ctrTest.PositiveCount, ctrTest.NegativeCount)
	ml := ctr.NewAFM(nil)
	ml.Fit(context.Background(), ctrTrain, ctrTest,
		ctr.NewFitConfig().
			SetVerbose(10).
			SetJobs(runtime.NumCPU()).
			SetPatience(10))

	features := make([]lo.Tuple2[[]int32, []float32], ctrTest.Count())
	embeddings := make([][][]float32, ctrTest.Count())
	users := make([]int32, ctrTest.Count())
	items := make([]int32, ctrTest.Count())
	targets := make([]float32, ctrTest.Count())
	for i := 0; i < ctrTest.Count(); i++ {
		indices, values, embedding, target := ctrTest.Get(i)
		features[i] = lo.Tuple2[[]int32, []float32]{A: indices, B: values}
		embeddings[i] = embedding
		users[i] = ctrTest.Users[i]
		items[i] = ctrTest.Items[i]
		targets[i] = target
	}
	allPrediction := ml.BatchInternalPredict(features, embeddings, runtime.NumCPU())

	userPositives := make(map[int32]mapset.Set[int32])
	userSamples := make(map[int32][]int)
	for i, user := range users {
		userSamples[user] = append(userSamples[user], i)
		if targets[i] > 0 {
			if _, ok := userPositives[user]; !ok {
				userPositives[user] = mapset.NewSet[int32]()
			}
			userPositives[user].Add(items[i])
		}
	}

	var sumNDCG float32
	var ndcgUsers float32
	for user, indices := range userSamples {
		targetSet, ok := userPositives[user]
		if !ok || targetSet.Cardinality() == 0 {
			continue
		}
		type scoredItem struct {
			item  int32
			score float32
		}
		scored := make([]scoredItem, 0, len(indices))
		for _, idx := range indices {
			scored = append(scored, scoredItem{item: items[idx], score: allPrediction[idx]})
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

	fmt.Println("AFM NDCG:", ndcg)
	return ndcg
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

func EvaluateLLM(cfg *config.Config, train, test dataset.CFSplit) float32 {
	fmt.Println("Evaluating LLM...")
	chat, err := logics.NewChatRanker(cfg.OpenAI, cfg.Recommend.Ranker.Prompt)
	if err != nil {
		log.Logger().Fatal("failed to create chat ranker", zap.Error(err))
	}

	var sum atomic.Float32
	var count atomic.Float32
	negatives := test.SampleUserNegatives(train, 99)
	parallel.Detachable(context.Background(), test.CountUsers(), 1, 1, func(pCtx *parallel.Context, userIdx int) {
		targetSet := mapset.NewSet(test.GetUserFeedback()[userIdx]...)
		negativeSample := negatives[userIdx]
		candidates := make([]*data.Item, 0, targetSet.Cardinality()+len(negativeSample))
		for _, itemIdx := range negativeSample {
			candidates = append(candidates, &test.GetItems()[itemIdx])
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
		result, err := chat.Rank(context.Background(), &data.User{}, feedback, candidates)
		if err != nil {
			if apiError, ok := err.(*openai.APIError); ok && apiError.HTTPStatusCode == 421 {
				return
			}
			log.Logger().Fatal("failed to rank items for user", zap.Int("user", userIdx), zap.Error(err))
		}
		pCtx.Attach()
		score := cf.NDCG(targetSet, lo.Map(result, func(itemId string, _ int) int32 {
			return train.GetItemDict().Id(itemId)
		}))
		sum.Add(score)
		count.Add(1)
		log.Logger().Info("LLM ranking result",
			zap.Int("user", userIdx),
			zap.Int("feedback", len(feedback)),
			zap.Int("candidates", len(candidates)),
			zap.Int("results", len(result)),
			zap.Float32("NDCG", score),
		)
	})

	score := sum.Load() / count.Load()
	return score
}

func init() {
	rootCmd.PersistentFlags().StringP("config", "c", "", "Path to configuration file")
	rootCmd.AddCommand(llmCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Logger().Fatal("failed to execute command", zap.Error(err))
	}
}
