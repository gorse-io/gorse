// Copyright 2021 gorse Project Authors
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
	"sync"
)

const (
	NeighborTypeAuto    = "auto"
	NeighborTypeSimilar = "similar"
	NeighborTypeRelated = "related"
)

// Config is the configuration for the engine.
type Config struct {
	Database  DatabaseConfig  `toml:"database"`
	Master    MasterConfig    `toml:"master"`
	Server    ServerConfig    `toml:"server"`
	Recommend RecommendConfig `toml:"recommend"`
}

// LoadDefaultIfNil loads default settings if config is nil.
func (config *Config) LoadDefaultIfNil() *Config {
	if config == nil {
		return &Config{
			Database:  *(*DatabaseConfig)(nil).LoadDefaultIfNil(),
			Master:    *(*MasterConfig)(nil).LoadDefaultIfNil(),
			Server:    *(*ServerConfig)(nil).LoadDefaultIfNil(),
			Recommend: *(*RecommendConfig)(nil).LoadDefaultIfNil(),
		}
	}
	return config
}

// validate Config.
func (config *Config) validate() {
	config.Database.validate()
	config.Master.validate()
	config.Server.validate()
	config.Recommend.validate()
}

// DatabaseConfig is the configuration for the database.
type DatabaseConfig struct {
	DataStore            string   `toml:"data_store"`              // database for data store
	CacheStore           string   `toml:"cache_store"`             // database for cache store
	AutoInsertUser       bool     `toml:"auto_insert_user"`        // insert new users while inserting feedback
	AutoInsertItem       bool     `toml:"auto_insert_item"`        // insert new items while inserting feedback
	CacheSize            int      `toml:"cache_size"`              // cache size for recommended/popular/latest items
	PositiveFeedbackType []string `toml:"positive_feedback_types"` // positive feedback type
	ReadFeedbackTypes    []string `toml:"read_feedback_types"`     // feedback type for read event
	PositiveFeedbackTTL  uint     `toml:"positive_feedback_ttl"`   // time-to-live of positive feedbacks
	ItemTTL              uint     `toml:"item_ttl"`                // item-to-live of items
}

// LoadDefaultIfNil loads default settings if config is nil.
func (config *DatabaseConfig) LoadDefaultIfNil() *DatabaseConfig {
	if config == nil {
		return &DatabaseConfig{
			AutoInsertUser: true,
			AutoInsertItem: true,
			CacheSize:      100,
		}
	}
	return config
}

// validate MasterConfig.
func (config *DatabaseConfig) validate() {
	validatePositive("cache_size", config.CacheSize)
	validateNotEmpty("positive_feedback_types", config.PositiveFeedbackType)
	validateNotEmpty("read_feedback_types", config.ReadFeedbackTypes)
}

// MasterConfig is the configuration for the master.
type MasterConfig struct {
	Port        int    `toml:"port"`         // master port
	Host        string `toml:"host"`         // master host
	HttpPort    int    `toml:"http_port"`    // HTTP port
	HttpHost    string `toml:"http_host"`    // HTTP host
	NumJobs     int    `toml:"n_jobs"`       // number of working jobs
	MetaTimeout int    `toml:"meta_timeout"` // cluster meta timeout (second)
}

// LoadDefaultIfNil loads default settings if config is nil.
func (config *MasterConfig) LoadDefaultIfNil() *MasterConfig {
	if config == nil {
		return &MasterConfig{
			Port:        8086,
			Host:        "127.0.0.1",
			HttpPort:    8088,
			HttpHost:    "127.0.0.1",
			NumJobs:     1,
			MetaTimeout: 60,
		}
	}
	return config
}

// validate MasterConfig.
func (config *MasterConfig) validate() {
	validatePositive("n_jobs", config.NumJobs)
	validatePositive("meta_timeout", config.MetaTimeout)
}

