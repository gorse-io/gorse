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
	"sort"
	"strconv"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/olekukonko/tablewriter"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	agentBenchmarkTopK      = 100
	agentBenchmarkTestRatio = 0.8
)

type agentBenchmarkScore struct {
	name    string
	users   int
	ndcg    float32
	overlap float32
}

var benchAgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Benchmark agent recommenders",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if len(cfg.Recommend.Agent) == 0 {
			return fmt.Errorf("no agent recommender configured")
		}

		dataClient, err := data.Open(cfg.Database.DataStore, cfg.Database.DataTablePrefix,
			storage.WithIsolationLevel(cfg.Database.MySQL.IsolationLevel))
		if err != nil {
			return fmt.Errorf("open data client: %w", err)
		}
		defer func() {
			if err := dataClient.Close(); err != nil {
				log.Logger().Error("failed to close data client", zap.Error(err))
			}
		}()

		feedbackByUser, err := loadAgentFeedback(cmd.Context(), dataClient, cfg)
		if err != nil {
			return err
		}
		jobs, _ := cmd.Flags().GetInt("jobs")
		scores := make([]agentBenchmarkScore, 0, len(cfg.Recommend.Agent))
		for _, agentConfig := range cfg.Recommend.Agent {
			score, err := benchmarkAgent(cmd.Context(), cfg, agentConfig, dataClient, feedbackByUser, jobs)
			if err != nil {
				return err
			}
			scores = append(scores, score)
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"Agent", "#Users", "NDCG@100", "Overlap@100"})
		rows := lo.Map(scores, func(score agentBenchmarkScore, _ int) []string {
			return []string{
				score.name,
				strconv.Itoa(score.users),
				fmt.Sprintf("%.4f", score.ndcg),
				fmt.Sprintf("%.4f", score.overlap),
			}
		})
		if err := table.Bulk(rows); err != nil {
			return fmt.Errorf("write result table: %w", err)
		}
		if err := table.Render(); err != nil {
			return fmt.Errorf("render result table: %w", err)
		}
		return nil
	},
}

func loadAgentFeedback(ctx context.Context, dataClient data.Database, cfg *config.Config) (map[string][]data.Feedback, error) {
	feedbackByUser := make(map[string][]data.Feedback)
	feedbackChan, errChan := dataClient.GetFeedbackStream(ctx, 10_000,
		data.WithFeedbackTypes(cfg.Recommend.DataSource.PositiveFeedbackTypes...))
	for batch := range feedbackChan {
		for _, feedback := range batch {
			feedbackByUser[feedback.UserId] = append(feedbackByUser[feedback.UserId], feedback)
		}
	}
	if err := <-errChan; err != nil {
		return nil, fmt.Errorf("load positive feedback: %w", err)
	}
	return feedbackByUser, nil
}

func benchmarkAgent(ctx context.Context, cfg *config.Config, agentConfig config.AgentConfig, dataClient data.Database,
	feedbackByUser map[string][]data.Feedback, jobs int) (agentBenchmarkScore, error) {
	userIds := make([]string, 0, len(feedbackByUser))
	for userId, feedback := range feedbackByUser {
		train, test := splitAgentFeedback(feedback, agentBenchmarkTestRatio)
		if len(train) > 0 && len(test) > 0 {
			userIds = append(userIds, userId)
		}
	}
	sort.Strings(userIds)

	var mu sync.Mutex
	var ndcg, overlap float32
	var count int
	var firstErr error
	err := parallel.ForEach(ctx, userIds, jobs, func(_ int, userId string) {
		train, test := splitAgentFeedback(feedbackByUser[userId], agentBenchmarkTestRatio)
		excludeSet := mapset.NewSet[string]()
		for _, feedback := range train {
			excludeSet.Add(feedback.ItemId)
		}
		agent, err := logics.NewAgent(agentConfig, cfg.OpenAI, dataClient, userId, train, nil, excludeSet, agentBenchmarkTopK)
		if err != nil {
			mu.Lock()
			if firstErr == nil {
				firstErr = fmt.Errorf("create agent %q for user %q: %w", agentConfig.Name, userId, err)
			}
			mu.Unlock()
			return
		}
		recommendations, err := agent.Recommend(ctx)
		if err != nil {
			mu.Lock()
			if firstErr == nil {
				firstErr = fmt.Errorf("recommend with agent %q for user %q: %w", agentConfig.Name, userId, err)
			}
			mu.Unlock()
			return
		}
		rankList := lo.Map(recommendations, func(score cache.Score, _ int) string {
			return score.Id
		})
		userNDCG, userOverlap := evaluateAgentRecommendations(rankList, test, agentBenchmarkTopK)
		mu.Lock()
		ndcg += userNDCG
		overlap += userOverlap
		count++
		mu.Unlock()
	})
	if err != nil {
		return agentBenchmarkScore{}, fmt.Errorf("benchmark agent %q: %w", agentConfig.Name, err)
	}
	if firstErr != nil {
		return agentBenchmarkScore{}, firstErr
	}
	if count > 0 {
		ndcg /= float32(count)
		overlap /= float32(count)
	}
	return agentBenchmarkScore{name: agentConfig.Name, users: count, ndcg: ndcg, overlap: overlap}, nil
}

func splitAgentFeedback(feedback []data.Feedback, testRatio float64) ([]data.Feedback, []data.Feedback) {
	if len(feedback) < 2 {
		return append([]data.Feedback(nil), feedback...), nil
	}
	sorted := append([]data.Feedback(nil), feedback...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})
	numTest := int(float64(len(sorted)) * testRatio)
	if numTest == 0 {
		numTest = 1
	}
	if numTest >= len(sorted) {
		numTest = len(sorted) - 1
	}
	split := len(sorted) - numTest
	return sorted[:split], sorted[split:]
}

func evaluateAgentRecommendations(rankList []string, target []data.Feedback, topK int) (float32, float32) {
	targetSet := mapset.NewSet[string]()
	for _, feedback := range target {
		targetSet.Add(feedback.ItemId)
	}
	if targetSet.Cardinality() == 0 || topK <= 0 {
		return 0, 0
	}
	if len(rankList) > topK {
		rankList = rankList[:topK]
	}

	var dcg float64
	hits := 0
	seen := mapset.NewSet[string]()
	for i, itemId := range rankList {
		if seen.Contains(itemId) {
			continue
		}
		seen.Add(itemId)
		if targetSet.Contains(itemId) {
			dcg += 1 / math.Log2(float64(i)+2)
			hits++
		}
	}
	var idcg float64
	for i := 0; i < min(targetSet.Cardinality(), topK); i++ {
		idcg += 1 / math.Log2(float64(i)+2)
	}
	return float32(dcg / idcg), float32(hits) / float32(targetSet.Cardinality())
}

func init() {
	rootCmd.AddCommand(benchAgentCmd)
}
