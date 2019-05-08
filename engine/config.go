package engine

import (
	"github.com/BurntSushi/toml"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
	"log"
	"path"
)

type TomlConfig struct {
	Server    ServerConfig    `toml:"server"`
	Database  DatabaseConfig  `toml:"database"`
	Params    ParamsConfig    `toml:"params"`
	Recommend RecommendConfig `toml:"recommend"`
}

type ServerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type DatabaseConfig struct {
	File string `toml:"file"`
}

type RecommendConfig struct {
	Model           string `toml:"model"`
	Similarity      string `toml:"similarity"`
	CacheSize       int    `toml:"cache_size"`
	UpdateThreshold int    `toml:"update_threshold"`
	CheckPeriod     int    `toml:"check_period"`
}

type ParamsConfig struct {
	// Hyper-parameters
	Lr            float64 `toml:"lr"`              // learning rate
	Reg           float64 `toml:"reg"`             // regularization strength
	NEpochs       int     `toml:"n_epochs"`        // number of epochs
	NFactors      int     `toml:"n_factors"`       // number of factors
	RandomState   int     `toml:"random_state"`    // random state (seed)
	UseBias       bool    `toml:"use_bias"`        // use bias
	InitMean      float64 `toml:"init_mean"`       // mean of gaussian initial parameter
	InitStdDev    float64 `toml:"init_std"`        // standard deviation of gaussian initial parameter
	InitLow       float64 `toml:"init_low"`        // lower bound of uniform initial parameter
	InitHigh      float64 `toml:"init_high"`       // upper bound of uniform initial parameter
	NUserClusters int     `toml:"n_user_clusters"` // number of user cluster
	NItemClusters int     `toml:"n_item_clusters"` // number of item cluster
	Type          string  `toml:"type"`            // type for KNN
	UserBased     bool    `toml:"user_based"`      // user based if true. otherwise item based.
	Similarity    string  `toml:"similarity"`      // similarity metrics
	K             int     `toml:"k"`               // number of neighbors
	MinK          int     `toml:"min_k"`           // least number of neighbors
	Optimizer     string  `toml:"optimizer"`       // optimizer for optimization (SGD/ALS/BPR)
	Shrinkage     int     `toml:"shrinkage"`       // shrinkage strength of similarity
	Alpha         float64 `toml:"alpha"`           // alpha value, depend on context
}

func (config *ParamsConfig) ToParams(metaData toml.MetaData) base.Params {
	type ParamValues struct {
		name  string
		key   base.ParamName
		value interface{}
	}
	values := []ParamValues{
		{"lr", base.Lr, config.Lr},
		{"reg", base.Reg, config.Reg},
		{"n_epochs", base.NEpochs, config.NEpochs},
		{"n_factors", base.NFactors, config.NFactors},
		{"random_state", base.RandomState, config.RandomState},
		{"use_bias", base.UseBias, config.UseBias},
		{"init_mean", base.InitMean, config.InitMean},
		{"init_std", base.InitStdDev, config.InitStdDev},
		{"init_low", base.InitLow, config.InitLow},
		{"init_high", base.InitHigh, config.InitHigh},
		{"n_user_clusters", base.NUserClusters, config.NUserClusters},
		{"n_item_clusters", base.NItemClusters, config.NItemClusters},
		{"type", base.Type, config.Type},
		{"user_based", base.UserBased, config.UserBased},
		{"similarity", base.Similarity, config.Similarity},
		{"k", base.K, config.K},
		{"min_k", base.MinK, config.MinK},
		{"optimizer", base.Optimizer, config.Optimizer},
		{"shrinkage", base.Shrinkage, config.Shrinkage},
		{"alpha", base.Alpha, config.Alpha},
	}
	params := base.Params{}
	for _, v := range values {
		if metaData.IsDefined("params", v.name) {
			params[v.key] = v.value
		}
	}
	return params
}

// LoadModel creates model from name and parameters.
func LoadModel(name string, params base.Params) core.ModelInterface {
	switch name {
	case "svd":
		return model.NewSVD(params)
	case "knn":
		return model.NewKNN(params)
	case "slope_one":
		return model.NewSlopOne(params)
	case "co_clustering":
		return model.NewCoClustering(params)
	case "nmf":
		return model.NewNMF(params)
	case "wrmf":
		return model.NewWRMF(params)
	case "svd++":
		return model.NewSVDpp(params)
	}
	return nil
}

// LoadSimilarity creates similarity metric from name.
func LoadSimilarity(name string) base.FuncSimilarity {
	switch name {
	case "pearson":
		return base.PearsonSimilarity
	case "cosine":
		return base.CosineSimilarity
	case "msd":
		return base.MSDSimilarity
	case "intersect":
		return base.IntersectSimilarity
	}
	return nil
}

// FillDefault fill default values for missing values.
func (config *TomlConfig) FillDefault(meta toml.MetaData) {
	if !meta.IsDefined("server", "host") {
		config.Server.Host = "127.0.0.1"
	}
	if !meta.IsDefined("server", "port") {
		config.Server.Port = 8080
	}
	if !meta.IsDefined("database", "file") {
		config.Database.File = path.Join(core.GorseDir, "gorse.db")
	}
	if !meta.IsDefined("recommend", "model") {
		config.Recommend.Model = "svd"
	}
	if !meta.IsDefined("recommend", "cache_size") {
		config.Recommend.CacheSize = 100
	}
	if !meta.IsDefined("recommend", "update_threshold") {
		config.Recommend.UpdateThreshold = 10
	}
	if !meta.IsDefined("recommend", "check_period") {
		config.Recommend.CheckPeriod = 1
	}
	if !meta.IsDefined("recommend", "similarity") {
		config.Recommend.Similarity = "pearson"
	}
}

// LoadConfig loads configuration from toml file.
func LoadConfig(path string) (TomlConfig, toml.MetaData) {
	var conf TomlConfig
	metaData, err := toml.DecodeFile(path, &conf)
	if err != nil {
		log.Print(err)
	}
	conf.FillDefault(metaData)
	return conf, metaData
}