// RecommendConfig is the configuration of recommendation setup.
type RecommendConfig struct {
	PopularWindow                int                     `toml:"popular_window"`
	FitPeriod                    int                     `toml:"fit_period"`
	SearchPeriod                 int                     `toml:"search_period"`
	SearchEpoch                  int                     `toml:"search_epoch"`
	SearchTrials                 int                     `toml:"search_trials"`
	RefreshRecommendPeriod       int                     `toml:"refresh_recommend_period"`
	FallbackRecommend            []string                `toml:"fallback_recommend"`
	ExploreRecommend             ExploreRecommendWrapper `toml:"explore"`
	ItemNeighborType             string                  `toml:"item_neighbor_type"`
	UserNeighborType             string                  `toml:"user_neighbor_type"`
	EnableLatestRecommend        bool                    `toml:"enable_latest_recommend"`
	EnablePopularRecommend       bool                    `toml:"enable_popular_recommend"`
	EnableUserBasedRecommend     bool                    `toml:"enable_user_based_recommend"`
	EnableItemBasedRecommend     bool                    `toml:"enable_item_based_recommend"`
	EnableColRecommend           bool                    `toml:"enable_collaborative_recommend"`
	EnableClickThroughPrediction bool                    `toml:"enable_click_through_prediction"`
}

type ExploreRecommendWrapper struct {
	Recommend map[string]float64 `toml:"recommend"`
	Lock      sync.Mutex
}

func (config *RecommendConfig) GetExploreRecommend(key string) (value float64, exist bool) {
	if config == nil || len(config.ExploreRecommend.Recommend) == 0 {
		return 0.0, false
	}
	config.ExploreRecommend.Lock.Lock()
	defer config.ExploreRecommend.Lock.Unlock()
	value, exist = config.ExploreRecommend.Recommend[key]
	return
}

// LoadDefaultIfNil loads default settings if config is nil.
func (config *RecommendConfig) LoadDefaultIfNil() *RecommendConfig {
	if config == nil {
		return &RecommendConfig{
			PopularWindow:                180,
			FitPeriod:                    60,
			SearchPeriod:                 180,
			SearchEpoch:                  100,
			SearchTrials:                 10,
			RefreshRecommendPeriod:       5,
			FallbackRecommend:            []string{"latest"},
			ItemNeighborType:             "auto",
			UserNeighborType:             "auto",
			EnableLatestRecommend:        false,
			EnablePopularRecommend:       false,
			EnableUserBasedRecommend:     false,
			EnableItemBasedRecommend:     false,
			EnableColRecommend:           true,
			EnableClickThroughPrediction: false,
			ExploreRecommend: ExploreRecommendWrapper{
				Recommend: make(map[string]float64, 0),
				Lock:      sync.Mutex{},
			},
		}
	}
	return config
}

// validate RecommendConfig.
func (config *RecommendConfig) validate() {
	validateNotNegative("popular_window", config.PopularWindow)
	validatePositive("fit_period", config.FitPeriod)
	validatePositive("search_period", config.SearchPeriod)
	validatePositive("search_epoch", config.SearchEpoch)
	validatePositive("search_trials", config.SearchTrials)
	validatePositive("refresh_recommend_period", config.RefreshRecommendPeriod)
	validateSubset("fallback_recommend", config.FallbackRecommend, []string{"item_based", "popular", "latest"})
	validateIn("item_neighbor_type", config.ItemNeighborType, []string{"similar", "related", "auto"})
	validateIn("user_neighbor_type", config.UserNeighborType, []string{"similar", "related", "auto"})
}

// ServerConfig is the configuration for the server.
type ServerConfig struct {
	APIKey   string `toml:"api_key"`   // default number of returned items
	DefaultN int    `toml:"default_n"` // secret key for RESTful APIs (SSL required)
}

// LoadDefaultIfNil loads default settings if config is nil.
func (config *ServerConfig) LoadDefaultIfNil() *ServerConfig {
	if config == nil {
		return &ServerConfig{
			DefaultN: 10,
		}
	}
	return config
}

// validate ServerConfig.
func (config *ServerConfig) validate() {
	validatePositive("default_n", config.DefaultN)
}

