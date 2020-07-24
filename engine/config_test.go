package engine

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
	"path"
	"reflect"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	config, _ := LoadConfig("../example/file_config/config_test.toml")

	/* Check configuration */

	// server configuration
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)
	// database configuration
	assert.Equal(t, "~/.gorse/temp/database.db", config.Database.File)
	// recommend configuration
	assert.Equal(t, "svd", config.Recommend.Model)
	assert.Equal(t, "pearson", config.Recommend.Similarity)
	assert.Equal(t, 100, config.Recommend.CacheSize)
	assert.Equal(t, 10, config.Recommend.UpdateThreshold)
	assert.Equal(t, 1, config.Recommend.CheckPeriod)
	assert.Equal(t, 10, config.Recommend.FitJobs)
	assert.Equal(t, true, config.Recommend.Once)
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
	config, meta := LoadConfig("../example/file_config/config_test.toml")
	params := config.Params.ToParams(meta)
	assert.Equal(t, 9, len(params))

	// test on empty configuration
	config, meta = LoadConfig("../example/file_config/config_empty.toml")
	params = config.Params.ToParams(meta)
	assert.Equal(t, 0, len(params))
}

func TestTomlConfig_FillDefault(t *testing.T) {
	config, _ := LoadConfig("../example/file_config/config_empty.toml")

	/* Check configuration */

	// server configuration
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)
	// database configuration
	assert.Equal(t, path.Join(core.GorseDir, "gorse.db"), config.Database.File)
	// recommend configuration
	assert.Equal(t, "bpr", config.Recommend.Model)
	assert.Equal(t, "implicit", config.Recommend.Similarity)
	assert.Equal(t, 100, config.Recommend.CacheSize)
	assert.Equal(t, 10, config.Recommend.UpdateThreshold)
	assert.Equal(t, 1, config.Recommend.CheckPeriod)
	assert.Equal(t, 1, config.Recommend.FitJobs)

	config, _ = LoadConfig("../example/file_config/config_not_exist.toml")

	/* Check configuration */

	// server configuration
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)
	// database configuration
	assert.Equal(t, path.Join(core.GorseDir, "gorse.db"), config.Database.File)
	// recommend configuration
	assert.Equal(t, "bpr", config.Recommend.Model)
	assert.Equal(t, "implicit", config.Recommend.Similarity)
	assert.Equal(t, 100, config.Recommend.CacheSize)
	assert.Equal(t, 10, config.Recommend.UpdateThreshold)
	assert.Equal(t, 1, config.Recommend.CheckPeriod)
	assert.Equal(t, 1, config.Recommend.FitJobs)
	assert.Equal(t, false, config.Recommend.Once)
}

func TestLoadSimilarity(t *testing.T) {
	assert.Equal(t, reflect.ValueOf(base.PearsonSimilarity).Pointer(),
		reflect.ValueOf(LoadSimilarity("pearson")).Pointer())
	assert.Equal(t, reflect.ValueOf(base.CosineSimilarity).Pointer(),
		reflect.ValueOf(LoadSimilarity("cosine")).Pointer())
	assert.Equal(t, reflect.ValueOf(base.MSDSimilarity).Pointer(),
		reflect.ValueOf(LoadSimilarity("msd")).Pointer())
	assert.Equal(t, reflect.ValueOf(base.ImplicitSimilarity).Pointer(),
		reflect.ValueOf(LoadSimilarity("implicit")).Pointer())

	// Test similarity not existed
	assert.Equal(t, uintptr(0), reflect.ValueOf(LoadSimilarity("none")).Pointer())
}

func TestLoadModel(t *testing.T) {
	type Model struct {
		name   string
		typeOf reflect.Type
	}
	models := []Model{
		{"bpr", reflect.TypeOf(model.NewBPR(nil))},
		{"knn", reflect.TypeOf(model.NewKNN(nil))},
		{"als", reflect.TypeOf(model.NewALS(nil))},
		{"knn", reflect.TypeOf(model.NewKNN(nil))},
	}
	for _, m := range models {
		assert.Equal(t, m.typeOf, reflect.TypeOf(LoadModel(m.name, nil)))
	}

	// Test model not existed
	assert.Equal(t, nil, LoadModel("none", nil))
}
