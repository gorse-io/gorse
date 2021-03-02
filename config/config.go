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
	Similar SimilarConfig `toml:"similar"`
	Latest  LatestConfig  `toml:"latest"`
	Popular PopularConfig `toml:"popular"`
	CF      CFConfig      `toml:"cf"`
	Rank    RankConfig    `toml:"rank"`
	// nodes
	Master MasterConfig `toml:"master"`
}

func (config *Config) LoadDefaultIfNil() *Config {
	if config == nil {
		return &Config{
			Database: *(*DatabaseConfig)(nil).LoadDefaultIfNil(),
			Similar:  *(*SimilarConfig)(nil).LoadDefaultIfNil(),
			Latest:   *(*LatestConfig)(nil).LoadDefaultIfNil(),
			Popular:  *(*PopularConfig)(nil).LoadDefaultIfNil(),
			CF:       *(*CFConfig)(nil).LoadDefaultIfNil(),
			Rank:     *(*RankConfig)(nil).LoadDefaultIfNil(),
			Master:   *(*MasterConfig)(nil).LoadDefaultIfNil(),
		}
	}
	return config
}

type SimilarConfig struct {
	NumSimilar   int `toml:"n_similar"`
	UpdatePeriod int `toml:"update_period"`
}

func (c *SimilarConfig) LoadDefaultIfNil() *SimilarConfig {
	if c == nil {
		return &SimilarConfig{
			NumSimilar:   100,
			UpdatePeriod: 60,
		}
	}
	return c
}

type LatestConfig struct {
	NumLatest    int `toml:"n_latest"`
	UpdatePeriod int `toml:"update_period"`
}

func (c *LatestConfig) LoadDefaultIfNil() *LatestConfig {
	if c == nil {
		return &LatestConfig{
			NumLatest:    100,
			UpdatePeriod: 10,
		}
	}
	return c
}

type PopularConfig struct {
	NumPopular   int `toml:"n_popular"`
	UpdatePeriod int `toml:"update_period"`
	TimeWindow   int `toml:"time_window"`
}

func (c *PopularConfig) LoadDefaultIfNil() *PopularConfig {
	if c == nil {
		return &PopularConfig{
			NumPopular:   100,
			UpdatePeriod: 1440,
			TimeWindow:   365,
		}
	}
	return c
}

/* CFConfig is configuration for collaborative filtering model */
type CFConfig struct {
	NumCF         int      `toml:"n_cf"`
	CFModel       string   `toml:"cf_model"`
	FitPeriod     int      `toml:"fit_period"`
	PredictPeriod int      `toml:"predict_period"`
	FeedbackTypes []string `toml:"feedback_types"`
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
	FitJobs      int `toml:"fit_jobs"`     // number of fit jobs
	Verbose      int `toml:"verbose"`      // verbose period
	Candidates   int `toml:"n_candidates"` // number of candidates for test
	TopK         int `toml:"top_k"`        // evaluate top k recommendations
	NumTestUsers int `toml:"n_test_users"` // number of users in test set
}

func (c *CFConfig) LoadDefaultIfNil() *CFConfig {
	if c == nil {
		return &CFConfig{
			FeedbackTypes: []string{""},
			NumCF:         800,
			CFModel:       "als",
			PredictPeriod: 60,
			FitPeriod:     1440,
			FitJobs:       1,
			Verbose:       10,
			Candidates:    100,
			TopK:          10,
		}
	}
	return c
}

func (c *CFConfig) GetFitConfig() *cf.FitConfig {
	return &cf.FitConfig{
		Jobs:       c.FitJobs,
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
	Task          string   `toml:"task"`
	FeedbackTypes []string `toml:"feedback_types"`
	FitPeriod     int      `toml:"fit_period"`
	// fit config
	FitJobs int `toml:"fit_jobs"`
	Verbose int `toml:"verbose"`
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
			FeedbackTypes: []string{""},
			FitPeriod:     60,
			Task:          "r",
			FitJobs:       1,
			Verbose:       10,
		}
	}
	return c
}

