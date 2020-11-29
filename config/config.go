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
	Server   ServerConfig   `toml:"server"`
	Database DatabaseConfig `toml:"database"`
	Model    ModelConfig    `toml:"model"`
}

func (config *Config) LoadDefaultIfNil() *Config {
	if config == nil {
		return &Config{
			Server:   *(*ServerConfig)(nil).LoadDefaultIfNil(),
			Database: *(*DatabaseConfig)(nil).LoadDefaultIfNil(),
			Model:    *(*ModelConfig)(nil).LoadDefaultIfNil(),
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

type ModelConfig struct {
	Params ParamsConfig `toml:"params"`
	Fit    FitConfig    `toml:"fit"`
}

func (config *ModelConfig) LoadDefaultIfNil() *ModelConfig {
	if config == nil {
		return &ModelConfig{
			Params: ParamsConfig{},
			Fit:    *(*FitConfig)(nil).LoadDefaultIfNil(),
		}
	}
	return config
}

type FitConfig struct {
	Jobs       int `toml:"n_jobs"`
	Verbose    int `toml:"verbose"`
	Candidates int `toml:"n_candidates"`
	TopK       int `toml:"top_k"`
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
	// Default fit config
	defaultFitConfig := *(*FitConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("model", "fit", "n_jobs") {
		config.Model.Fit.Jobs = defaultFitConfig.Jobs
	}
	if !meta.IsDefined("model", "fit", "verbose") {
		config.Model.Fit.Verbose = defaultFitConfig.Verbose
	}
	if !meta.IsDefined("model", "fit", "n_candidates") {
		config.Model.Fit.Candidates = defaultFitConfig.Candidates
	}
	if !meta.IsDefined("model", "fit", "top_k") {
		config.Model.Fit.TopK = defaultFitConfig.TopK
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
