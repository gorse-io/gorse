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
	"testing"
	"time"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitAgentFeedback(t *testing.T) {
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{ItemId: "item3"}, Timestamp: time.Unix(3, 0)},
		{FeedbackKey: data.FeedbackKey{ItemId: "item1"}, Timestamp: time.Unix(1, 0)},
		{FeedbackKey: data.FeedbackKey{ItemId: "item5"}, Timestamp: time.Unix(5, 0)},
		{FeedbackKey: data.FeedbackKey{ItemId: "item2"}, Timestamp: time.Unix(2, 0)},
		{FeedbackKey: data.FeedbackKey{ItemId: "item4"}, Timestamp: time.Unix(4, 0)},
	}

	train, test := splitAgentFeedback(feedback, 0.8)

	require.Len(t, train, 1)
	assert.Equal(t, "item1", train[0].ItemId)
	require.Len(t, test, 4)
	assert.Equal(t, []string{"item2", "item3", "item4", "item5"}, []string{
		test[0].ItemId, test[1].ItemId, test[2].ItemId, test[3].ItemId,
	})
}

func TestEvaluateAgentRecommendations(t *testing.T) {
	ndcg, overlap := evaluateAgentRecommendations(
		[]string{"other", "item2", "item1"},
		[]data.Feedback{
			{FeedbackKey: data.FeedbackKey{ItemId: "item1"}},
			{FeedbackKey: data.FeedbackKey{ItemId: "item2"}},
		},
		100,
	)

	assert.InDelta(t, 0.6934, ndcg, 1e-4)
	assert.Equal(t, float32(1), overlap)
}

func TestBenchmarkAgentReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := benchmarkAgent(ctx, &config.Config{}, config.AgentConfig{}, nil, map[string][]data.Feedback{
		"user": {
			{FeedbackKey: data.FeedbackKey{ItemId: "item1"}, Timestamp: time.Unix(1, 0)},
			{FeedbackKey: data.FeedbackKey{ItemId: "item2"}, Timestamp: time.Unix(2, 0)},
		},
	}, 1)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestAgentCommandRegistered(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"agent"})
	require.NoError(t, err)
	assert.Same(t, benchAgentCmd, command)
}
