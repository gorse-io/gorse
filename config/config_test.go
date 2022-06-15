// Copyright 2020 gorse Project Authors
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

package config

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

func TestUnmarshal(t *testing.T) {
	data, err := ioutil.ReadFile("config.toml.template")
	assert.NoError(t, err)
	text := string(data)
	text = strings.Replace(text, "dashboard_user_name = \"\"", "dashboard_user_name = \"admin\"", -1)
	text = strings.Replace(text, "dashboard_password = \"\"", "dashboard_password = \"password\"", -1)
	text = strings.Replace(text, "api_key = \"\"", "api_key = \"19260817\"", -1)
	viper.SetConfigType("toml")
	err = viper.ReadConfig(strings.NewReader(text))
	assert.NoError(t, err)
	var config Config
	err = viper.Unmarshal(&config)
	assert.NoError(t, err)

	// [database]
	assert.Equal(t, "redis://localhost:6379/0", config.Database.CacheStore)
	assert.Equal(t, "mysql://gorse:gorse_pass@tcp(localhost:3306)/gorse", config.Database.DataStore)
	// [master]
	assert.Equal(t, 8086, config.Master.Port)
	assert.Equal(t, "0.0.0.0", config.Master.Host)
	assert.Equal(t, 8088, config.Master.HttpPort)
	assert.Equal(t, "0.0.0.0", config.Master.HttpHost)
	assert.Equal(t, 1, config.Master.NumJobs)
	assert.Equal(t, 10*time.Second, config.Master.MetaTimeout)
	assert.Equal(t, "admin", config.Master.DashboardUserName)
	assert.Equal(t, "password", config.Master.DashboardPassword)
	// [server]
	assert.Equal(t, 10, config.Server.DefaultN)
	assert.Equal(t, "19260817", config.Server.APIKey)
	assert.Equal(t, 5*time.Second, config.Server.ClockError)
	assert.True(t, config.Server.AutoInsertUser)
	assert.False(t, config.Server.AutoInsertItem)
	assert.Equal(t, 10*time.Second, config.Server.CacheExpire)
	// [recommend]
	assert.Equal(t, 100, config.Recommend.CacheSize)
	assert.Equal(t, 72*time.Hour, config.Recommend.CacheExpire)
	// [recommend.data_source]
	assert.Equal(t, []string{"star", "like"}, config.Recommend.DataSource.PositiveFeedbackTypes)
	assert.Equal(t, []string{"read"}, config.Recommend.DataSource.ReadFeedbackTypes)
	assert.Equal(t, uint(0), config.Recommend.DataSource.PositiveFeedbackTTL)
	assert.Equal(t, uint(0), config.Recommend.DataSource.ItemTTL)
	// [recommend.popular]
	assert.Equal(t, 30*24*time.Hour, config.Recommend.Popular.PopularWindow)
	// [recommend.user_neighbors]
	assert.Equal(t, "similar", config.Recommend.UserNeighbors.NeighborType)
	assert.True(t, config.Recommend.UserNeighbors.EnableIndex)
	assert.Equal(t, float32(0.8), config.Recommend.UserNeighbors.IndexRecall)
	assert.Equal(t, 3, config.Recommend.UserNeighbors.IndexFitEpoch)
	// [recommend.item_neighbors]
	assert.Equal(t, "similar", config.Recommend.ItemNeighbors.NeighborType)
	assert.True(t, config.Recommend.ItemNeighbors.EnableIndex)
	assert.Equal(t, float32(0.8), config.Recommend.ItemNeighbors.IndexRecall)
	assert.Equal(t, 3, config.Recommend.ItemNeighbors.IndexFitEpoch)
	// [recommend.collaborative]
	assert.True(t, config.Recommend.Collaborative.EnableIndex)
	assert.Equal(t, float32(0.9), config.Recommend.Collaborative.IndexRecall)
	assert.Equal(t, 3, config.Recommend.Collaborative.IndexFitEpoch)
	assert.Equal(t, 60*time.Minute, config.Recommend.Collaborative.ModelFitPeriod)
	assert.Equal(t, 360*time.Minute, config.Recommend.Collaborative.ModelSearchPeriod)
	assert.Equal(t, 100, config.Recommend.Collaborative.ModelSearchEpoch)
	assert.Equal(t, 10, config.Recommend.Collaborative.ModelSearchTrials)
	// [recommend.replacement]
	assert.False(t, config.Recommend.Replacement.EnableReplacement)
	assert.Equal(t, 0.8, config.Recommend.Replacement.PositiveReplacementDecay)
	assert.Equal(t, 0.6, config.Recommend.Replacement.ReadReplacementDecay)
	// [recommend.offline]
	assert.Equal(t, time.Minute, config.Recommend.Offline.CheckRecommendPeriod)
	assert.Equal(t, 24*time.Hour, config.Recommend.Offline.RefreshRecommendPeriod)
	assert.True(t, config.Recommend.Offline.EnableColRecommend)
	assert.False(t, config.Recommend.Offline.EnableItemBasedRecommend)
	assert.True(t, config.Recommend.Offline.EnableUserBasedRecommend)
	assert.False(t, config.Recommend.Offline.EnablePopularRecommend)
	assert.True(t, config.Recommend.Offline.EnableLatestRecommend)
	assert.True(t, config.Recommend.Offline.EnableClickThroughPrediction)
	assert.Equal(t, map[string]float64{"popular": 0.1, "latest": 0.2}, config.Recommend.Offline.ExploreRecommend)
	value, exist := config.Recommend.Offline.GetExploreRecommend("popular")
	assert.Equal(t, true, exist)
	assert.Equal(t, 0.1, value)
	value, exist = config.Recommend.Offline.GetExploreRecommend("latest")
	assert.Equal(t, true, exist)
	assert.Equal(t, 0.2, value)
	_, exist = config.Recommend.Offline.GetExploreRecommend("unknown")
	assert.Equal(t, false, exist)
	// [recommend.online]
	assert.Equal(t, []string{"item_based", "latest"}, config.Recommend.Online.FallbackRecommend)
	assert.Equal(t, 10, config.Recommend.Online.NumFeedbackFallbackItemBased)
}

