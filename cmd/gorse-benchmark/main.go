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
	"log"
	"os"
	"runtime"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/samber/lo"
	"github.com/samber/lo/mutable"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"go.uber.org/atomic"
	"golang.org/x/term"
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
			log.Fatalf("failed to load config: %v", err)
		}
		// Load dataset
		m := master.NewMaster(cfg, os.TempDir(), false)
		m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.DataTablePrefix,
			storage.WithIsolationLevel(m.Config.Database.MySQL.IsolationLevel))
		if err != nil {
			log.Fatalf("failed to open data client: %v", err)
		}
		evaluator := master.NewOnlineEvaluator(
			m.Config.Recommend.DataSource.PositiveFeedbackTypes,
			m.Config.Recommend.DataSource.ReadFeedbackTypes)
		dataset, _, err := m.LoadDataFromDatabase(context.Background(), m.DataClient,
			m.Config.Recommend.DataSource.PositiveFeedbackTypes,
			m.Config.Recommend.DataSource.ReadFeedbackTypes,
			m.Config.Recommend.DataSource.ItemTTL,
			m.Config.Recommend.DataSource.PositiveFeedbackTTL,
			evaluator,
			nil)
		if err != nil {
			log.Fatalf("failed to load dataset: %v", err)
		}
		fmt.Println("Dataset loaded:")
		fmt.Printf("  Users: %d\n", dataset.CountUsers())
		fmt.Printf("  Items: %d\n", dataset.CountItems())
		fmt.Printf("  Positive Feedbacks: %d\n", dataset.CountPositive())
		fmt.Printf("  Negative Feedbacks: %d\n", dataset.CountNegative())
		// Split dataset
		train, test := dataset.Split(0.2, 42)
		EvaluateAFM(train, test)
		// EvaluateLLM(cfg, train, test, aux.GetItems())
	},
}

func EvaluateLLM(cfg *config.Config, train, test dataset.CTRSplit, items []data.Item) float32 {
	PrintHorizontalLine("-")
	fmt.Println("Evaluating LLM...")
	chat, err := logics.NewChatRanker(cfg.OpenAI, cfg.Recommend.Ranker.Prompt)
	if err != nil {
		log.Fatalf("failed to create chat ranker: %v", err)
	}

	userTrain := make(map[int32][]int32, train.CountUsers())
	for i := 0; i < train.Count(); i++ {
		indices, _, _, target := train.Get(i)
		userId := indices[0]
		itemId := indices[1] - int32(train.CountUsers())
		if target > 0 {
			userTrain[userId] = append(userTrain[userId], itemId)
		}
	}

	userTest := make(map[int32][]int32, test.CountUsers())
	userPositive := make(map[int32]mapset.Set[int32])
	userNegative := make(map[int32]mapset.Set[int32])
	for i := 0; i < test.Count(); i++ {
		indices, _, _, target := test.Get(i)
		userId := indices[0]
		itemId := indices[1] - int32(test.CountUsers())
		userTest[userId] = append(userTest[userId], itemId)
		if target > 0 {
			if _, ok := userPositive[userId]; !ok {
				userPositive[userId] = mapset.NewSet[int32]()
			}
			userPositive[userId].Add(itemId)
		} else {
			if _, ok := userNegative[userId]; !ok {
				userNegative[userId] = mapset.NewSet[int32]()
			}
			userNegative[userId].Add(itemId)
		}
	}

	var sumAUC atomic.Float32
	var validUsers atomic.Float32
	parallel.Detachable(context.Background(), len(userTest), runtime.NumCPU(), 100, func(pCtx *parallel.Context, userIdx int) {
		userId := int32(userIdx)
		testItems := userTest[userId]
		if len(userTrain[userId]) > 100 || len(userTrain[userId]) == 0 {
			return
		}
		if _, ok := userPositive[userId]; !ok {
			return
		}
		if _, ok := userNegative[userId]; !ok {
			return
		}
		candidates := make([]*data.Item, 0, len(testItems))
		for _, itemId := range testItems {
			candidates = append(candidates, &items[itemId])
		}
		mutable.Reverse(candidates)
		feedback := make([]*logics.FeedbackItem, 0, len(testItems))
		for _, itemId := range userTrain[userId] {
			feedback = append(feedback, &logics.FeedbackItem{
				Item: items[itemId],
			})
		}
		pCtx.Detach()
		result, err := chat.Rank(context.Background(), &data.User{}, feedback, candidates)
		if err != nil {
			if apiError, ok := err.(*openai.APIError); ok && apiError.HTTPStatusCode == 421 {
				return
			}
			log.Fatalf("failed to rank items for user %d: %v", userId, err)
		}
		pCtx.Attach()
		var posPredictions, negPredictions []float32
		for i, name := range result {
			itemId := test.GetIndex().EncodeItem(name) - int32(test.CountUsers())
			if userPositive[userId].Contains(itemId) {
				posPredictions = append(posPredictions, float32(len(result)-i))
			} else if userNegative[userId].Contains(itemId) {
				negPredictions = append(negPredictions, float32(len(result)-i))
			} else {
				log.Fatalf("item %s not found in test set for user %d", name, userId)
			}
		}
		if len(negPredictions) == 0 || len(posPredictions) == 0 {
			return
		}
		sumAUC.Add(ctr.AUC(posPredictions, negPredictions) * float32(len(posPredictions)))
		validUsers.Add(float32(len(posPredictions)))
		fmt.Printf("User %d AUC: %f pos: %d/%d, neg: %d/%d\n", userId, ctr.AUC(posPredictions, negPredictions),
			len(posPredictions), userPositive[userId].Cardinality(),
			len(negPredictions), userNegative[userId].Cardinality())
	})
	if validUsers.Load() == 0 {
		return 0
	}

	score := sumAUC.Load() / validUsers.Load()
	fmt.Println("LLM GAUC:", score)
	return score
}

