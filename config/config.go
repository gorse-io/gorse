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
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/model/rank"
)

// Config is the configuration for the engine.
type Config struct {
	// database
	Database DatabaseConfig `toml:"database"`
	// strategies
	Similar       SimilarConfig `toml:"similar"`
	Latest        LatestConfig  `toml:"latest"`
	Popular       PopularConfig `toml:"popular"`
	Collaborative CFConfig      `toml:"collaborative"`
	Rank          RankConfig    `toml:"rank"`
	Subscribe     SubscribeConfig
	// nodes
	Master MasterConfig `toml:"master"`
	Server ServerConfig `toml:"server"`
}

func (config *Config) LoadDefaultIfNil() *Config {
	if config == nil {
		return &Config{
			Database:      *(*DatabaseConfig)(nil).LoadDefaultIfNil(),
			Similar:       *(*SimilarConfig)(nil).LoadDefaultIfNil(),
			Latest:        *(*LatestConfig)(nil).LoadDefaultIfNil(),
			Popular:       *(*PopularConfig)(nil).LoadDefaultIfNil(),
			Collaborative: *(*CFConfig)(nil).LoadDefaultIfNil(),
			Rank:          *(*RankConfig)(nil).LoadDefaultIfNil(),
			Master:        *(*MasterConfig)(nil).LoadDefaultIfNil(),
		}
	}
	return config
}

type SubscribeConfig struct {
	ImplicitSubscribe bool `toml:"implicit_subscribe"`
	NumTopLabels      int  `toml:"num_top_labels"`
}

type SimilarConfig struct {
	NumCache     int `toml:"n_cache"`
	UpdatePeriod int `toml:"update_period"`
}

func (c *SimilarConfig) LoadDefaultIfNil() *SimilarConfig {
	if c == nil {
		return &SimilarConfig{
			NumCache:     100,
			UpdatePeriod: 60,
		}
	}
	return c
}

type LatestConfig struct {
	NumCache     int `toml:"n_cache"`
	UpdatePeriod int `toml:"update_period"`
}

func (c *LatestConfig) LoadDefaultIfNil() *LatestConfig {
	if c == nil {
		return &LatestConfig{
			NumCache:     100,
			UpdatePeriod: 10,
		}
	}
	return c
}

type PopularConfig struct {
	NumCache     int `toml:"n_cache"`
	UpdatePeriod int `toml:"update_period"`
	TimeWindow   int `toml:"time_window"`
}

func (c *PopularConfig) LoadDefaultIfNil() *PopularConfig {
	if c == nil {
		return &PopularConfig{
			NumCache:     100,
			UpdatePeriod: 1440,
			TimeWindow:   365,
		}
	}
	return c
}

/* CFConfig is configuration for collaborative filtering model */
type CFConfig struct {
	NumCached     int    `toml:"n_cache"`
	CFModel       string `toml:"cf_model"`
	FitPeriod     int    `toml:"fit_period"`
	PredictPeriod int    `toml:"predict_period"`
	// Hyper-parameters
	Lr          float64 `toml:"lr"`           // learning rate
	Reg         float64 `toml:"reg"`          // regularization strength
	NEpochs     int     `toml:"n_epochs"`     // number of epochs
	NFactors    int     `toml:"n_factors"`    // number of factors
	RandomState int     `toml:"random_state"` // random state (seed)
	UseBias     bool    `toml:"use_bias"`     // use bias
	InitMean    float64 `toml:"init_mean"`    // mean of gaussian initial parameter
	InitStdDev  float64 `toml:"init_std"`     // standard deviation of gaussian initial parameter
	Alpha       float64 `toml:"alpha"`        // weight for negative samples in ALS/CCD
	// fit config
	Verbose      int `toml:"verbose"`      // verbose period
	Candidates   int `toml:"n_candidates"` // number of candidates for test
	TopK         int `toml:"top_k"`        // evaluate top k recommendations
	NumTestUsers int `toml:"n_test_users"` // number of users in test set
}

func (c *CFConfig) LoadDefaultIfNil() *CFConfig {
	if c == nil {
		return &CFConfig{
			NumCached:     800,
			CFModel:       "als",
			PredictPeriod: 60,
			FitPeriod:     1440,
			Verbose:       10,
			Candidates:    100,
			TopK:          10,
		}
	}
	return c
}

func (c *CFConfig) GetFitConfig(nJobs int) *cf.FitConfig {
	return &cf.FitConfig{
		Jobs:       nJobs,
		Verbose:    c.Verbose,
		Candidates: c.Candidates,
		TopK:       c.TopK,
	}
}

