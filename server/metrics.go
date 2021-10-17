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

package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	GetRecommendSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "server",
		Name:      "get_recommend_seconds",
	})
	LoadCTRRecommendCacheSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "server",
		Name:      "load_ctr_recommend_cache_seconds",
	})
	LoadCollaborativeRecommendCacheSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "server",
		Name:      "load_collaborative_recommend_cache_seconds",
	})
	ItemBasedRecommendSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "server",
		Name:      "item_based_recommend_seconds",
	})
	UserBasedRecommendSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "server",
		Name:      "user_based_recommend_seconds",
	})
	LoadLatestRecommendCacheSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "server",
		Name:      "load_latest_recommend_cache_seconds",
	})
	LoadPopularRecommendCacheSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "server",
		Name:      "load_popular_recommend_cache_seconds",
	})
)
