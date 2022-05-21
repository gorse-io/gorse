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

package worker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	LabelStep = "step"
	LabelData = "data"
)

var (
	UpdateUserRecommendTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "worker",
		Name:      "update_user_recommend_total",
	})
	OfflineRecommendStepSecondsVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "worker",
		Name:      "offline_recommend_step_seconds",
	}, []string{LabelStep})
	OfflineRecommendTotalSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "worker",
		Name:      "offline_recommend_total_seconds",
	})
	CollaborativeFilteringIndexRecall = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "worker",
		Name:      "collaborative_filtering_index_recall",
	})
	MemoryInuseBytesVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "worker",
		Name:      "memory_inuse_bytes",
	}, []string{LabelData})
)
