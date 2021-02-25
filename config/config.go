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
)

// Config is the configuration for the engine.
type Config struct {
	Common   CommonConfig   `toml:"common"`
	Online   OnlineConfig   `toml:"online"`
	Server   ServerConfig   `toml:"server"`
	Database DatabaseConfig `toml:"database"`
	Master   MasterConfig   `toml:"master"`
	Worker   WorkerConfig   `toml:"worker"`
}

func (config *Config) LoadDefaultIfNil() *Config {
	if config == nil {
		return &Config{
			Common:   *(*CommonConfig)(nil).LoadDefaultIfNil(),
			Server:   *(*ServerConfig)(nil).LoadDefaultIfNil(),
			Database: *(*DatabaseConfig)(nil).LoadDefaultIfNil(),
			Master:   *(*MasterConfig)(nil).LoadDefaultIfNil(),
			Worker:   *(*WorkerConfig)(nil).LoadDefaultIfNil(),
		}
	}
	return config
}

type OnlineConfig struct {
	NegativeValue float32 `toml:"negative_value"`

	NumPopular int `toml:"num_popular"`
	NumLatest  int `toml:"num_latest"`
	NumMatch   int `toml:"num_match"`
}

type CommonConfig struct {
	// insert new users while inserting feedback
	AutoInsertUser bool `toml:"auto_insert_user"`
	// insert new items while inserting feedback
	AutoInsertItem bool `toml:"auto_insert_item"`
	// cluster meta timeout (second)
	ClusterMetaTimeout int `toml:"cluster_meta_timeout"`

	MatchFeedbackType string `toml:"match_feedback_type"`
	RankFeedbackType  string

	RetryInterval int `toml:"retry_interval"`
	RetryLimit    int `toml:"retry_limit"`
	CacheSize     int `toml:"cache_size"`
}

func (config *CommonConfig) LoadDefaultIfNil() *CommonConfig {
	if config == nil {
		return &CommonConfig{
			RetryInterval: 1,
			RetryLimit:    10,
			CacheSize:     1000,
		}
	}
	return config
}

// ServerConfig is the configuration for the cmd.
type ServerConfig struct {
	DefaultReturnNumber int `toml:"default_n"`
}

func (config *ServerConfig) LoadDefaultIfNil() *ServerConfig {
	if config == nil {
		return &ServerConfig{
			DefaultReturnNumber: 100,
		}
	}
	return config
}

type WorkerConfig struct {
	LeaderAddr      string `toml:"leader_addr"`
	Host            string `toml:"host"`
	GossipPort      int    `toml:"gossip_port"`
	RPCPort         int    `toml:"rpc_port"`
	PredictInterval int    `toml:"predict_interval"`
	GossipInterval  int    `toml:"gossip_interval"`
}

func (config *WorkerConfig) LoadDefaultIfNil() *WorkerConfig {
	if config == nil {
		return &WorkerConfig{
			LeaderAddr:      "127.0.0.1:6384",
			Host:            "127.0.0.1",
			GossipPort:      6385,
			RPCPort:         6386,
			PredictInterval: 1,
			GossipInterval:  1,
		}
	}
	return config
}

type MasterConfig struct {
	Model             string       `toml:"model"`
	FitInterval       int          `toml:"fit_interval"`
	BroadcastInterval int          `toml:"broadcast_interval"`
	Params            ParamsConfig `toml:"params"`
	Fit               FitConfig    `toml:"fit"`
	Port              int          `toml:"port"`
	Host              string       `toml:"host"`
}

func (config *MasterConfig) LoadDefaultIfNil() *MasterConfig {
	if config == nil {
		return &MasterConfig{
			Model:             "als",
			FitInterval:       1,
			BroadcastInterval: 1,
			Params:            ParamsConfig{},
			Fit:               *(*FitConfig)(nil).LoadDefaultIfNil(),
			Port:              6384,
			Host:              "127.0.0.1",
		}
	}
	return config
}

type FitConfig struct {
	Jobs         int `toml:"n_jobs"`
	Verbose      int `toml:"verbose"`
	Candidates   int `toml:"n_candidates"`
	TopK         int `toml:"top_k"`
	NumTestUsers int `toml:"num_test_users"`
}

