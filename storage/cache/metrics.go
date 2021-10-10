// Copyright 2021 gorse Project Authors
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

package cache

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	SetScoresSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "cache",
		Name:      "cache_set_scores_seconds",
	})
	GetScoresSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "cache",
		Name:      "cache_get_scores_seconds",
	})
	ClearScoresSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "cache",
		Name:      "cache_clear_scores_seconds",
	})
	AppendScoresSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "cache",
		Name:      "cache_append_scores_seconds",
	})

	SetScoresTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "cache",
		Name:      "cache_set_scores_times",
	})
	GetScoresTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "cache",
		Name:      "cache_get_scores_times",
	})
	ClearScoresTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "cache",
		Name:      "cache_clear_scores_times",
	})
	AppendScoresTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "cache",
		Name:      "cache_append_scores_times",
	})
)
