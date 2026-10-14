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
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestDecisionCommand(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"decision"})
	require.NoError(t, err)
	require.Equal(t, "decision", cmd.Name())
	require.NotNil(t, cmd.Run)
	require.NotNil(t, cmd.PersistentFlags().Lookup("user-auc"))
}

func TestEvaluateDecision(t *testing.T) {
	t.Chdir(t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer decision-key", r.Header.Get("Authorization"))
		var request struct {
			Model     string `json:"model"`
			State     string `json:"state"`
			Questions map[string]struct {
				Type         string `json:"type"`
				Instructions string `json:"instructions"`
			} `json:"questions"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "decision-model", request.Model)
		require.Equal(t, "history", request.State)
		require.Len(t, request.Questions, 2)
		require.Equal(t, "noul", request.Questions["positive"].Type)
		require.Equal(t, "positive candidate", request.Questions["positive"].Instructions)
		require.Equal(t, "negative candidate", request.Questions["negative"].Instructions)
		_, err := w.Write([]byte(`{"answers":{"positive":{"type":"noul","noul":0.9},"negative":{"type":"noul","noul":0.1}}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	cfg := config.GetDefaultConfig()
	cfg.Recommend.ContextSize = 1
	cfg.Recommend.Ranker.DecisionAPI = config.DecisionAPIConfig{
		URL: server.URL, Model: "decision-model", AuthToken: "decision-key",
		StateTemplate: "{{ feedback[0].ItemId }}", InstructionsTemplate: "{{ item.Comment }}",
	}
	builder := dataset.NewUnifiedMapIndexBuilder()
	builder.AddUser("user")
	items := []data.Item{
		{ItemId: "history"},
		{ItemId: "positive", Comment: "positive candidate"},
		{ItemId: "negative", Comment: "negative candidate"},
	}
	for _, item := range items {
		builder.AddItem(item.ItemId)
	}
	index := builder.Build()
	train := &ctr.Dataset{
		Index: index, Users: []int32{0, 0}, Items: []int32{0, 2}, Target: []float32{1, -1},
		UserLabels: make([][]lo.Tuple2[int32, float32], 1), ItemLabels: make([][]lo.Tuple2[int32, float32], 3),
	}
	test := &ctr.Dataset{
		Index: index, Users: []int32{0, 0}, Items: []int32{1, 2}, Target: []float32{1, -1},
		UserLabels: train.UserLabels, ItemLabels: train.ItemLabels,
	}
	var scores sync.Map
	EvaluateDecision(cfg, train, test, items, true, &scores)
	score, ok := scores.Load("decision-model")
	require.True(t, ok)
	require.Equal(t, float64(1), score)
	file, err := os.Open("decision-model.csv")
	require.NoError(t, err)
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	require.NoError(t, err)
	require.Equal(t, [][]string{{"Feedback", "Candidates", "AUC"}, {"1", "2", "1.0000"}}, rows)
}
