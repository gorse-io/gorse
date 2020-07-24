package engine

import (
	"github.com/BurntSushi/toml"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
	"log"
	"path"
)

// TomlConfig is the configuration for the engine.
type TomlConfig struct {
	Server    ServerConfig    `toml:"server"`
	Database  DatabaseConfig  `toml:"database"`
	Params    ParamsConfig    `toml:"params"`
	Recommend RecommendConfig `toml:"recommend"`
}

// ServerConfig is the configuration for the server.
type ServerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

// DatabaseConfig is the configuration for the database.
type DatabaseConfig struct {
	File string `toml:"file"`
}

// RecommendConfig is the configuration for recommendation.
type RecommendConfig struct {
	Model           string `toml:"model"`
	Similarity      string `toml:"similarity"`
	CacheSize       int    `toml:"cache_size"`
	UpdateThreshold int    `toml:"update_threshold"`
	CheckPeriod     int    `toml:"check_period"`
	FitJobs         int    `toml:"fit_jobs"`
	Once            bool   `toml:"once"`
}

// ParamsConfig is the configuration for hyper-parameters of the recommendation model.
type ParamsConfig struct {
	// Hyper-parameters
	Lr          float64 `toml:"lr"`           // learning rate
	Reg         float64 `toml:"reg"`          // regularization strength
	NEpochs     int     `toml:"n_epochs"`     // number of epochs
	NFactors    int     `toml:"n_factors"`    // number of factors
	RandomState int     `toml:"random_state"` // random state (seed)
	UseBias     bool    `toml:"use_bias"`     // use bias
	InitMean    float64 `toml:"init_mean"`    // mean of gaussian initial parameter
	InitStdDev  float64 `toml:"init_std"`     // standard deviation of gaussian initial parameter
	Alpha       float64 `toml:"alpha"`        // alpha value, depend on context
}

// ToParams convert a configuration for hyper-parameters into hyper-parameters.
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
	case "bpr":
		return model.NewBPR(params)
	case "als":
		return model.NewALS(params)
	case "knn":
		return model.NewKNN(params)
	case "item_pop":
		return model.NewItemPop(params)
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
	case "implicit":
		return base.ImplicitSimilarity
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
		config.Recommend.Model = "bpr"
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
	if !meta.IsDefined("recommend", "fit_jobs") {
		config.Recommend.FitJobs = 1
	}
	if !meta.IsDefined("recommend", "similarity") {
		config.Recommend.Similarity = "implicit"
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
