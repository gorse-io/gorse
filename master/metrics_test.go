// Copyright 2022 gorse Project Authors
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

package master

import (
	"testing"
	"time"

	"github.com/gorse-io/gorse/storage/cache"
	"github.com/stretchr/testify/assert"
)

func TestOnlineEvaluator(t *testing.T) {
	evaluator1 := NewOnlineEvaluator()
	result := evaluator1.Evaluate()
	assert.Empty(t, result)

	evaluator2 := NewOnlineEvaluator()
	evaluator2.TruncatedDateToday = time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC)
	evaluator2.EvaluateDays = 2
	evaluator2.Read(1, 1, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Read(1, 2, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Read(1, 3, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Read(1, 4, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Read(1, 5, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Positive("star", 1, 1, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Positive("like", 1, 1, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Read(2, 1, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Read(2, 2, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Read(2, 3, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Read(2, 4, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Positive("like", 2, 1, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Positive("star", 2, 1, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator2.Positive("star", 2, 3, time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC))
	evaluator2.Positive("fork", 3, 3, time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC))
	result = evaluator2.Evaluate()
	assert.ElementsMatch(t, []cache.TimeSeriesPoint{
		{"PositiveFeedbackRate/star", time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC), 0},
		{"PositiveFeedbackRate/star", time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC), 0.35},
		{"PositiveFeedbackRate/like", time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC), 0},
		{"PositiveFeedbackRate/like", time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC), 0.225},
		{"PositiveFeedbackRate/fork", time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC), 0},
		{"PositiveFeedbackRate/fork", time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC), 0},
	}, result)
}