func TestSetDefault(t *testing.T) {
	setDefault()
	err := viper.ReadConfig(strings.NewReader(""))
	assert.NoError(t, err)
	var config Config
	err = viper.Unmarshal(&config)
	assert.NoError(t, err)
	assert.Equal(t, GetDefaultConfig(), &config)
}

type environmentVariable struct {
	key   string
	value string
}

func TestBindEnv(t *testing.T) {
	variables := []environmentVariable{
		{"GORSE_CACHE_STORE", "redis://<cache_store>"},
		{"GORSE_DATA_STORE", "mysql://<data_store>"},
		{"GORSE_MASTER_PORT", "123"},
		{"GORSE_MASTER_HOST", "<master_host>"},
		{"GORSE_MASTER_HTTP_PORT", "456"},
		{"GORSE_MASTER_HTTP_HOST", "<master_http_host>"},
		{"GORSE_MASTER_JOBS", "789"},
		{"GORSE_DASHBOARD_USER_NAME", "user_name"},
		{"GORSE_DASHBOARD_PASSWORD", "password"},
		{"GORSE_SERVER_API_KEY", "<server_api_key>"},
	}
	for _, variable := range variables {
		err := os.Setenv(variable.key, variable.value)
		assert.NoError(t, err)
	}

	config, err := LoadConfig("config.toml.template", false)
	assert.NoError(t, err)
	assert.Equal(t, "redis://<cache_store>", config.Database.CacheStore)
	assert.Equal(t, "mysql://<data_store>", config.Database.DataStore)
	assert.Equal(t, 123, config.Master.Port)
	assert.Equal(t, "<master_host>", config.Master.Host)
	assert.Equal(t, 456, config.Master.HttpPort)
	assert.Equal(t, "<master_http_host>", config.Master.HttpHost)
	assert.Equal(t, 789, config.Master.NumJobs)
	assert.Equal(t, "user_name", config.Master.DashboardUserName)
	assert.Equal(t, "password", config.Master.DashboardPassword)
	assert.Equal(t, "<server_api_key>", config.Server.APIKey)

	// check default values
	assert.Equal(t, 100, config.Recommend.CacheSize)
}