// FillDefault fill default values for missing values.
func (config *Config) FillDefault(meta toml.MetaData) {
	// Default database config
	defaultDBConfig := *(*DatabaseConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("database", "auto_insert_user") {
		config.Database.AutoInsertUser = defaultDBConfig.AutoInsertUser
	}
	if !meta.IsDefined("database", "auto_insert_item") {
		config.Database.AutoInsertItem = defaultDBConfig.AutoInsertItem
	}
	if !meta.IsDefined("database", "cache_size") {
		config.Database.CacheSize = defaultDBConfig.CacheSize
	}
	// Default master config
	defaultMasterConfig := *(*MasterConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("master", "port") {
		config.Master.Port = defaultMasterConfig.Port
	}
	if !meta.IsDefined("master", "host") {
		config.Master.Host = defaultMasterConfig.Host
	}
	if !meta.IsDefined("master", "http_port") {
		config.Master.HttpPort = defaultMasterConfig.HttpPort
	}
	if !meta.IsDefined("master", "http_host") {
		config.Master.HttpHost = defaultMasterConfig.HttpHost
	}
	if !meta.IsDefined("master", "n_jobs") {
		config.Master.NumJobs = defaultMasterConfig.NumJobs
	}
	if !meta.IsDefined("master", "meta_timeout") {
		config.Master.MetaTimeout = defaultMasterConfig.MetaTimeout
	}
	// Default server config
	defaultServerConfig := *(*ServerConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("server", "api_key") {
		config.Server.APIKey = defaultServerConfig.APIKey
	}
	if !meta.IsDefined("server", "default_n") {
		config.Server.DefaultN = defaultServerConfig.DefaultN
	}
	// Default recommend config
	defaultRecommendConfig := *(*RecommendConfig)(nil).LoadDefaultIfNil()
	if !meta.IsDefined("recommend", "popular_window") {
		config.Recommend.PopularWindow = defaultRecommendConfig.PopularWindow
	}
	if !meta.IsDefined("recommend", "fit_period") {
		config.Recommend.FitPeriod = defaultRecommendConfig.FitPeriod
	}
	if !meta.IsDefined("recommend", "search_period") {
		config.Recommend.SearchPeriod = defaultRecommendConfig.SearchPeriod
	}
	if !meta.IsDefined("recommend", "search_epoch") {
		config.Recommend.SearchEpoch = defaultRecommendConfig.SearchEpoch
	}
	if !meta.IsDefined("recommend", "search_trials") {
		config.Recommend.SearchTrials = defaultRecommendConfig.SearchTrials
	}
	if !meta.IsDefined("recommend", "refresh_recommend_period") {
		config.Recommend.RefreshRecommendPeriod = defaultRecommendConfig.RefreshRecommendPeriod
	}
	if !meta.IsDefined("recommend", "fallback_recommend") {
		config.Recommend.FallbackRecommend = defaultRecommendConfig.FallbackRecommend
	}
	if !meta.IsDefined("recommend", "item_neighbor_type") {
		config.Recommend.ItemNeighborType = defaultRecommendConfig.ItemNeighborType
	}
	if !meta.IsDefined("recommend", "user_neighbor_type") {
		config.Recommend.UserNeighborType = defaultRecommendConfig.UserNeighborType
	}
	if !meta.IsDefined("recommend", "enable_latest_recommend") {
		config.Recommend.EnableLatestRecommend = defaultRecommendConfig.EnableLatestRecommend
	}
	if !meta.IsDefined("recommend", "enable_popular_recommend") {
		config.Recommend.EnablePopularRecommend = defaultRecommendConfig.EnablePopularRecommend
	}
	if !meta.IsDefined("recommend", "enable_user_based_recommend") {
		config.Recommend.EnableUserBasedRecommend = defaultRecommendConfig.EnableUserBasedRecommend
	}
	if !meta.IsDefined("recommend", "enable_item_based_recommend") {
		config.Recommend.EnableItemBasedRecommend = defaultRecommendConfig.EnableItemBasedRecommend
	}
	if !meta.IsDefined("recommend", "enable_collaborative_recommend") {
		config.Recommend.EnableColRecommend = defaultRecommendConfig.EnableColRecommend
	}
	if !meta.IsDefined("recommend", "enable_click_through_prediction") {
		config.Recommend.EnableClickThroughPrediction = defaultRecommendConfig.EnableClickThroughPrediction
	}
	if !meta.IsDefined("recommend", "explore") {
		config.Recommend.ExploreRecommend = ExploreRecommendWrapper{
			Recommend: make(map[string]float64, 0),
			Lock:      sync.Mutex{},
		}
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
	conf.validate()
	return &conf, &metaData, nil
}
