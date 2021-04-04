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
	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	config, _, err := LoadConfig("../misc/config_test/config.toml")
	assert.Nil(t, err)

	// database configuration
	assert.Equal(t, "redis://localhost:6379", config.Database.CacheStore)
	assert.Equal(t, "mysql://root@tcp(localhost:3306)/gitrec?parseTime=true", config.Database.DataStore)
	assert.Equal(t, true, config.Database.AutoInsertUser)
	assert.Equal(t, false, config.Database.AutoInsertItem)
	assert.Equal(t, []string{"star"}, config.Database.MatchFeedbackType)
	assert.Equal(t, []string{"fork"}, config.Database.RankFeedbackType)

	// similar configuration
	assert.Equal(t, 500, config.Similar.NumCache)
	assert.Equal(t, 120, config.Similar.UpdatePeriod)

	// latest configuration
	assert.Equal(t, 500, config.Latest.NumCache)
	assert.Equal(t, 30, config.Latest.UpdatePeriod)

	// popular configuration
	assert.Equal(t, 500, config.Popular.NumCache)
	assert.Equal(t, 120, config.Popular.UpdatePeriod)
	assert.Equal(t, 360, config.Popular.TimeWindow)

	// cf config
	assert.Equal(t, 1000, config.Collaborative.NumCached)
	assert.Equal(t, "als", config.Collaborative.CFModel)
	assert.Equal(t, 60, config.Collaborative.FitPeriod)
	assert.Equal(t, 60, config.Collaborative.PredictPeriod)

	assert.Equal(t, 0.05, config.Collaborative.Lr)
	assert.Equal(t, 0.01, config.Collaborative.Reg)
	assert.Equal(t, 100, config.Collaborative.NEpochs)
	assert.Equal(t, 10, config.Collaborative.NFactors)
	assert.Equal(t, 21, config.Collaborative.RandomState)
	assert.Equal(t, false, config.Collaborative.UseBias)
	assert.Equal(t, 0.0, config.Collaborative.InitMean)
	assert.Equal(t, 0.001, config.Collaborative.InitStdDev)
	assert.Equal(t, 1.0, config.Collaborative.Alpha)

	assert.Equal(t, 10, config.Collaborative.Verbose)
	assert.Equal(t, 100, config.Collaborative.Candidates)
	assert.Equal(t, 10, config.Collaborative.TopK)
	assert.Equal(t, 10000, config.Collaborative.NumTestUsers)

	// rank config
	assert.Equal(t, 60, config.Rank.FitPeriod)
	assert.Equal(t, "r", config.Rank.Task)

	assert.Equal(t, 0.05, config.Rank.Lr)
	assert.Equal(t, 0.01, config.Rank.Reg)
	assert.Equal(t, 100, config.Rank.NEpochs)
	assert.Equal(t, 10, config.Rank.NFactors)
	assert.Equal(t, 21, config.Rank.RandomState)
	assert.Equal(t, false, config.Rank.UseBias)
	assert.Equal(t, 0.0, config.Rank.InitMean)
	assert.Equal(t, 0.001, config.Rank.InitStdDev)

	assert.Equal(t, 10, config.Rank.Verbose)

	// master configuration
	assert.Equal(t, 8086, config.Master.Port)
	assert.Equal(t, "127.0.0.1", config.Master.Host)
	assert.Equal(t, 4, config.Master.Jobs)
	assert.Equal(t, 30, config.Master.MetaTimeout)

	// server configuration
	assert.Equal(t, "p@ssword", config.Server.APIKey)
}

func TestConfig_FillDefault(t *testing.T) {
	var config Config
	meta, err := toml.Decode("", &config)
	assert.Nil(t, err)
	config.FillDefault(meta)
	assert.Equal(t, *(*Config)(nil).LoadDefaultIfNil(), config)
}
