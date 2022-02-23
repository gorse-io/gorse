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
	Database  DatabaseConfig  `mapstructure:"database"`
	Master    MasterConfig    `mapstructure:"master"`
	Server    ServerConfig    `mapstructure:"server"`
	Recommend RecommendConfig `mapstructure:"recommend"`
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
	DataStore            string   `mapstructure:"data_store"`              // database for data store
	CacheStore           string   `mapstructure:"cache_store"`             // database for cache store
	AutoInsertUser       bool     `mapstructure:"auto_insert_user"`        // insert new users while inserting feedback
	AutoInsertItem       bool     `mapstructure:"auto_insert_item"`        // insert new items while inserting feedback
	CacheSize            int      `mapstructure:"cache_size"`              // cache size for recommended/popular/latest items
	PositiveFeedbackType []string `mapstructure:"positive_feedback_types"` // positive feedback type
	ReadFeedbackTypes    []string `mapstructure:"read_feedback_types"`     // feedback type for read event
	PositiveFeedbackTTL  uint     `mapstructure:"positive_feedback_ttl"`   // time-to-live of positive feedbacks
	ItemTTL              uint     `mapstructure:"item_ttl"`                // item-to-live of items
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
	Port              int    `mapstructure:"port"`                // master port
	Host              string `mapstructure:"host"`                // master host
	HttpPort          int    `mapstructure:"http_port"`           // HTTP port
	HttpHost          string `mapstructure:"http_host"`           // HTTP host
	NumJobs           int    `mapstructure:"n_jobs"`              // number of working jobs
	MetaTimeout       int    `mapstructure:"meta_timeout"`        // cluster meta timeout (second)
	DashboardUserName string `mapstructure:"dashboard_user_name"` // dashboard user name
	DashboardPassword string `mapstructure:"dashboard_password"`  // dashboard password
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
	PopularWindow                int                `mapstructure:"popular_window"`
	FitPeriod                    int                `mapstructure:"fit_period"`
	SearchPeriod                 int                `mapstructure:"search_period"`
	SearchEpoch                  int                `mapstructure:"search_epoch"`
	SearchTrials                 int                `mapstructure:"search_trials"`
	CheckRecommendPeriod         int                `mapstructure:"check_recommend_period"`
	RefreshRecommendPeriod       int                `mapstructure:"refresh_recommend_period"`
	FallbackRecommend            []string           `mapstructure:"fallback_recommend"`
	NumFeedbackFallbackItemBased int                `mapstructure:"num_feedback_fallback_item_based"`
	ExploreRecommend             map[string]float64 `mapstructure:"explore_recommend"`
	EnableItemNeighborIndex      bool               `mapstructure:"enable_item_neighbor_index"`
	ItemNeighborType             string             `mapstructure:"item_neighbor_type"`
	ItemNeighborIndexRecall      float32            `mapstructure:"item_neighbor_index_recall"`
	ItemNeighborIndexFitEpoch    int                `mapstructure:"item_neighbor_index_fit_epoch"`
	EnableUserNeighborIndex      bool               `mapstructure:"enable_user_neighbor_index"`
	UserNeighborType             string             `mapstructure:"user_neighbor_type"`
	UserNeighborIndexRecall      float32            `mapstructure:"user_neighbor_index_recall"`
	UserNeighborIndexFitEpoch    int                `mapstructure:"user_neighbor_index_fit_epoch"`
	EnableLatestRecommend        bool               `mapstructure:"enable_latest_recommend"`
	EnablePopularRecommend       bool               `mapstructure:"enable_popular_recommend"`
	EnableUserBasedRecommend     bool               `mapstructure:"enable_user_based_recommend"`
	EnableItemBasedRecommend     bool               `mapstructure:"enable_item_based_recommend"`
	EnableColRecommend           bool               `mapstructure:"enable_collaborative_recommend"`
	EnableColIndex               bool               `mapstructure:"enable_collaborative_index"`
	ColIndexRecall               float32            `mapstructure:"collaborative_index_recall"`
	ColIndexFitEpoch             int                `mapstructure:"collaborative_index_fit_epoch"`
	EnableClickThroughPrediction bool               `mapstructure:"enable_click_through_prediction"`
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
			CheckRecommendPeriod:         1,
			RefreshRecommendPeriod:       5,
			FallbackRecommend:            []string{"latest"},
			NumFeedbackFallbackItemBased: 10,
			ItemNeighborType:             "auto",
			EnableItemNeighborIndex:      false,
			ItemNeighborIndexRecall:      0.8,
			ItemNeighborIndexFitEpoch:    3,
			UserNeighborType:             "auto",
			EnableUserNeighborIndex:      false,
			UserNeighborIndexRecall:      0.8,
			UserNeighborIndexFitEpoch:    3,
			EnableLatestRecommend:        false,
			EnablePopularRecommend:       false,
			EnableUserBasedRecommend:     false,
			EnableItemBasedRecommend:     false,
			EnableColRecommend:           true,
			EnableColIndex:               false,
			ColIndexRecall:               0.9,
			ColIndexFitEpoch:             3,
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
	APIKey   string `mapstructure:"api_key"`   // default number of returned items
	DefaultN int    `mapstructure:"default_n"` // secret key for RESTful APIs (SSL required)
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
	viper.SetDefault("recommend.check_recommend_period", defaultRecommendConfig.CheckRecommendPeriod)
	viper.SetDefault("recommend.refresh_recommend_period", defaultRecommendConfig.RefreshRecommendPeriod)
	viper.SetDefault("recommend.fallback_recommend", defaultRecommendConfig.FallbackRecommend)
	viper.SetDefault("recommend.num_feedback_fallback_item_based", defaultRecommendConfig.NumFeedbackFallbackItemBased)
	viper.SetDefault("recommend.item_neighbor_type", defaultRecommendConfig.ItemNeighborType)
	viper.SetDefault("recommend.enable_item_neighbor_index", defaultRecommendConfig.EnableItemNeighborIndex)
	viper.SetDefault("recommend.item_neighbor_index_recall", defaultRecommendConfig.ItemNeighborIndexRecall)
	viper.SetDefault("recommend.item_neighbor_index_fit_epoch", defaultRecommendConfig.ItemNeighborIndexFitEpoch)
	viper.SetDefault("recommend.user_neighbor_type", defaultRecommendConfig.UserNeighborType)
	viper.SetDefault("recommend.enable_user_neighbor_index", defaultRecommendConfig.EnableUserNeighborIndex)
	viper.SetDefault("recommend.user_neighbor_index_recall", defaultRecommendConfig.UserNeighborIndexRecall)
	viper.SetDefault("recommend.user_neighbor_index_fit_epoch", defaultRecommendConfig.UserNeighborIndexFitEpoch)
	viper.SetDefault("recommend.enable_latest_recommend", defaultRecommendConfig.EnableLatestRecommend)
	viper.SetDefault("recommend.enable_popular_recommend", defaultRecommendConfig.EnablePopularRecommend)
	viper.SetDefault("recommend.enable_user_based_recommend", defaultRecommendConfig.EnableUserBasedRecommend)
	viper.SetDefault("recommend.enable_item_based_recommend", defaultRecommendConfig.EnableItemBasedRecommend)
	viper.SetDefault("recommend.enable_collaborative_recommend", defaultRecommendConfig.EnableColRecommend)
	viper.SetDefault("recommend.enable_collaborative_index", defaultRecommendConfig.EnableColIndex)
	viper.SetDefault("recommend.collaborative_index_recall", defaultRecommendConfig.ColIndexRecall)
	viper.SetDefault("recommend.collaborative_index_fit_epoch", defaultRecommendConfig.ColIndexFitEpoch)
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
		{"master.dashboard_user_name", "GORSE_DASHBOARD_USER_NAME"},
		{"master.dashboard_password", "GORSE_DASHBOARD_PASSWORD"},
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
