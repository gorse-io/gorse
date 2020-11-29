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

// Config is the configuration for the engine.
type Config struct {
	Server    ServerConfig    `toml:"server"`
	Database  DatabaseConfig  `toml:"database"`
	Params    ParamsConfig    `toml:"params"`
	Recommend RecommendConfig `toml:"recommend"`
}

// ServerConfig is the configuration for the cmd.
type ServerConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	DefaultN int
}

func (config *ServerConfig) LoadDefaultIfNil() *ServerConfig {
	if config == nil {
		return &ServerConfig{
			Host:     "127.0.0.1",
			Port:     6381,
			DefaultN: 100,
		}
	}
	return config
}

type FitConfig struct {
	Jobs       int `toml:"n_jobs"`
	Verbose    int `toml:"verbose"`
	Candidates int `toml:"n_candidates"`
	TopK       int
}

func (config *FitConfig) LoadDefaultIfNil() *FitConfig {
	if config == nil {
		return &FitConfig{
			Jobs:       1,
			Verbose:    1,
			Candidates: 100,
			TopK:       10,
		}
	}
	return config
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
	Weight      float64 `toml:"alpha"`        // alpha value, depend on context
}

// DatabaseConfig is the configuration for the database.
type DatabaseConfig struct {
	Path string `toml:"path"`
}

// RecommendConfig is the configuration for recommendation.
type RecommendConfig struct {
	Model           string `toml:"model"`
	Similarity      string `toml:"similarity"`
	TopN            int    `toml:"top_n"`
	UpdateThreshold int    `toml:"update_threshold"`
	FitThreshold    int    `toml:"fit_threshold"`
	CheckPeriod     int    `toml:"check_period"`
	FitJobs         int    `toml:"fit_jobs"`
	UpdateJobs      int    `toml:"update_jobs"`
	Collectors      []string
}

// ToParams convert a configuration for hyper-parameters into hyper-parameters.
//func (config *ParamsConfig) ToParams(metaData toml.MetaData) Params {
//	type ParamValues struct {
//		name  string
//		key   ParamName
//		value interface{}
//	}
//	values := []ParamValues{
//		{"lr", Lr, config.Lr},
//		{"reg", Reg, config.Reg},
//		{"n_epochs", NEpochs, config.NEpochs},
//		{"n_factors", NFactors, config.NFactors},
//		{"random_state", RandomState, config.RandomState},
//		{"use_bias", UseBias, config.UseBias},
//		{"init_mean", InitMean, config.InitMean},
//		{"init_std", InitStdDev, config.InitStdDev},
//		{"alpha", Weight, config.Weight},
//	}
//	params := Params{}
//	for _, v := range values {
//		if metaData.IsDefined("params", v.name) {
//			params[v.key] = v.value
//		}
//	}
//	return params
//}

// LoadModel creates model from name and parameters.
//func LoadModel(name string, params Params) temp.RankModel {
//	switch name {
//	//case "bpr":
//	//	return model.NewBPR(params)
//	//case "als":
//	//	return model.NewALS(params)
//	case "item_pop":
//		return temp.NewItemPop(params)
//	}
//	return nil
//}

// FillDefault fill default values for missing values.
//func (config *Config) FillDefault(meta toml.MetaData) {
//	if !meta.IsDefined("server", "host") {
//		config.Server.Host = "127.0.0.1"
//	}
//	if !meta.IsDefined("server", "port") {
//		config.Server.Port = 8080
//	}
//	if !meta.IsDefined("database", "path") {
//		config.Database.Path = path.Join(temp.GorseDir, "database")
//	}
//	if !meta.IsDefined("recommend", "model") {
//		config.Recommend.Model = "als"
//	}
//	if !meta.IsDefined("recommend", "top_n") {
//		config.Recommend.TopN = 100
//	}
//	if !meta.IsDefined("recommend", "update_threshold") {
//		config.Recommend.UpdateThreshold = 10
//	}
//	if !meta.IsDefined("recommend", "fit_threshold") {
//		config.Recommend.FitThreshold = 100
//	}
//	if !meta.IsDefined("recommend", "check_period") {
//		config.Recommend.CheckPeriod = 1
//	}
//	if !meta.IsDefined("recommend", "fit_jobs") {
//		config.Recommend.FitJobs = 1
//	}
//	if !meta.IsDefined("recommend", "update_jobs") {
//		config.Recommend.UpdateJobs = 1
//	}
//	if !meta.IsDefined("recommend", "similarity") {
//		config.Recommend.Similarity = "feedback"
//	}
//	if !meta.IsDefined("recommend", "collectors") {
//		config.Recommend.Collectors = []string{"all"}
//	}
//}

// LoadConfig loads configuration from toml file.
//func LoadConfig(path string) (*Config, toml.MetaData, error) {
//	var conf Config
//	metaData, err := toml.DecodeFile(path, &conf)
//	if err != nil {
//		return nil, toml.MetaData{}, err
//	}
//	conf.FillDefault(metaData)
//	return &conf, metaData, nil
//}