func EvaluateAFM(train, test dataset.CTRSplit) float32 {
	ml := ctr.NewAFM(nil)
	ml.Fit(context.Background(), train, test,
		ctr.NewFitConfig().
			SetVerbose(10).
			SetJobs(runtime.NumCPU()).
			SetPatience(10))

	userPositiveCount := make(map[int32]int, train.CountUsers())
	userNegativeCount := make(map[int32]int, train.CountUsers())
	for i := 0; i < train.Count(); i++ {
		indices, _, _, target := train.Get(i)
		userId := indices[0]
		if target > 0 {
			userPositiveCount[userId]++
		} else {
			userNegativeCount[userId]++
		}
	}

	var posFeatures, negFeatures []lo.Tuple2[[]int32, []float32]
	var posEmbeddings, negEmbeddings [][][]float32
	var posUsers, negUsers []int32
	for i := 0; i < test.Count(); i++ {
		indices, values, embeddings, target := test.Get(i)
		userId := indices[0]
		if target > 0 {
			posFeatures = append(posFeatures, lo.Tuple2[[]int32, []float32]{A: indices, B: values})
			posEmbeddings = append(posEmbeddings, embeddings)
			posUsers = append(posUsers, userId)
		} else {
			negFeatures = append(negFeatures, lo.Tuple2[[]int32, []float32]{A: indices, B: values})
			negEmbeddings = append(negEmbeddings, embeddings)
			negUsers = append(negUsers, userId)
		}
	}
	posPrediction := ml.BatchInternalPredict(posFeatures, posEmbeddings, runtime.NumCPU())
	negPrediction := ml.BatchInternalPredict(negFeatures, negEmbeddings, runtime.NumCPU())

	userPosPrediction := make(map[int32][]float32)
	for i, p := range posPrediction {
		userPosPrediction[posUsers[i]] = append(userPosPrediction[posUsers[i]], p)
	}
	userNegPrediction := make(map[int32][]float32)
	for i, p := range negPrediction {
		userNegPrediction[negUsers[i]] = append(userNegPrediction[negUsers[i]], p)
	}

	var sumAUC float32
	var validUsers float32
	for user, pos := range userPosPrediction {
		if userPositiveCount[user] == 0 || userNegativeCount[user] == 0 {
			continue
		}
		if neg, ok := userNegPrediction[user]; ok {
			sumAUC += ctr.AUC(pos, neg) * float32(len(pos))
			validUsers += float32(len(pos))
		}
	}
	if validUsers == 0 {
		return 0
	}
	score := sumAUC / validUsers

	fmt.Println("AFM GAUC:", score)
	return score
}

func PrintHorizontalLine(char string) {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
	}
	line := strings.Repeat(char, width)
	fmt.Println(line)
}

func init() {
	rootCmd.PersistentFlags().StringP("config", "c", "", "Path to configuration file")
	rootCmd.AddCommand(llmCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
