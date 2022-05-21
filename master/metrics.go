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

package master

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	LabelFeedbackType = "feedback_type"
	LabelStep         = "step"
	LabelData         = "data"
)

var (
	LoadDatasetStepSecondsVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "load_dataset_step_seconds",
	}, []string{LabelStep})
	LoadDatasetTotalSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "load_dataset_total_seconds",
	})
	FindUserNeighborsSecondsVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "find_user_neighbors_seconds",
	}, []string{LabelStep})
	FindUserNeighborsTotalSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "find_user_neighbors_total_seconds",
	})
	FindItemNeighborsSecondsVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "find_item_neighbors_seconds",
	}, []string{"step"})
	FindItemNeighborsTotalSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "find_item_neighbors_total_seconds",
	})
	UpdateUserNeighborsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "update_user_neighbors_total",
	})
	UpdateItemNeighborsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "update_item_neighbors_total",
	})
	CacheScannedTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "cache_scanned_total",
	})
	CacheReclaimedTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "cache_reclaimed_total",
	})
	CacheScannedSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "cache_scanned_seconds",
	})

	CollaborativeFilteringFitSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "collaborative_filtering_fit_seconds",
	})
	CollaborativeFilteringSearchSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "collaborative_filtering_search_seconds",
	})
	CollaborativeFilteringNDCG10 = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "collaborative_filtering_ndcg_10",
	})
	CollaborativeFilteringPrecision10 = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "collaborative_filtering_precision_10",
	})
	CollaborativeFilteringRecall10 = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "collaborative_filtering_recall_10",
	})
	CollaborativeFilteringSearchPrecision10 = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "collaborative_filtering_search_precision_10",
	})
	RankingFitSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "ranking_fit_seconds",
	})
	RankingSearchSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "ranking_search_seconds",
	})
	RankingPrecision = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "ranking_model_precision",
	})
	RankingRecall = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "ranking_model_recall",
	})
	RankingAUC = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "ranking_model_auc",
	})
	RankingSearchPrecision = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "ranking_search_precision",
	})
	UserNeighborIndexRecall = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "user_neighbor_index_recall",
	})
	ItemNeighborIndexRecall = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "item_neighbor_index_recall",
	})

	UsersTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "users_total",
	})
	ActiveUsersTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "active_users_total",
	})
	InactiveUsersTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "inactive_users_total",
	})
	ItemsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "items_total",
	})
	ActiveItemsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "active_items_total",
	})
	InactiveItemsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "inactive_items_total",
	})
	UserLabelsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "user_labels_total",
	})
	ItemLabelsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "item_labels_total",
	})
	FeedbacksTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "feedbacks_total",
	})
	ImplicitFeedbacksTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "implicit_feedbacks_total",
	})
	PositiveFeedbacksTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "positive_feedbacks_total",
	})
	NegativeFeedbackTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "negative_feedbacks_total",
	})
	PositiveFeedbackRateVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "positive_feedback_rate",
	}, []string{LabelFeedbackType})
	MemoryInuseBytesVec = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gorse",
		Subsystem: "master",
		Name:      "memory_inuse_bytes",
	}, []string{LabelData})
)