func (c *RankConfig) GetFitConfig() *rank.FitConfig {
	return &rank.FitConfig{
		Jobs:    c.FitJobs,
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
	Port               int    `toml:"port"`
	Host               string `toml:"host"`
	Jobs               int    `toml:"jobs"`
	ClusterMetaTimeout int    `toml:"cluster_meta_timeout"`
}

func (config *MasterConfig) LoadDefaultIfNil() *MasterConfig {
	if config == nil {
		return &MasterConfig{
			Port:               8086,
			Host:               "127.0.0.1",
			Jobs:               2,
			ClusterMetaTimeout: 60,
		}
	}
	return config
}

// DatabaseConfig is the configuration for the database.
type DatabaseConfig struct {
	// database for data store
	DataStore string `toml:"data_store"`
	// database for cache store
	CacheStore string `toml:"cache_store"`
	// insert new users while inserting feedback
	AutoInsertUser bool `toml:"auto_insert_user"`
	// insert new items while inserting feedback
	AutoInsertItem bool `toml:"auto_insert_item"`
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
	if !meta.IsDefined("similar", "n_similar") {
		config.Similar.NumSimilar = defaultSimilarConfig.NumSimilar
	}
	if !meta.IsDefined("similar", "update_period") {
		config.Similar.UpdatePeriod = defaultSimilarConfig.UpdatePeriod
	}
	// Default latest config
	defaultLatestConfig := *(*LatestConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("latest", "n_latest") {
		config.Latest.NumLatest = defaultLatestConfig.NumLatest
	}
	if !meta.IsDefined("latest", "update_period") {
		config.Latest.UpdatePeriod = defaultLatestConfig.UpdatePeriod
	}
	// Default popular config
	defaultPopularConfig := *(*PopularConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("popular", "n_popular") {
		config.Popular.NumPopular = defaultPopularConfig.NumPopular
	}
	if !meta.IsDefined("popular", "update_period") {
		config.Popular.UpdatePeriod = defaultPopularConfig.UpdatePeriod
	}
	if !meta.IsDefined("popular", "time_window") {
		config.Popular.TimeWindow = defaultPopularConfig.TimeWindow
	}
	// default CF config
	defaultCFConfig := *(*CFConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("cf", "feedback_type") {
		config.CF.FeedbackTypes = defaultCFConfig.FeedbackTypes
	}
	if !meta.IsDefined("cf", "n_cf") {
		config.CF.NumCF = defaultCFConfig.NumCF
	}
	if !meta.IsDefined("cf", "cf_model") {
		config.CF.CFModel = defaultCFConfig.CFModel
	}
	if !meta.IsDefined("cf", "predict_period") {
		config.CF.PredictPeriod = defaultCFConfig.PredictPeriod
	}
	if !meta.IsDefined("cf", "fit_period") {
		config.CF.FitPeriod = defaultCFConfig.FitPeriod
	}
	if !meta.IsDefined("cf", "fit_jobs") {
		config.CF.FitJobs = defaultCFConfig.FitJobs
	}
	if !meta.IsDefined("cf", "verbose") {
		config.CF.Verbose = defaultCFConfig.Verbose
	}
	if !meta.IsDefined("cf", "n_candidates") {
		config.CF.Candidates = defaultCFConfig.Candidates
	}
	if !meta.IsDefined("cf", "top_k") {
		config.CF.TopK = defaultCFConfig.TopK
	}
	if !meta.IsDefined("cf", "n_test_users") {
		config.CF.NumTestUsers = defaultCFConfig.NumTestUsers
	}
	// default rank config
	defaultRankConfig := *(*RankConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("rank", "feedback_type") {
		config.Rank.FeedbackTypes = defaultRankConfig.FeedbackTypes
	}
	if !meta.IsDefined("rank", "fit_period") {
		config.Rank.FitPeriod = defaultRankConfig.FitPeriod
	}
	if !meta.IsDefined("rank", "task") {
		config.Rank.Task = defaultRankConfig.Task
	}
	if !meta.IsDefined("rank", "fit_jobs") {
		config.Rank.FitJobs = defaultRankConfig.FitJobs
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
	if !meta.IsDefined("master", "jobs") {
		config.Master.Jobs = defaultMasterConfig.Jobs
	}
	if !meta.IsDefined("master", "cluster_meta_timeout") {
		config.Master.ClusterMetaTimeout = defaultMasterConfig.ClusterMetaTimeout
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
