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
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	config, _, err := LoadConfig("config.toml.template")
	assert.Nil(t, err)

	// database configuration
	assert.Equal(t, "redis://localhost:6379", config.Database.CacheStore)
	assert.Equal(t, "mysql://root@tcp(localhost:3306)/gorse?parseTime=true", config.Database.DataStore)
	assert.Equal(t, true, config.Database.AutoInsertUser)
	assert.Equal(t, false, config.Database.AutoInsertItem)
	assert.Equal(t, 200, config.Database.CacheSize)
	assert.Equal(t, []string{"star", "like"}, config.Database.PositiveFeedbackType)
	assert.Equal(t, []string{"like"}, config.Database.ClickFeedbackTypes)
	assert.Equal(t, "read", config.Database.ReadFeedbackType)
	assert.Equal(t, uint(0), config.Database.PositiveFeedbackTTL)
	assert.Equal(t, uint(0), config.Database.ItemTTL)

	// master configuration
	assert.Equal(t, 8086, config.Master.Port)
	assert.Equal(t, "0.0.0.0", config.Master.Host)
	assert.Equal(t, 8088, config.Master.HttpPort)
	assert.Equal(t, "0.0.0.0", config.Master.HttpHost)
	assert.Equal(t, 2, config.Master.SearchJobs)
	assert.Equal(t, 2, config.Master.FitJobs)
	assert.Equal(t, 10, config.Master.MetaTimeout)

	// server configuration
	assert.Equal(t, 20, config.Server.DefaultN)
	assert.Equal(t, "", config.Server.APIKey)

	// recommend configuration
	assert.Equal(t, 365, config.Recommend.PopularWindow)
	assert.Equal(t, 10, config.Recommend.FitPeriod)
	assert.Equal(t, 60, config.Recommend.SearchPeriod)
	assert.Equal(t, 100, config.Recommend.SearchEpoch)
	assert.Equal(t, 10, config.Recommend.SearchTrials)
	assert.Equal(t, 1, config.Recommend.RefreshRecommendPeriod)
	assert.Equal(t, "latest", config.Recommend.FallbackRecommend)
	assert.Equal(t, 20, config.Recommend.ExploreLatestNum)
}

func TestConfig_FillDefault(t *testing.T) {
	var config Config
	meta, err := toml.Decode("", &config)
	assert.Nil(t, err)
	config.FillDefault(meta)
	assert.Equal(t, *(*Config)(nil).LoadDefaultIfNil(), config)
}
