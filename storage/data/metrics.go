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

package data

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	BatchInsertItemsSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_batch_insert_items_seconds",
	})
	DeleteItemSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_delete_item_seconds",
	})
	GetItemSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_item_seconds",
	})
	GetItemFeedbackSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_item_feedback_seconds",
	})
	BatchInsertUsersSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_batch_insert_users_seconds",
	})
	DeleteUserSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_delete_user_seconds",
	})
	GetUserSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_user_seconds",
	})
	GetUserFeedbackSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_user_feedback_seconds",
	})
	GetUserItemFeedbackSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_user_item_feedback_seconds",
	})
	DeleteUserItemFeedbackSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_delete_user_item_feedback_seconds",
	})
	BatchInsertFeedbackSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_batch_insert_feedback_seconds",
	})
	InsertMeasurementSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_insert_measurement_seconds",
	})
	GetClickThroughRateSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_click_through_rate_seconds",
	})

	BatchInsertItemsTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_batch_insert_items_times",
	})
	DeleteItemTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_delete_item_times",
	})
	GetItemTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_item_times",
	})
	GetItemFeedbackTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_item_feedback_times",
	})
	BatchInsertUsersTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_batch_insert_users_times",
	})
	DeleteUserTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_delete_user_times",
	})
	GetUserTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_user_times",
	})
	GetUserFeedbackTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_user_feedback_times",
	})
	GetUserItemFeedbackTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_user_item_feedback_times",
	})
	DeleteUserItemFeedbackTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_delete_user_item_feedback_times",
	})
	BatchInsertFeedbackTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_batch_insert_feedback_times",
	})
	InsertMeasurementTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_insert_measurement_times",
	})
	GetClickThroughRateTimes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "gorse",
		Subsystem: "database",
		Name:      "database_get_click_through_rate_times",
	})
)