func (c *CFConfig) GetParams(metaData *toml.MetaData) model.Params {
	type ParamValues struct {
		name  string
		key   model.ParamName
		value interface{}
	}
	values := []ParamValues{
		{"lr", model.Lr, c.Lr},
		{"reg", model.Reg, c.Reg},
		{"n_epochs", model.NEpochs, c.NEpochs},
		{"n_factors", model.NFactors, c.NFactors},
		{"random_state", model.RandomState, c.RandomState},
		{"init_mean", model.InitMean, c.InitMean},
		{"init_std", model.InitStdDev, c.InitStdDev},
		{"alpha", model.Alpha, c.Alpha},
	}
	params := model.Params{}
	for _, v := range values {
		if metaData.IsDefined("cf", v.name) {
			params[v.key] = v.value
		}
	}
	return params
}

/* RankConfig is configuration for rank model */
type RankConfig struct {
	Task      string `toml:"task"`
	FitPeriod int    `toml:"fit_period"`
	// fit config
	Verbose    int     `toml:"verbose"`
	SplitRatio float32 `toml:"split_ratio"`
	// Hyper-parameters
	Lr          float64 `toml:"lr"`           // learning rate
	Reg         float64 `toml:"reg"`          // regularization strength
	NEpochs     int     `toml:"n_epochs"`     // number of epochs
	NFactors    int     `toml:"n_factors"`    // number of factors
	RandomState int     `toml:"random_state"` // random state (seed)
	UseBias     bool    `toml:"use_bias"`     // use bias
	InitMean    float64 `toml:"init_mean"`    // mean of gaussian initial parameter
	InitStdDev  float64 `toml:"init_std"`     // standard deviation of gaussian initial parameter
}

func (c *RankConfig) LoadDefaultIfNil() *RankConfig {
	if c == nil {
		return &RankConfig{
			FitPeriod: 60,
			Task:      "r",
			Verbose:   10,
		}
	}
	return c
}

func (c *RankConfig) GetFitConfig(nJobs int) *rank.FitConfig {
	return &rank.FitConfig{
		Jobs:    nJobs,
		Verbose: c.Verbose,
	}
}

func (c *RankConfig) GetParams(metaData *toml.MetaData) model.Params {
	type ParamValues struct {
		name  string
		key   model.ParamName
		value interface{}
	}
	values := []ParamValues{
		{"lr", model.Lr, c.Lr},
		{"reg", model.Reg, c.Reg},
		{"n_epochs", model.NEpochs, c.NEpochs},
		{"n_factors", model.NFactors, c.NFactors},
		{"random_state", model.RandomState, c.RandomState},
		{"init_mean", model.InitMean, c.InitMean},
		{"init_std", model.InitStdDev, c.InitStdDev},
	}
	params := model.Params{}
	for _, v := range values {
		if metaData.IsDefined("rank", v.name) {
			params[v.key] = v.value
		}
	}
	return params
}

// MasterConfig is the configuration for the master.
type MasterConfig struct {
	Port         int    `toml:"port"`
	Host         string `toml:"host"`
	Jobs         int    `toml:"n_jobs"`
	MetaTimeout  int    `toml:"cluster_meta_timeout"`
	RetryTimeout int    `toml:"retry_timeout"`
}

func (config *MasterConfig) LoadDefaultIfNil() *MasterConfig {
	if config == nil {
		return &MasterConfig{
			Port:        8086,
			Host:        "127.0.0.1",
			Jobs:        2,
			MetaTimeout: 60,
		}
	}
	return config
}

type ServerConfig struct {
	APIKey   string `toml:"api_key"`
	DefaultN int    `toml:"default_n"`
}

// DatabaseConfig is the configuration for the database.
type DatabaseConfig struct {
	DataStore         string   `toml:"data_store"`           // database for data store
	CacheStore        string   `toml:"cache_store"`          // database for cache store
	AutoInsertUser    bool     `toml:"auto_insert_user"`     // insert new users while inserting feedback
	AutoInsertItem    bool     `toml:"auto_insert_item"`     // insert new items while inserting feedback
	MatchFeedbackType []string `toml:"match_feedback_types"` // feedback type of matching
	RankFeedbackType  []string `toml:"rank_feedback_types"`  // feedback type of ranking
}

func (config *DatabaseConfig) LoadDefaultIfNil() *DatabaseConfig {
	if config == nil {
		return &DatabaseConfig{
			CacheStore:     "redis://127.0.0.1:6379",
			DataStore:      "mysql://root@tcp(127.0.0.1:3306)/gorse",
			AutoInsertUser: true,
			AutoInsertItem: true,
		}
	}
	return config
}

