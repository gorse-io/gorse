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
	// params configuration
	assert.Equal(t, 0.05, config.Params.Lr)
	assert.Equal(t, 0.01, config.Params.Reg)
	assert.Equal(t, 100, config.Params.NEpochs)
	assert.Equal(t, 10, config.Params.NFactors)
	assert.Equal(t, 21, config.Params.RandomState)
	assert.Equal(t, false, config.Params.UseBias)
	assert.Equal(t, 0.0, config.Params.InitMean)
	assert.Equal(t, 0.001, config.Params.InitStdDev)
	assert.Equal(t, 10, config.Params.NUserClusters)
	assert.Equal(t, 10, config.Params.NItemClusters)
	assert.Equal(t, "baseline", config.Params.Type)
	assert.Equal(t, true, config.Params.UserBased)
	assert.Equal(t, "pearson", config.Params.Similarity)
	assert.Equal(t, 100, config.Params.K)
	assert.Equal(t, 5, config.Params.MinK)
	assert.Equal(t, "bpr", config.Params.Optimizer)
	assert.Equal(t, 1.0, config.Params.Alpha)
}

func TestParamsConfig_ToParams(t *testing.T) {
	// test on full configuration
	config, meta := LoadConfig("../example/file_config/config_test.toml")
	params := config.Params.ToParams(meta)
	assert.Equal(t, 20, len(params))

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
	assert.Equal(t, "svd", config.Recommend.Model)
	assert.Equal(t, "pearson", config.Recommend.Similarity)
	assert.Equal(t, 100, config.Recommend.CacheSize)
	assert.Equal(t, 10, config.Recommend.UpdateThreshold)
	assert.Equal(t, 1, config.Recommend.CheckPeriod)

	config, _ = LoadConfig("../example/file_config/config_not_exist.toml")

	/* Check configuration */

	// server configuration
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)
	// database configuration
	assert.Equal(t, path.Join(core.GorseDir, "gorse.db"), config.Database.File)
	// recommend configuration
	assert.Equal(t, "svd", config.Recommend.Model)
	assert.Equal(t, "pearson", config.Recommend.Similarity)
	assert.Equal(t, 100, config.Recommend.CacheSize)
	assert.Equal(t, 10, config.Recommend.UpdateThreshold)
	assert.Equal(t, 1, config.Recommend.CheckPeriod)
}

func TestLoadSimilarity(t *testing.T) {
	assert.Equal(t, reflect.ValueOf(base.PearsonSimilarity).Pointer(),
		reflect.ValueOf(LoadSimilarity("pearson")).Pointer())
	assert.Equal(t, reflect.ValueOf(base.CosineSimilarity).Pointer(),
		reflect.ValueOf(LoadSimilarity("cosine")).Pointer())
	assert.Equal(t, reflect.ValueOf(base.MSDSimilarity).Pointer(),
		reflect.ValueOf(LoadSimilarity("msd")).Pointer())

	// Test similarity not existed
	assert.Equal(t, uintptr(0), reflect.ValueOf(LoadSimilarity("none")).Pointer())
}

func TestLoadModel(t *testing.T) {
	type Model struct {
		name   string
		typeOf reflect.Type
	}
	models := []Model{
		{"svd", reflect.TypeOf(model.NewSVD(nil))},
		{"knn", reflect.TypeOf(model.NewKNN(nil))},
		{"slope_one", reflect.TypeOf(model.NewSlopOne(nil))},
		{"co_clustering", reflect.TypeOf(model.NewCoClustering(nil))},
		{"nmf", reflect.TypeOf(model.NewNMF(nil))},
		{"wrmf", reflect.TypeOf(model.NewWRMF(nil))},
		{"svd++", reflect.TypeOf(model.NewSVDpp(nil))},
	}
	for _, m := range models {
		assert.Equal(t, m.typeOf, reflect.TypeOf(LoadModel(m.name, nil)))
	}

	// Test model not existed
	assert.Equal(t, nil, LoadModel("none", nil))
}
