package server

import (
	"github.com/zhenghaoz/gorse/base"
)

type TomlConfig struct {
	Server   ServeConfig    `toml:"server"`
	Database DatabaseConfig `toml:"database"`
	Params   ParamsConfig   `toml:"params"`
}

type ServeConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type DatabaseConfig struct {
	Driver string `toml:"driver"`
	Access string `toml:"access"`
}

type RecommendConfig struct {
	Base            string  `toml:"base"`
	CacheSize       int     `toml:"cache_size"`
	NewUser         string  `toml:"new_user"`
	UpdateThreshold int     `toml:"update_threshold"`
	ExploreProp     float64 `toml:"explore_prop"`
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

func (config *ParamsConfig) ToParams() base.Params {
	params := base.Params{}
	params[base.Lr] = config.Lr
	params[base.Reg] = config.Reg
	params[base.NEpochs] = config.NEpochs
	params[base.NFactors] = config.NFactors
	params[base.RandomState] = config.RandomState
	params[base.UseBias] = config.UseBias
	params[base.InitMean] = config.InitMean
	params[base.InitStdDev] = config.InitStdDev
	params[base.InitLow] = config.InitLow
	params[base.InitHigh] = config.InitHigh
	params[base.NUserClusters] = config.NUserClusters
	params[base.NItemClusters] = config.NItemClusters
	params[base.Type] = config.Type
	params[base.UserBased] = config.UserBased
	params[base.Similarity] = config.Similarity
	params[base.K] = config.K
	params[base.MinK] = config.MinK
	params[base.Optimizer] = config.Optimizer
	params[base.Shrinkage] = config.Shrinkage
	params[base.Alpha] = config.Alpha
	return params
}
