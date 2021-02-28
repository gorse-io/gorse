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
	config, _, err := LoadConfig("../config/config.toml.template")
	assert.Nil(t, err)

	// database configuration
	assert.Equal(t, "redis://192.168.199.246:6379", config.Database.CacheStore)
	assert.Equal(t, "mysql://root@tcp(127.0.0.1:3306)/gitrec?parseTime=true", config.Database.DataStore)
	assert.Equal(t, true, config.Database.AutoInsertUser)
	assert.Equal(t, false, config.Database.AutoInsertItem)
	assert.Equal(t, 60, config.Database.ClusterMetaTimeout)

	// latest configuration
	assert.Equal(t, 98, config.Latest.NumLatest)
	assert.Equal(t, 10, config.Latest.UpdatePeriod)

	// popular configuration
	assert.Equal(t, 99, config.Popular.NumPopular)
	assert.Equal(t, 1442, config.Popular.UpdatePeriod)
	assert.Equal(t, 360, config.Popular.TimeWindow)

	// cf config
	assert.Equal(t, 799, config.CF.NumCF)
	assert.Equal(t, "ccd", config.CF.CFModel)
	assert.Equal(t, 1441, config.CF.UpdatePeriod)

	assert.Equal(t, 0.05, config.CF.Lr)
	assert.Equal(t, 0.01, config.CF.Reg)
	assert.Equal(t, 100, config.CF.NEpochs)
	assert.Equal(t, 10, config.CF.NFactors)
	assert.Equal(t, 21, config.CF.RandomState)
	assert.Equal(t, false, config.CF.UseBias)
	assert.Equal(t, 0.0, config.CF.InitMean)
	assert.Equal(t, 0.001, config.CF.InitStdDev)
	assert.Equal(t, 1.0, config.CF.Alpha)

	assert.Equal(t, 4, config.CF.FitJobs)
	assert.Equal(t, 10, config.CF.Verbose)
	assert.Equal(t, 100, config.CF.Candidates)
	assert.Equal(t, 10, config.CF.TopK)
	assert.Equal(t, 10000, config.CF.NumTestUsers)

	// rank config
	assert.Equal(t, "r", config.Rank.Task)

	assert.Equal(t, 0.05, config.Rank.Lr)
	assert.Equal(t, 0.01, config.Rank.Reg)
	assert.Equal(t, 100, config.Rank.NEpochs)
	assert.Equal(t, 10, config.Rank.NFactors)
	assert.Equal(t, 21, config.Rank.RandomState)
	assert.Equal(t, false, config.Rank.UseBias)
	assert.Equal(t, 0.0, config.Rank.InitMean)
	assert.Equal(t, 0.001, config.Rank.InitStdDev)

	assert.Equal(t, 4, config.Rank.FitJobs)
	assert.Equal(t, 10, config.Rank.Verbose)

	// master configuration
	assert.Equal(t, 8086, config.Master.Port)
	assert.Equal(t, "127.0.0.1", config.Master.Host)
	assert.Equal(t, 4, config.Master.Jobs)

	// server configuration
	assert.Equal(t, 10, config.Server.DefaultReturnNumber)
}

func TestConfig_FillDefault(t *testing.T) {
	var config Config
	meta, err := toml.Decode("", &config)
	assert.Nil(t, err)
	config.FillDefault(meta)
	assert.Equal(t, *(*Config)(nil).LoadDefaultIfNil(), config)
}
