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
	"github.com/juju/errors"
	"github.com/spf13/viper"
	"github.com/zhenghaoz/gorse/base"
	"go.uber.org/zap"
	"sync"
)

const (
	NeighborTypeAuto    = "auto"
	NeighborTypeSimilar = "similar"
	NeighborTypeRelated = "related"
)

// Config is the configuration for the engine.
type Config struct {
	Database  DatabaseConfig  `toml:"database" mapstructure:"database"`
	Master    MasterConfig    `toml:"master" mapstructure:"master"`
	Server    ServerConfig    `toml:"server" mapstructure:"server"`
	Recommend RecommendConfig `toml:"recommend" mapstructure:"recommend"`
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
	DataStore            string   `toml:"data_store" mapstructure:"data_store"`                           // database for data store
	CacheStore           string   `toml:"cache_store" mapstructure:"cache_store"`                         // database for cache store
	AutoInsertUser       bool     `toml:"auto_insert_user" mapstructure:"auto_insert_user"`               // insert new users while inserting feedback
	AutoInsertItem       bool     `toml:"auto_insert_item" mapstructure:"auto_insert_item"`               // insert new items while inserting feedback
	CacheSize            int      `toml:"cache_size" mapstructure:"cache_size"`                           // cache size for recommended/popular/latest items
	PositiveFeedbackType []string `toml:"positive_feedback_types" mapstructure:"positive_feedback_types"` // positive feedback type
	ReadFeedbackTypes    []string `toml:"read_feedback_types" mapstructure:"read_feedback_types"`         // feedback type for read event
	PositiveFeedbackTTL  uint     `toml:"positive_feedback_ttl" mapstructure:"positive_feedback_ttl"`     // time-to-live of positive feedbacks
	ItemTTL              uint     `toml:"item_ttl" mapstructure:"item_ttl"`                               // item-to-live of items
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
	Port        int    `toml:"port" mapstructure:"port"`                 // master port
	Host        string `toml:"host" mapstructure:"host"`                 // master host
	HttpPort    int    `toml:"http_port" mapstructure:"http_port"`       // HTTP port
	HttpHost    string `toml:"http_host" mapstructure:"http_host"`       // HTTP host
	NumJobs     int    `toml:"n_jobs" mapstructure:"n_jobs"`             // number of working jobs
	MetaTimeout int    `toml:"meta_timeout" mapstructure:"meta_timeout"` // cluster meta timeout (second)
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
	PopularWindow                int                `toml:"popular_window" mapstructure:"popular_window"`
	FitPeriod                    int                `toml:"fit_period" mapstructure:"fit_period"`
	SearchPeriod                 int                `toml:"search_period" mapstructure:"search_period"`
	SearchEpoch                  int                `toml:"search_epoch" mapstructure:"search_epoch"`
	SearchTrials                 int                `toml:"search_trials" mapstructure:"search_trials"`
	RefreshRecommendPeriod       int                `toml:"refresh_recommend_period" mapstructure:"refresh_recommend_period"`
	FallbackRecommend            []string           `toml:"fallback_recommend" mapstructure:"fallback_recommend"`
	NumFeedbackFallbackItemBased int                `toml:"num_feedback_fallback_item_based" mapstructure:"num_feedback_fallback_item_based"`
	ExploreRecommend             map[string]float64 `toml:"explore_recommend" mapstructure:"explore_recommend"`
	ItemNeighborType             string             `toml:"item_neighbor_type" mapstructure:"item_neighbor_type"`
	UserNeighborType             string             `toml:"user_neighbor_type" mapstructure:"user_neighbor_type"`
	EnableLatestRecommend        bool               `toml:"enable_latest_recommend" mapstructure:"enable_latest_recommend"`
	EnablePopularRecommend       bool               `toml:"enable_popular_recommend" mapstructure:"enable_popular_recommend"`
	EnableUserBasedRecommend     bool               `toml:"enable_user_based_recommend" mapstructure:"enable_user_based_recommend"`
	EnableItemBasedRecommend     bool               `toml:"enable_item_based_recommend" mapstructure:"enable_item_based_recommend"`
	EnableColRecommend           bool               `toml:"enable_collaborative_recommend" mapstructure:"enable_collaborative_recommend"`
	EnableClickThroughPrediction bool               `toml:"enable_click_through_prediction" mapstructure:"enable_click_through_prediction"`
	exploreRecommendLock         sync.RWMutex
}

func (config *RecommendConfig) Lock() {
	config.exploreRecommendLock.Lock()
}

func (config *RecommendConfig) UnLock() {
	config.exploreRecommendLock.Unlock()
}