func (config *FitConfig) LoadDefaultIfNil() *FitConfig {
	if config == nil {
		return &FitConfig{
			Jobs:       1,
			Verbose:    10,
			Candidates: 100,
			TopK:       10,
		}
	}
	return config
}

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
	Weight      float64 `toml:"weight"`       // weight for negative samples in ALS
}

// DatabaseConfig is the configuration for the database.
type DatabaseConfig struct {
	DataStore  string `toml:"data_store"`
	CacheStore string `toml:"cache_store"`
}

func (config *DatabaseConfig) LoadDefaultIfNil() *DatabaseConfig {
	if config == nil {
		return &DatabaseConfig{
			DataStore: "badger://~/.gorse/data/",
		}
	}
	return config
}

// FillDefault fill default values for missing values.
func (config *Config) FillDefault(meta toml.MetaData) {
	// Default common
	defaultCommonConfig := *(*CommonConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("common", "retry_interval") {
		config.Common.RetryInterval = defaultCommonConfig.RetryInterval
	}
	if !meta.IsDefined("common", "retry_limit") {
		config.Common.RetryLimit = defaultCommonConfig.RetryLimit
	}
	if !meta.IsDefined("common", "cache_size") {
		config.Common.CacheSize = defaultCommonConfig.CacheSize
	}
	// Default server config
	defaultServerConfig := *(*ServerConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("server", "default_n") {
		config.Server.DefaultReturnNumber = defaultServerConfig.DefaultReturnNumber
	}
	// Default database config
	defaultDBConfig := *(*DatabaseConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("database", "data_store") {
		config.Database.DataStore = defaultDBConfig.DataStore
	}
	// Default master config
	defaultFitConfig := *(*FitConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("master", "fit", "n_jobs") {
		config.Master.Fit.Jobs = defaultFitConfig.Jobs
	}
	if !meta.IsDefined("master", "fit", "verbose") {
		config.Master.Fit.Verbose = defaultFitConfig.Verbose
	}
	if !meta.IsDefined("master", "fit", "n_candidates") {
		config.Master.Fit.Candidates = defaultFitConfig.Candidates
	}
	if !meta.IsDefined("master", "fit", "top_k") {
		config.Master.Fit.TopK = defaultFitConfig.TopK
	}
	defaultLeaderConfig := *(*MasterConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("master", "model") {
		config.Master.Model = defaultLeaderConfig.Model
	}
	if !meta.IsDefined("leader", "fit_interval") {
		config.Master.FitInterval = defaultLeaderConfig.FitInterval
	}
	if !meta.IsDefined("leader", "broadcast_interval") {
		config.Master.BroadcastInterval = defaultLeaderConfig.BroadcastInterval
	}
	if !meta.IsDefined("master", "port") {
		config.Master.Port = defaultLeaderConfig.Port
	}
	if !meta.IsDefined("master", "host") {
		config.Master.Host = defaultLeaderConfig.Host
	}
	// Default worker config
	defaultWorkerConfig := *(*WorkerConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("worker", "leader_addr") {
		config.Worker.LeaderAddr = defaultWorkerConfig.LeaderAddr
	}
	if !meta.IsDefined("worker", "host") {
		config.Worker.Host = defaultWorkerConfig.Host
	}
	if !meta.IsDefined("worker", "rpc_port") {
		config.Worker.RPCPort = defaultWorkerConfig.RPCPort
	}
	if !meta.IsDefined("worker", "predict_interval") {
		config.Worker.PredictInterval = defaultWorkerConfig.PredictInterval
	}
	if !meta.IsDefined("worker", "gossip_interval") {
		config.Worker.GossipInterval = defaultWorkerConfig.GossipInterval
	}
	if !meta.IsDefined("worker", "gossip_port") {
		config.Worker.GossipPort = defaultWorkerConfig.GossipPort
	}
	// Default online config
	if !meta.IsDefined("online", "num_latest") {
		config.Online.NumLatest = 250
	}
	if !meta.IsDefined("online", "num_popular") {
		config.Online.NumPopular = 250
	}
	if !meta.IsDefined("online", "num_match") {
		config.Online.NumMatch = 500
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
