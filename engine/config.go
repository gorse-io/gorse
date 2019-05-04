package engine

import (
	"github.com/BurntSushi/toml"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
	"log"
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
	CacheSize       int    `toml:"cache_size"`
	UpdateThreshold int    `toml:"update_threshold"`
	CheckPeriod     int    `toml:"check_period"`
	Similarity      string `toml:"similarity"`
	ListSize        int    `toml:"list_size"`
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
	params := base.Params{}
	if metaData.IsDefined("params", "lr") {
		params[base.Lr] = config.Lr
	}
	if metaData.IsDefined("params", "reg") {
		params[base.Reg] = config.Reg
	}
	if metaData.IsDefined("params", "n_epochs") {
		params[base.NEpochs] = config.NEpochs
	}
	if metaData.IsDefined("params", "n_factors") {
		params[base.NFactors] = config.NFactors
	}
	if metaData.IsDefined("params", "random_state") {
		params[base.RandomState] = config.RandomState
	}
	if metaData.IsDefined("params", "use_bias") {
		params[base.UseBias] = config.UseBias
	}
	if metaData.IsDefined("params", "init_mean") {
		params[base.InitMean] = config.InitMean
	}
	if metaData.IsDefined("params", "init_std") {
		params[base.InitStdDev] = config.InitStdDev
	}
	if metaData.IsDefined("params", "init_low") {
		params[base.InitLow] = config.InitLow
	}
	if metaData.IsDefined("params", "init_high") {
		params[base.InitHigh] = config.InitHigh
	}
	if metaData.IsDefined("params", "n_user_clusters") {
		params[base.NUserClusters] = config.NUserClusters
	}
	if metaData.IsDefined("params", "n_item_clusters") {
		params[base.NItemClusters] = config.NItemClusters
	}
	if metaData.IsDefined("params", "type") {
		params[base.Type] = config.Type
	}
	if metaData.IsDefined("params", "user_based") {
		params[base.UserBased] = config.UserBased
	}
	if metaData.IsDefined("params", "similarity") {
		params[base.Similarity] = config.Similarity
	}
	if metaData.IsDefined("params", "k") {
		params[base.K] = config.K
	}
	if metaData.IsDefined("params", "min_k") {
		params[base.MinK] = config.MinK
	}
	if metaData.IsDefined("params", "optimizer") {
		params[base.Optimizer] = config.Optimizer
	}
	if metaData.IsDefined("params", "shrinkage") {
		params[base.Shrinkage] = config.Shrinkage
	}
	if metaData.IsDefined("params", "alpha") {
		params[base.Alpha] = config.Alpha
	}
	return params
}

func CreateModelFromName(name string, params base.Params) core.ModelInterface {
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
	default:
		log.Fatalf("unkown model %v\n", name)
	}
	panic("CreateModelFromName error")
}

func CreateSimilarityFromName(name string) base.FuncSimilarity {
	switch name {
	case "pearson":
		return base.PearsonSimilarity
	case "cosine":
		return base.CosineSimilarity
	case "msd":
		return base.MSDSimilarity
	default:
		log.Fatalf("unkown similarity %v\n", name)
	}
	panic("CreateSimilarityFromName error")
}

func LoadConfig(path string) (TomlConfig, toml.MetaData) {
	var conf TomlConfig
	metaData, err := toml.DecodeFile(path, &conf)
	if err != nil {
		log.Fatal(err)
	}
	return conf, metaData
}