func (config *RecommendConfig) GetExploreRecommend(key string) (value float64, exist bool) {
	if config == nil {
		return 0.0, false
	}
	config.exploreRecommendLock.RLock()
	defer config.exploreRecommendLock.RUnlock()
	value, exist = config.ExploreRecommend[key]
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
			NumFeedbackFallbackItemBased: 10,
			ItemNeighborType:             "auto",
			UserNeighborType:             "auto",
			EnableLatestRecommend:        false,
			EnablePopularRecommend:       false,
			EnableUserBasedRecommend:     false,
			EnableItemBasedRecommend:     false,
			EnableColRecommend:           true,
			EnableClickThroughPrediction: false,
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
	APIKey   string `toml:"api_key" mapstructure:"api_key"`     // default number of returned items
	DefaultN int    `toml:"default_n" mapstructure:"default_n"` // secret key for RESTful APIs (SSL required)
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

func init() {
	// Default database config
	defaultDBConfig := *(*DatabaseConfig)(nil).LoadDefaultIfNil()
	viper.SetDefault("database.auto_insert_user", defaultDBConfig.AutoInsertUser)
	viper.SetDefault("database.auto_insert_item", defaultDBConfig.AutoInsertItem)
	viper.SetDefault("database.cache_size", defaultDBConfig.CacheSize)
	// Default master config
	defaultMasterConfig := *(*MasterConfig)(nil).LoadDefaultIfNil()
	viper.SetDefault("master.port", defaultMasterConfig.Port)
	viper.SetDefault("master.host", defaultMasterConfig.Host)
	viper.SetDefault("master.http_port", defaultMasterConfig.HttpPort)
	viper.SetDefault("master.http_host", defaultMasterConfig.HttpHost)
	viper.SetDefault("master.n_jobs", defaultMasterConfig.NumJobs)
	viper.SetDefault("master.meta_timeout", defaultMasterConfig.MetaTimeout)
	// Default server config
	defaultServerConfig := *(*ServerConfig)(nil).LoadDefaultIfNil()
	viper.SetDefault("server.api_key", defaultServerConfig.APIKey)
	viper.SetDefault("server.default_n", defaultServerConfig.DefaultN)
	// Default recommend config
	defaultRecommendConfig := *(*RecommendConfig)(nil).LoadDefaultIfNil()
	viper.SetDefault("recommend.popular_window", defaultRecommendConfig.PopularWindow)
	viper.SetDefault("recommend.fit_period", defaultRecommendConfig.FitPeriod)
	viper.SetDefault("recommend.search_period", defaultRecommendConfig.SearchPeriod)
	viper.SetDefault("recommend.search_epoch", defaultRecommendConfig.SearchEpoch)
	viper.SetDefault("recommend.search_trials", defaultRecommendConfig.SearchTrials)
	viper.SetDefault("recommend.refresh_recommend_period", defaultRecommendConfig.RefreshRecommendPeriod)
	viper.SetDefault("recommend.fallback_recommend", defaultRecommendConfig.FallbackRecommend)
	viper.SetDefault("recommend.num_feedback_fallback_item_based", defaultRecommendConfig.NumFeedbackFallbackItemBased)
	viper.SetDefault("recommend.item_neighbor_type", defaultRecommendConfig.ItemNeighborType)
	viper.SetDefault("recommend.user_neighbor_type", defaultRecommendConfig.UserNeighborType)
	viper.SetDefault("recommend.enable_latest_recommend", defaultRecommendConfig.EnableLatestRecommend)
	viper.SetDefault("recommend.enable_popular_recommend", defaultRecommendConfig.EnablePopularRecommend)
	viper.SetDefault("recommend.enable_user_based_recommend", defaultRecommendConfig.EnableUserBasedRecommend)
	viper.SetDefault("recommend.enable_item_based_recommend", defaultRecommendConfig.EnableItemBasedRecommend)
	viper.SetDefault("recommend.enable_collaborative_recommend", defaultRecommendConfig.EnableColRecommend)
	viper.SetDefault("recommend.enable_click_through_prediction", defaultRecommendConfig.EnableClickThroughPrediction)
}

type configBinding struct {
	key string
	env string
}

// LoadConfig loads configuration from toml file.
func LoadConfig(path string) (*Config, error) {
	// bind environment bindings
	bindings := []configBinding{
		{"database.cache_store", "GORSE_CACHE_STORE"},
		{"database.data_store", "GORSE_DATA_STORE"},
		{"master.port", "GORSE_MASTER_PORT"},
		{"master.host", "GORSE_MASTER_HOST"},
		{"master.http_port", "GORSE_MASTER_HTTP_PORT"},
		{"master.http_host", "GORSE_MASTER_HTTP_HOST"},
		{"master.n_jobs", "GORSE_MASTER_JOBS"},
		{"server.api_key", "GORSE_SERVER_API_KEY"},
	}
	for _, binding := range bindings {
		err := viper.BindEnv(binding.key, binding.env)
		if err != nil {
			base.Logger().Fatal("failed to bind a Viper key to a ENV variable", zap.Error(err))
		}
	}

	// load config file
	viper.SetConfigType("toml")
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		return nil, errors.Trace(err)
	}

	// unmarshal config file
	var conf Config
	if err := viper.Unmarshal(&conf); err != nil {
		return nil, errors.Trace(err)
	}
	conf.validate()
	return &conf, nil
}
