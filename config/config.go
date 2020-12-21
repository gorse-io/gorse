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
	Server   ServerConfig   `toml:"server"`
	Database DatabaseConfig `toml:"database"`
	Leader   LeaderConfig   `toml:"leader"`
	Worker   WorkerConfig   `toml:"worker"`
}

func (config *Config) LoadDefaultIfNil() *Config {
	if config == nil {
		return &Config{
			Common:   *(*CommonConfig)(nil).LoadDefaultIfNil(),
			Server:   *(*ServerConfig)(nil).LoadDefaultIfNil(),
			Database: *(*DatabaseConfig)(nil).LoadDefaultIfNil(),
			Leader:   *(*LeaderConfig)(nil).LoadDefaultIfNil(),
			Worker:   *(*WorkerConfig)(nil).LoadDefaultIfNil(),
		}
	}
	return config
}

type CommonConfig struct {
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
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	DefaultN int    `toml:"default_n"`
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

type LeaderConfig struct {
	Model             string       `toml:"model"`
	FitInterval       int          `toml:"fit_interval"`
	BroadcastInterval int          `toml:"broadcast_interval"`
	Params            ParamsConfig `toml:"params"`
	Fit               FitConfig    `toml:"fit"`
	GossipPort        int          `toml:"gossip_port"`
	Host              string       `toml:"host"`
}

func (config *LeaderConfig) LoadDefaultIfNil() *LeaderConfig {
	if config == nil {
		return &LeaderConfig{
			Model:             "als",
			FitInterval:       1,
			BroadcastInterval: 1,
			Params:            ParamsConfig{},
			Fit:               *(*FitConfig)(nil).LoadDefaultIfNil(),
			GossipPort:        6384,
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
	Path string `toml:"path"`
}

func (config *DatabaseConfig) LoadDefaultIfNil() *DatabaseConfig {
	if config == nil {
		return &DatabaseConfig{
			Path: "badger://~/.gorse/data/",
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
	if !meta.IsDefined("server", "host") {
		config.Server.Host = defaultServerConfig.Host
	}
	if !meta.IsDefined("server", "port") {
		config.Server.Port = defaultServerConfig.Port
	}
	if !meta.IsDefined("server", "default_n") {
		config.Server.DefaultN = defaultServerConfig.DefaultN
	}
	// Default database config
	defaultDBConfig := *(*DatabaseConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("database", "path") {
		config.Database.Path = defaultDBConfig.Path
	}
	// Default leader config
	defaultFitConfig := *(*FitConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("leader", "fit", "n_jobs") {
		config.Leader.Fit.Jobs = defaultFitConfig.Jobs
	}
	if !meta.IsDefined("leader", "fit", "verbose") {
		config.Leader.Fit.Verbose = defaultFitConfig.Verbose
	}
	if !meta.IsDefined("leader", "fit", "n_candidates") {
		config.Leader.Fit.Candidates = defaultFitConfig.Candidates
	}
	if !meta.IsDefined("leader", "fit", "top_k") {
		config.Leader.Fit.TopK = defaultFitConfig.TopK
	}
	defaultLeaderConfig := *(*LeaderConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("leader", "model") {
		config.Leader.Model = defaultLeaderConfig.Model
	}
	if !meta.IsDefined("leader", "fit_interval") {
		config.Leader.FitInterval = defaultLeaderConfig.FitInterval
	}
	if !meta.IsDefined("leader", "broadcast_interval") {
		config.Leader.BroadcastInterval = defaultLeaderConfig.BroadcastInterval
	}
	if !meta.IsDefined("leader", "gossip_port") {
		config.Leader.GossipPort = defaultLeaderConfig.GossipPort
	}
	if !meta.IsDefined("leader", "host") {
		config.Leader.Host = defaultLeaderConfig.Host
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
