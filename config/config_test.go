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
	config, _, err := LoadConfig("../test/config/config.toml")
	assert.Nil(t, err)

	// common configuration
	assert.Equal(t, 1000, config.Common.CacheSize)
	assert.Equal(t, 10, config.Common.RetryLimit)
	assert.Equal(t, 1, config.Common.RetryInterval)

	// server configuration
	assert.Equal(t, "0.0.0.0", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, 10, config.Server.DefaultReturnNumber)

	// worker configuration
	assert.Equal(t, "127.0.0.1:9000", config.Worker.LeaderAddr)
	assert.Equal(t, "127.0.0.1", config.Worker.Host)
	assert.Equal(t, 9001, config.Worker.GossipPort)
	assert.Equal(t, 9002, config.Worker.RPCPort)
	assert.Equal(t, 10, config.Worker.PredictInterval)
	assert.Equal(t, 2, config.Worker.GossipInterval)

	// database configuration
	assert.Equal(t, "redis://127.0.0.1:6398", config.Database.DataStore)

	// leader configuration
	assert.Equal(t, 9000, config.Master.Port)
	assert.Equal(t, "127.0.0.1", config.Master.Host)
	assert.Equal(t, "als", config.Master.Model)
	assert.Equal(t, 60, config.Master.FitInterval)
	assert.Equal(t, 3, config.Master.BroadcastInterval)
	// params
	assert.Equal(t, 0.05, config.Master.Params.Lr)
	assert.Equal(t, 0.01, config.Master.Params.Reg)
	assert.Equal(t, 100, config.Master.Params.NEpochs)
	assert.Equal(t, 10, config.Master.Params.NFactors)
	assert.Equal(t, 21, config.Master.Params.RandomState)
	assert.Equal(t, false, config.Master.Params.UseBias)
	assert.Equal(t, 0.0, config.Master.Params.InitMean)
	assert.Equal(t, 0.001, config.Master.Params.InitStdDev)
	assert.Equal(t, 1.0, config.Master.Params.Weight)
	// fit
	assert.Equal(t, 4, config.Master.Fit.Jobs)
	assert.Equal(t, 10, config.Master.Fit.Verbose)
	assert.Equal(t, 100, config.Master.Fit.Candidates)
	assert.Equal(t, 10, config.Master.Fit.TopK)
	assert.Equal(t, 10000, config.Master.Fit.NumTestUsers)
}

func TestConfig_FillDefault(t *testing.T) {
	var config Config
	meta, err := toml.Decode("", &config)
	assert.Nil(t, err)
	config.FillDefault(meta)
	assert.Equal(t, *(*Config)(nil).LoadDefaultIfNil(), config)
}
