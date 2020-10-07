// Copyright 2020 Zhenghao Zhang
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
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
	"path"
	"reflect"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	config, _, err := LoadConfig("../example/config/config_test.toml")
	if err != nil {
		t.Fatal(err)
	}

	/* Check configuration */

	// cmd configuration
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)
	// database configuration
	assert.Equal(t, "database", config.Database.Path)
	// recommend configuration
	assert.Equal(t, "bpr", config.Recommend.Model)
	assert.Equal(t, "tag", config.Recommend.Similarity)
	assert.Equal(t, 1024, config.Recommend.TopN)
	assert.Equal(t, 10, config.Recommend.UpdateThreshold)
	assert.Equal(t, 100, config.Recommend.FitThreshold)
	assert.Equal(t, 7, config.Recommend.CheckPeriod)
	assert.Equal(t, 8, config.Recommend.FitJobs)
	assert.Equal(t, 9, config.Recommend.UpdateJobs)
	assert.Equal(t, []string{"pop", "latest", "neighbor"}, config.Recommend.Collectors)
	// params configuration
	assert.Equal(t, 0.05, config.Params.Lr)
	assert.Equal(t, 0.01, config.Params.Reg)
	assert.Equal(t, 100, config.Params.NEpochs)
	assert.Equal(t, 10, config.Params.NFactors)
	assert.Equal(t, 21, config.Params.RandomState)
	assert.Equal(t, false, config.Params.UseBias)
	assert.Equal(t, 0.0, config.Params.InitMean)
	assert.Equal(t, 0.001, config.Params.InitStdDev)
	assert.Equal(t, 1.0, config.Params.Alpha)
}

func TestParamsConfig_ToParams(t *testing.T) {
	// test on full configuration
	config, meta, err := LoadConfig("../example/config/config_test.toml")
	if err != nil {
		t.Fatal(err)
	}
	params := config.Params.ToParams(meta)
	assert.Equal(t, 9, len(params))

	// test on empty configuration
	config, meta, err = LoadConfig("../example/config/config_empty.toml")
	if err != nil {
		t.Fatal(err)
	}
	params = config.Params.ToParams(meta)
	assert.Equal(t, 0, len(params))
}

func TestConfig_FillDefault(t *testing.T) {
	config, _, err := LoadConfig("../example/config/config_empty.toml")
	if err != nil {
		t.Fatal(err)
	}

	/* Check configuration */

	// cmd configuration
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)
	// database configuration
	assert.Equal(t, path.Join(model.GorseDir, "database"), config.Database.Path)
	// recommend configuration
	assert.Equal(t, "als", config.Recommend.Model)
	assert.Equal(t, "feedback", config.Recommend.Similarity)
	assert.Equal(t, 100, config.Recommend.TopN)
	assert.Equal(t, 10, config.Recommend.UpdateThreshold)
	assert.Equal(t, 100, config.Recommend.FitThreshold)
	assert.Equal(t, 1, config.Recommend.CheckPeriod)
	assert.Equal(t, 1, config.Recommend.FitJobs)
	assert.Equal(t, 1, config.Recommend.UpdateJobs)
	assert.Equal(t, []string{"all"}, config.Recommend.Collectors)

	config, _, err = LoadConfig("../example/config/config_not_exist.toml")
	assert.NotNil(t, err)
}

func TestLoadModel(t *testing.T) {
	type Model struct {
		name   string
		typeOf reflect.Type
	}
	models := []Model{
		{"bpr", reflect.TypeOf(model.NewBPR(nil))},
		{"als", reflect.TypeOf(model.NewALS(nil))},
	}
	for _, m := range models {
		assert.Equal(t, m.typeOf, reflect.TypeOf(LoadModel(m.name, nil)))
	}

	// Test model not existed
	assert.Equal(t, nil, LoadModel("none", nil))
}