func TestConfig_UserNeighborDigest(t *testing.T) {
	cfg1, cfg2 := GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.UserNeighbors.NeighborType = "auto"
	cfg2.Recommend.UserNeighbors.NeighborType = "manual"
	assert.NotEqual(t, cfg1.UserNeighborDigest(), cfg2.UserNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.UserNeighbors.EnableIndex = true
	cfg2.Recommend.UserNeighbors.EnableIndex = false
	assert.NotEqual(t, cfg1.UserNeighborDigest(), cfg2.UserNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.UserNeighbors.NeighborType = "auto"
	cfg2.Recommend.UserNeighbors.NeighborType = "auto"
	cfg1.Recommend.DataSource.PositiveFeedbackTypes = []string{"positive"}
	cfg2.Recommend.DataSource.PositiveFeedbackTypes = []string{"negative"}
	assert.NotEqual(t, cfg1.UserNeighborDigest(), cfg2.UserNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.UserNeighbors.NeighborType = "related"
	cfg2.Recommend.UserNeighbors.NeighborType = "related"
	cfg1.Recommend.DataSource.PositiveFeedbackTypes = []string{"positive"}
	cfg2.Recommend.DataSource.PositiveFeedbackTypes = []string{"negative"}
	assert.NotEqual(t, cfg1.UserNeighborDigest(), cfg2.UserNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.UserNeighbors.NeighborType = "similar"
	cfg2.Recommend.UserNeighbors.NeighborType = "similar"
	cfg1.Recommend.DataSource.PositiveFeedbackTypes = []string{"positive"}
	cfg2.Recommend.DataSource.PositiveFeedbackTypes = []string{"negative"}
	assert.Equal(t, cfg1.UserNeighborDigest(), cfg2.UserNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.UserNeighbors.EnableIndex = true
	cfg2.Recommend.UserNeighbors.EnableIndex = true
	cfg1.Recommend.UserNeighbors.IndexRecall = 0.5
	cfg2.Recommend.UserNeighbors.IndexRecall = 0.6
	assert.NotEqual(t, cfg1.UserNeighborDigest(), cfg2.UserNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.UserNeighbors.EnableIndex = true
	cfg2.Recommend.UserNeighbors.EnableIndex = true
	cfg1.Recommend.UserNeighbors.IndexFitEpoch = 10
	cfg2.Recommend.UserNeighbors.IndexFitEpoch = 11
	assert.NotEqual(t, cfg1.UserNeighborDigest(), cfg2.UserNeighborDigest())
}

func TestConfig_ItemNeighborDigest(t *testing.T) {
	cfg1, cfg2 := GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.ItemNeighbors.NeighborType = "auto"
	cfg2.Recommend.ItemNeighbors.NeighborType = "related"
	assert.NotEqual(t, cfg1.ItemNeighborDigest(), cfg2.ItemNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.ItemNeighbors.EnableIndex = true
	cfg2.Recommend.ItemNeighbors.EnableIndex = false
	assert.NotEqual(t, cfg1.ItemNeighborDigest(), cfg2.ItemNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.ItemNeighbors.NeighborType = "auto"
	cfg2.Recommend.ItemNeighbors.NeighborType = "auto"
	cfg1.Recommend.DataSource.PositiveFeedbackTypes = []string{"positive"}
	cfg2.Recommend.DataSource.PositiveFeedbackTypes = []string{"negative"}
	assert.NotEqual(t, cfg1.ItemNeighborDigest(), cfg2.ItemNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.ItemNeighbors.NeighborType = "related"
	cfg2.Recommend.ItemNeighbors.NeighborType = "related"
	cfg1.Recommend.DataSource.PositiveFeedbackTypes = []string{"positive"}
	cfg2.Recommend.DataSource.PositiveFeedbackTypes = []string{"negative"}
	assert.NotEqual(t, cfg1.ItemNeighborDigest(), cfg2.ItemNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.ItemNeighbors.NeighborType = "similar"
	cfg2.Recommend.ItemNeighbors.NeighborType = "similar"
	cfg1.Recommend.DataSource.PositiveFeedbackTypes = []string{"positive"}
	cfg2.Recommend.DataSource.PositiveFeedbackTypes = []string{"negative"}
	assert.Equal(t, cfg1.ItemNeighborDigest(), cfg2.ItemNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.ItemNeighbors.EnableIndex = true
	cfg2.Recommend.ItemNeighbors.EnableIndex = true
	cfg1.Recommend.ItemNeighbors.IndexRecall = 0.5
	cfg2.Recommend.ItemNeighbors.IndexRecall = 0.6
	assert.NotEqual(t, cfg1.ItemNeighborDigest(), cfg2.ItemNeighborDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.ItemNeighbors.EnableIndex = true
	cfg2.Recommend.ItemNeighbors.EnableIndex = true
	cfg1.Recommend.ItemNeighbors.IndexFitEpoch = 10
	cfg2.Recommend.ItemNeighbors.IndexFitEpoch = 11
	assert.NotEqual(t, cfg1.ItemNeighborDigest(), cfg2.ItemNeighborDigest())
}

func TestConfig_OfflineRecommendDigest(t *testing.T) {
	// test explore recommendation
	cfg1, cfg2 := GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.ExploreRecommend = map[string]float64{"a": 0.5, "b": 0.6}
	cfg2.Recommend.Offline.ExploreRecommend = map[string]float64{"a": 0.6, "b": 0.5}
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	// test latest recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableLatestRecommend = true
	cfg2.Recommend.Offline.EnableLatestRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	// test popular recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnablePopularRecommend = true
	cfg2.Recommend.Offline.EnablePopularRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnablePopularRecommend = true
	cfg2.Recommend.Offline.EnablePopularRecommend = true
	cfg1.Recommend.Popular.PopularWindow = 10
	cfg2.Recommend.Popular.PopularWindow = 11
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnablePopularRecommend = false
	cfg2.Recommend.Offline.EnablePopularRecommend = false
	cfg1.Recommend.Popular.PopularWindow = 10
	cfg2.Recommend.Popular.PopularWindow = 11
	assert.Equal(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	// test user-based recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableUserBasedRecommend = true
	cfg2.Recommend.Offline.EnableUserBasedRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableUserBasedRecommend = true
	cfg2.Recommend.Offline.EnableUserBasedRecommend = true
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(WithUserNeighborDigest("1")), cfg2.OfflineRecommendDigest(WithUserNeighborDigest("2")))

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableUserBasedRecommend = false
	cfg2.Recommend.Offline.EnableUserBasedRecommend = false
	assert.Equal(t, cfg1.OfflineRecommendDigest(WithUserNeighborDigest("1")), cfg2.OfflineRecommendDigest(WithUserNeighborDigest("2")))

	// test item-based recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableItemBasedRecommend = true
	cfg2.Recommend.Offline.EnableItemBasedRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableItemBasedRecommend = true
	cfg2.Recommend.Offline.EnableItemBasedRecommend = true
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(WithItemNeighborDigest("1")), cfg2.OfflineRecommendDigest(WithItemNeighborDigest("2")))

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableItemBasedRecommend = false
	cfg2.Recommend.Offline.EnableItemBasedRecommend = false
	assert.Equal(t, cfg1.OfflineRecommendDigest(WithItemNeighborDigest("1")), cfg2.OfflineRecommendDigest(WithItemNeighborDigest("2")))

	// test collaborative-filtering recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableColRecommend = true
	cfg2.Recommend.Offline.EnableColRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableColRecommend = true
	cfg2.Recommend.Offline.EnableColRecommend = true
	cfg1.Recommend.Collaborative.EnableIndex = true
	cfg2.Recommend.Collaborative.EnableIndex = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableColRecommend = true
	cfg2.Recommend.Offline.EnableColRecommend = true
	cfg1.Recommend.Collaborative.EnableIndex = true
	cfg2.Recommend.Collaborative.EnableIndex = true
	cfg1.Recommend.Collaborative.IndexRecall = 0.4
	cfg2.Recommend.Collaborative.IndexRecall = 0.5
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableColRecommend = true
	cfg2.Recommend.Offline.EnableColRecommend = true
	cfg1.Recommend.Collaborative.EnableIndex = false
	cfg2.Recommend.Collaborative.EnableIndex = false
	cfg1.Recommend.Collaborative.IndexRecall = 0.4
	cfg2.Recommend.Collaborative.IndexRecall = 0.5
	assert.Equal(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableColRecommend = false
	cfg2.Recommend.Offline.EnableColRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(WithCollaborative(true)), cfg2.OfflineRecommendDigest())

	// test click-through rate prediction recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableClickThroughPrediction = true
	cfg2.Recommend.Offline.EnableClickThroughPrediction = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableClickThroughPrediction = false
	cfg2.Recommend.Offline.EnableClickThroughPrediction = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(WithRanking(true)), cfg2.OfflineRecommendDigest())

	// test replacement
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Replacement.EnableReplacement = true
	cfg2.Recommend.Replacement.EnableReplacement = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Replacement.EnableReplacement = true
	cfg2.Recommend.Replacement.EnableReplacement = true
	cfg1.Recommend.Replacement.PositiveReplacementDecay = 0.1
	cfg2.Recommend.Replacement.PositiveReplacementDecay = 0.2
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Replacement.EnableReplacement = false
	cfg2.Recommend.Replacement.EnableReplacement = false
	cfg1.Recommend.Replacement.PositiveReplacementDecay = 0.1
	cfg2.Recommend.Replacement.PositiveReplacementDecay = 0.2
	assert.Equal(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())
}
