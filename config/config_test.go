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
	"os"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	config, err := LoadConfig("config.toml.template")
	assert.NoError(t, err)

	// database configuration
	assert.Equal(t, "redis://localhost:6379/0", config.Database.CacheStore)
	assert.Equal(t, "mysql://gorse:gorse_pass@tcp(localhost:3306)/gorse?parseTime=true", config.Database.DataStore)
	assert.Equal(t, true, config.Database.AutoInsertUser)
	assert.Equal(t, false, config.Database.AutoInsertItem)
	assert.Equal(t, 100, config.Database.CacheSize)
	assert.Equal(t, []string{"star", "like"}, config.Database.PositiveFeedbackType)
	assert.Equal(t, []string{"read"}, config.Database.ReadFeedbackTypes)
	assert.Equal(t, uint(0), config.Database.PositiveFeedbackTTL)
	assert.Equal(t, uint(0), config.Database.ItemTTL)

	// master configuration
	assert.Equal(t, 8086, config.Master.Port)
	assert.Equal(t, "0.0.0.0", config.Master.Host)
	assert.Equal(t, 8088, config.Master.HttpPort)
	assert.Equal(t, "0.0.0.0", config.Master.HttpHost)
	assert.Equal(t, 4, config.Master.NumJobs)
	assert.Equal(t, 10, config.Master.MetaTimeout)
	assert.Equal(t, "admin", config.Master.DashboardUserName)
	assert.Equal(t, "password", config.Master.DashboardPassword)

	// server configuration
	assert.Equal(t, 10, config.Server.DefaultN)
	assert.Equal(t, "", config.Server.APIKey)

	// recommend configuration
	assert.Equal(t, 30, config.Recommend.PopularWindow)
	assert.Equal(t, 360, config.Recommend.FitPeriod)
	assert.Equal(t, 60, config.Recommend.SearchPeriod)
	assert.Equal(t, 100, config.Recommend.SearchEpoch)
	assert.Equal(t, 10, config.Recommend.SearchTrials)
	assert.Equal(t, 1, config.Recommend.CheckRecommendPeriod)
	assert.Equal(t, 1, config.Recommend.RefreshRecommendPeriod)
	assert.Equal(t, []string{"item_based", "latest"}, config.Recommend.FallbackRecommend)
	assert.Equal(t, map[string]float64{"popular": 0.1, "latest": 0.2}, config.Recommend.ExploreRecommend)
	assert.Equal(t, 10, config.Recommend.NumFeedbackFallbackItemBased)
	value, exist := config.Recommend.GetExploreRecommend("popular")
	assert.Equal(t, true, exist)
	assert.Equal(t, 0.1, value)
	value, exist = config.Recommend.GetExploreRecommend("latest")
	assert.Equal(t, true, exist)
	assert.Equal(t, 0.2, value)
	_, exist = config.Recommend.GetExploreRecommend("unknown")
	assert.Equal(t, false, exist)
	assert.Equal(t, "similar", config.Recommend.ItemNeighborType)
	assert.False(t, config.Recommend.EnableItemNeighborIndex)
	assert.Equal(t, float32(0.8), config.Recommend.ItemNeighborIndexRecall)
	assert.Equal(t, 3, config.Recommend.ItemNeighborIndexFitEpoch)
	assert.Equal(t, "similar", config.Recommend.UserNeighborType)
	assert.False(t, config.Recommend.EnableUserNeighborIndex)
	assert.Equal(t, float32(0.8), config.Recommend.UserNeighborIndexRecall)
	assert.Equal(t, 3, config.Recommend.UserNeighborIndexFitEpoch)
	assert.True(t, config.Recommend.EnableColRecommend)
	assert.False(t, config.Recommend.EnableColIndex)
	assert.Equal(t, float32(0.9), config.Recommend.ColIndexRecall)
	assert.Equal(t, 3, config.Recommend.ColIndexFitEpoch)
	assert.False(t, config.Recommend.EnableItemBasedRecommend)
	assert.True(t, config.Recommend.EnableUserBasedRecommend)
	assert.False(t, config.Recommend.EnablePopularRecommend)
	assert.True(t, config.Recommend.EnableLatestRecommend)
	assert.True(t, config.Recommend.EnableClickThroughPrediction)
}

func TestSetDefault(t *testing.T) {
	err := viper.ReadConfig(strings.NewReader(""))
	assert.NoError(t, err)
	var config Config
	err = viper.Unmarshal(&config)
	assert.NoError(t, err)
	assert.Equal(t, (*Config)(nil).LoadDefaultIfNil(), &config)
}

type environmentVariable struct {
	key   string
	value string
}

func TestBindEnv(t *testing.T) {
	variables := []environmentVariable{
		{"GORSE_CACHE_STORE", "<cache_store>"},
		{"GORSE_DATA_STORE", "<data_store>"},
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

	config, err := LoadConfig("config.toml.template")
	assert.NoError(t, err)
	assert.Equal(t, "<cache_store>", config.Database.CacheStore)
	assert.Equal(t, "<data_store>", config.Database.DataStore)
	assert.Equal(t, 123, config.Master.Port)
	assert.Equal(t, "<master_host>", config.Master.Host)
	assert.Equal(t, 456, config.Master.HttpPort)
	assert.Equal(t, "<master_http_host>", config.Master.HttpHost)
	assert.Equal(t, 789, config.Master.NumJobs)
	assert.Equal(t, "user_name", config.Master.DashboardUserName)
	assert.Equal(t, "password", config.Master.DashboardPassword)
	assert.Equal(t, "<server_api_key>", config.Server.APIKey)
}
