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
	config, _, err := LoadConfig("../example/config/config.toml")
	assert.Nil(t, err)
	// server configuration
	assert.Equal(t, "0.0.0.0", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, 10, config.Server.DefaultN)
	// database configuration
	assert.Equal(t, "redis://127.0.0.1:6398", config.Database.Path)
	// params configuration
	assert.Equal(t, 0.05, config.Leader.Params.Lr)
	assert.Equal(t, 0.01, config.Leader.Params.Reg)
	assert.Equal(t, 100, config.Leader.Params.NEpochs)
	assert.Equal(t, 10, config.Leader.Params.NFactors)
	assert.Equal(t, 21, config.Leader.Params.RandomState)
	assert.Equal(t, false, config.Leader.Params.UseBias)
	assert.Equal(t, 0.0, config.Leader.Params.InitMean)
	assert.Equal(t, 0.001, config.Leader.Params.InitStdDev)
	assert.Equal(t, 1.0, config.Leader.Params.Weight)
}

func TestConfig_FillDefault(t *testing.T) {
	var config Config
	meta, err := toml.Decode("", &config)
	assert.Nil(t, err)
	config.FillDefault(meta)
	assert.Equal(t, *(*Config)(nil).LoadDefaultIfNil(), config)
}