// FillDefault fill default values for missing values.
func (config *Config) FillDefault(meta toml.MetaData) {
	// Default database config
	defaultDBConfig := *(*DatabaseConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("database", "data_store") {
		config.Database.DataStore = defaultDBConfig.DataStore
	}
	if !meta.IsDefined("database", "cache_store") {
		config.Database.CacheStore = defaultDBConfig.CacheStore
	}
	if !meta.IsDefined("database", "auto_insert_user") {
		config.Database.AutoInsertUser = defaultDBConfig.AutoInsertUser
	}
	if !meta.IsDefined("database", "auto_insert_item") {
		config.Database.AutoInsertItem = defaultDBConfig.AutoInsertItem
	}
	// Default similar config
	defaultSimilarConfig := *(*SimilarConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("similar", "n_cache") {
		config.Similar.NumCache = defaultSimilarConfig.NumCache
	}
	if !meta.IsDefined("similar", "update_period") {
		config.Similar.UpdatePeriod = defaultSimilarConfig.UpdatePeriod
	}
	// Default latest config
	defaultLatestConfig := *(*LatestConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("latest", "n_cache") {
		config.Latest.NumCache = defaultLatestConfig.NumCache
	}
	if !meta.IsDefined("latest", "update_period") {
		config.Latest.UpdatePeriod = defaultLatestConfig.UpdatePeriod
	}
	// Default popular config
	defaultPopularConfig := *(*PopularConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("popular", "n_cache") {
		config.Popular.NumCache = defaultPopularConfig.NumCache
	}
	if !meta.IsDefined("popular", "update_period") {
		config.Popular.UpdatePeriod = defaultPopularConfig.UpdatePeriod
	}
	if !meta.IsDefined("popular", "time_window") {
		config.Popular.TimeWindow = defaultPopularConfig.TimeWindow
	}
	// default Collaborative config
	defaultCFConfig := *(*CFConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("collaborative", "n_cache") {
		config.Collaborative.NumCached = defaultCFConfig.NumCached
	}
	if !meta.IsDefined("collaborative", "cf_model") {
		config.Collaborative.CFModel = defaultCFConfig.CFModel
	}
	if !meta.IsDefined("collaborative", "predict_period") {
		config.Collaborative.PredictPeriod = defaultCFConfig.PredictPeriod
	}
	if !meta.IsDefined("collaborative", "fit_period") {
		config.Collaborative.FitPeriod = defaultCFConfig.FitPeriod
	}
	if !meta.IsDefined("collaborative", "verbose") {
		config.Collaborative.Verbose = defaultCFConfig.Verbose
	}
	if !meta.IsDefined("collaborative", "n_candidates") {
		config.Collaborative.Candidates = defaultCFConfig.Candidates
	}
	if !meta.IsDefined("collaborative", "top_k") {
		config.Collaborative.TopK = defaultCFConfig.TopK
	}
	if !meta.IsDefined("collaborative", "n_test_users") {
		config.Collaborative.NumTestUsers = defaultCFConfig.NumTestUsers
	}
	// default rank config
	defaultRankConfig := *(*RankConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("rank", "fit_period") {
		config.Rank.FitPeriod = defaultRankConfig.FitPeriod
	}
	if !meta.IsDefined("rank", "task") {
		config.Rank.Task = defaultRankConfig.Task
	}
	if !meta.IsDefined("rank", "verbose") {
		config.Rank.Verbose = defaultRankConfig.Verbose
	}
	// Default master config
	defaultMasterConfig := *(*MasterConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("master", "port") {
		config.Master.Port = defaultMasterConfig.Port
	}
	if !meta.IsDefined("master", "host") {
		config.Master.Host = defaultMasterConfig.Host
	}
	if !meta.IsDefined("master", "n_jobs") {
		config.Master.Jobs = defaultMasterConfig.Jobs
	}
	if !meta.IsDefined("master", "cluster_meta_timeout") {
		config.Master.MetaTimeout = defaultMasterConfig.MetaTimeout
	}
}

// LoadConfig loads configuration from toml file.
func LoadConfig(path string) (*Config, *toml.MetaData, error) {
	var conf Config
	metaData, err := toml.DecodeFile(path, &conf)
	if err != nil {
		return nil, nil, err
	}
	conf.FillDefault(metaData)
	return &conf, &metaData, nil
}
