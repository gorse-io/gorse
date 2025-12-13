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

	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/stretchr/testify/assert"
)

func TestOnlineEvaluator(t *testing.T) {
	positiveTypes := []expression.FeedbackTypeExpression{expression.MustParseFeedbackTypeExpression("read>=100")}
	readTypes := []expression.FeedbackTypeExpression{expression.MustParseFeedbackTypeExpression("read")}

	evaluator := NewOnlineEvaluator(positiveTypes, readTypes)
	result := evaluator.Evaluate()
	assert.Empty(t, result)

	evaluator = NewOnlineEvaluator(positiveTypes, readTypes)
	evaluator.WindowEnd = time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC)
	evaluator.WindowSize = 2
	evaluator.Add("read", 0, 1, 1, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator.Add("read", 0, 1, 2, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator.Add("read", 0, 1, 3, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator.Add("read", 100, 1, 4, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator.Add("read", 100, 2, 1, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator.Add("read", 0, 2, 2, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator.Add("read", 0, 2, 3, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator.Add("read", 0, 2, 4, time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC))
	evaluator.Add("read", 0, 2, 3, time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC))
	evaluator.Add("read", 100, 3, 3, time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC))
	result = evaluator.Evaluate()
	assert.ElementsMatch(t, []cache.TimeSeriesPoint{
		{Name: "ctr_read", Timestamp: time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC), Value: 0.5},
		{Name: "ctr_read", Timestamp: time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC), Value: 0.25},
		{Name: "ctr", Timestamp: time.Date(2005, 6, 16, 0, 0, 0, 0, time.UTC), Value: 0.5},
		{Name: "ctr", Timestamp: time.Date(2005, 6, 15, 0, 0, 0, 0, time.UTC), Value: 0.25},
	}, result)
}
