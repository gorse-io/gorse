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
	"time"
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

// DatabaseConfig is the configuration for the database.
type DatabaseConfig struct {
	DataStore  string `mapstructure:"data_store"`  // database for data store
	CacheStore string `mapstructure:"cache_store"` // database for cache store
}

// MasterConfig is the configuration for the master.
type MasterConfig struct {
	Port              int           `mapstructure:"port"`                // master port
	Host              string        `mapstructure:"host"`                // master host
	HttpPort          int           `mapstructure:"http_port"`           // HTTP port
	HttpHost          string        `mapstructure:"http_host"`           // HTTP host
	NumJobs           int           `mapstructure:"n_jobs"`              // number of working jobs
	MetaTimeout       time.Duration `mapstructure:"meta_timeout"`        // cluster meta timeout (second)
	DashboardUserName string        `mapstructure:"dashboard_user_name"` // dashboard user name
	DashboardPassword string        `mapstructure:"dashboard_password"`  // dashboard password
}

// ServerConfig is the configuration for the server.
type ServerConfig struct {
	APIKey         string        `mapstructure:"api_key"`          // default number of returned items
	DefaultN       int           `mapstructure:"default_n"`        // secret key for RESTful APIs (SSL required)
	ClockError     time.Duration `mapstructure:"clock_error"`      // clock error in the cluster in seconds
	AutoInsertUser bool          `mapstructure:"auto_insert_user"` // insert new users while inserting feedback
	AutoInsertItem bool          `mapstructure:"auto_insert_item"` // insert new items while inserting feedback
}

// RecommendConfig is the configuration of recommendation setup.
type RecommendConfig struct {
	CacheSize     int                 `mapstructure:"cache_size"`
	DataSource    DataSourceConfig    `mapstructure:"data_source"`
	Popular       PopularConfig       `mapstructure:"popular"`
	UserNeighbors NeighborsConfig     `mapstructure:"user_neighbors"`
	ItemNeighbors NeighborsConfig     `mapstructure:"item_neighbors"`
	Collaborative CollaborativeConfig `mapstructure:"collaborative"`
	Replacement   ReplacementConfig   `mapstructure:"replacement"`
	Offline       OfflineConfig       `mapstructure:"offline"`
	Online        OnlineConfig        `mapstructure:"online"`
}

type DataSourceConfig struct {
	PositiveFeedbackTypes []string `mapstructure:"positive_feedback_types"` // positive feedback type
	ReadFeedbackTypes     []string `mapstructure:"read_feedback_types"`     // feedback type for read event
	PositiveFeedbackTTL   uint     `mapstructure:"positive_feedback_ttl"`   // time-to-live of positive feedbacks
	ItemTTL               uint     `mapstructure:"item_ttl"`                // item-to-live of items
}

type PopularConfig struct {
	PopularWindow time.Duration `mapstructure:"popular_window"`
}

type NeighborsConfig struct {
	NeighborType  string  `mapstructure:"neighbor_type"`
	EnableIndex   bool    `mapstructure:"enable_index"`
	IndexRecall   float32 `mapstructure:"index_recall"`
	IndexFitEpoch int     `mapstructure:"index_fit_epoch"`
}

type CollaborativeConfig struct {
	ModelFitPeriod    time.Duration `mapstructure:"model_fit_period"`
	ModelSearchPeriod time.Duration `mapstructure:"model_search_period"`
	ModelSearchEpoch  int           `mapstructure:"model_search_epoch"`
	ModelSearchTrials int           `mapstructure:"model_search_trials"`
	EnableIndex       bool          `mapstructure:"enable_index"`
	IndexRecall       float32       `mapstructure:"index_recall"`
	IndexFitEpoch     int           `mapstructure:"index_fit_epoch"`
}

type ReplacementConfig struct {
	EnableReplacement        bool    `mapstructure:"enable_replacement"`
	PositiveReplacementDecay float64 `mapstructure:"positive_replacement_decay"`
	ReadReplacementDecay     float64 `mapstructure:"read_replacement_decay"`
}

type OfflineConfig struct {
	CheckRecommendPeriod         time.Duration      `mapstructure:"check_recommend_period"`
	RefreshRecommendPeriod       time.Duration      `mapstructure:"refresh_recommend_period"`
	ExploreRecommend             map[string]float64 `mapstructure:"explore_recommend"`
	EnableLatestRecommend        bool               `mapstructure:"enable_latest_recommend"`
	EnablePopularRecommend       bool               `mapstructure:"enable_popular_recommend"`
	EnableUserBasedRecommend     bool               `mapstructure:"enable_user_based_recommend"`
	EnableItemBasedRecommend     bool               `mapstructure:"enable_item_based_recommend"`
	EnableColRecommend           bool               `mapstructure:"enable_collaborative_recommend"`
	EnableClickThroughPrediction bool               `mapstructure:"enable_click_through_prediction"`
	exploreRecommendLock         sync.RWMutex
}

type OnlineConfig struct {
	FallbackRecommend            []string `mapstructure:"fallback_recommend"`
	NumFeedbackFallbackItemBased int      `mapstructure:"num_feedback_fallback_item_based"`
}

func GetDefaultConfig() *Config {
	return &Config{
		Master: MasterConfig{
			Port:        8086,
			Host:        "0.0.0.0",
			HttpPort:    8088,
			HttpHost:    "0.0.0.0",
			NumJobs:     1,
			MetaTimeout: 10 * time.Second,
		},
		Server: ServerConfig{
			DefaultN:       10,
			ClockError:     5 * time.Second,
			AutoInsertUser: true,
			AutoInsertItem: true,
		},
		Recommend: RecommendConfig{
			CacheSize: 100,
			Popular: PopularConfig{
				PopularWindow: 180 * 24 * time.Hour,
			},
			UserNeighbors: NeighborsConfig{
				NeighborType:  "auto",
				EnableIndex:   true,
				IndexRecall:   0.8,
				IndexFitEpoch: 3,
			},
			ItemNeighbors: NeighborsConfig{
				NeighborType:  "auto",
				EnableIndex:   true,
				IndexRecall:   0.8,
				IndexFitEpoch: 3,
			},
			Collaborative: CollaborativeConfig{
				ModelFitPeriod:    60 * time.Minute,
				ModelSearchPeriod: 180 * time.Minute,
				ModelSearchEpoch:  100,
				ModelSearchTrials: 10,
				EnableIndex:       true,
				IndexRecall:       0.9,
				IndexFitEpoch:     3,
			},
			Replacement: ReplacementConfig{
				EnableReplacement:        false,
				PositiveReplacementDecay: 0.8,
				ReadReplacementDecay:     0.6,
			},
			Offline: OfflineConfig{
				CheckRecommendPeriod:         time.Minute,
				RefreshRecommendPeriod:       120 * time.Hour,
				EnableLatestRecommend:        false,
				EnablePopularRecommend:       false,
				EnableUserBasedRecommend:     false,
				EnableItemBasedRecommend:     false,
				EnableColRecommend:           true,
				EnableClickThroughPrediction: false,
			},
			Online: OnlineConfig{
				FallbackRecommend:            []string{"latest"},
				NumFeedbackFallbackItemBased: 10,
			},
		},
	}
}

func (config *OfflineConfig) Lock() {
	config.exploreRecommendLock.Lock()
}

func (config *OfflineConfig) UnLock() {
	config.exploreRecommendLock.Unlock()
}

func (config *OfflineConfig) GetExploreRecommend(key string) (value float64, exist bool) {
	if config == nil {
		return 0.0, false
	}
	config.exploreRecommendLock.RLock()
	defer config.exploreRecommendLock.RUnlock()
	value, exist = config.ExploreRecommend[key]
	return
}

func setDefault() {
	defaultConfig := GetDefaultConfig()
	// [master]
	viper.SetDefault("master.port", defaultConfig.Master.Port)
	viper.SetDefault("master.host", defaultConfig.Master.Host)
	viper.SetDefault("master.http_port", defaultConfig.Master.HttpPort)
	viper.SetDefault("master.http_host", defaultConfig.Master.HttpHost)
	viper.SetDefault("master.n_jobs", defaultConfig.Master.NumJobs)
	viper.SetDefault("master.meta_timeout", defaultConfig.Master.MetaTimeout)
	// [server]
	viper.SetDefault("server.api_key", defaultConfig.Server.APIKey)
	viper.SetDefault("server.default_n", defaultConfig.Server.DefaultN)
	viper.SetDefault("server.clock_error", defaultConfig.Server.ClockError)
	viper.SetDefault("server.auto_insert_user", defaultConfig.Server.AutoInsertUser)
	viper.SetDefault("server.auto_insert_item", defaultConfig.Server.AutoInsertItem)
	// [recommend]
	viper.SetDefault("recommend.cache_size", defaultConfig.Recommend.CacheSize)
	// [recommend.popular]
	viper.SetDefault("recommend.popular.popular_window", defaultConfig.Recommend.Popular.PopularWindow)
	// [recommend.user_neighbors]
	viper.SetDefault("recommend.user_neighbors.neighbor_type", defaultConfig.Recommend.UserNeighbors.NeighborType)
	viper.SetDefault("recommend.user_neighbors.enable_index", defaultConfig.Recommend.UserNeighbors.EnableIndex)
	viper.SetDefault("recommend.user_neighbors.index_recall", defaultConfig.Recommend.UserNeighbors.IndexRecall)
	viper.SetDefault("recommend.user_neighbors.index_fit_epoch", defaultConfig.Recommend.UserNeighbors.IndexFitEpoch)
	// [recommend.item_neighbors]
	viper.SetDefault("recommend.item_neighbors.neighbor_type", defaultConfig.Recommend.ItemNeighbors.NeighborType)
	viper.SetDefault("recommend.item_neighbors.enable_index", defaultConfig.Recommend.ItemNeighbors.EnableIndex)
	viper.SetDefault("recommend.item_neighbors.index_recall", defaultConfig.Recommend.ItemNeighbors.IndexRecall)
	viper.SetDefault("recommend.item_neighbors.index_fit_epoch", defaultConfig.Recommend.ItemNeighbors.IndexFitEpoch)
	// [recommend.collaborative]
	viper.SetDefault("recommend.collaborative.model_fit_period", defaultConfig.Recommend.Collaborative.ModelFitPeriod)
	viper.SetDefault("recommend.collaborative.model_search_period", defaultConfig.Recommend.Collaborative.ModelSearchPeriod)
	viper.SetDefault("recommend.collaborative.model_search_epoch", defaultConfig.Recommend.Collaborative.ModelSearchEpoch)
	viper.SetDefault("recommend.collaborative.model_search_trials", defaultConfig.Recommend.Collaborative.ModelSearchTrials)
	viper.SetDefault("recommend.collaborative.enable_index", defaultConfig.Recommend.Collaborative.EnableIndex)
	viper.SetDefault("recommend.collaborative.index_recall", defaultConfig.Recommend.Collaborative.IndexRecall)
	viper.SetDefault("recommend.collaborative.index_fit_epoch", defaultConfig.Recommend.Collaborative.IndexFitEpoch)
	// [recommend.replacement]
	viper.SetDefault("recommend.replacement.enable_replacement", defaultConfig.Recommend.Replacement.EnableReplacement)
	viper.SetDefault("recommend.replacement.positive_replacement_decay", defaultConfig.Recommend.Replacement.PositiveReplacementDecay)
	viper.SetDefault("recommend.replacement.read_replacement_decay", defaultConfig.Recommend.Replacement.ReadReplacementDecay)
	// [recommend.offline]
	viper.SetDefault("recommend.offline.check_recommend_period", defaultConfig.Recommend.Offline.CheckRecommendPeriod)
	viper.SetDefault("recommend.offline.refresh_recommend_period", defaultConfig.Recommend.Offline.RefreshRecommendPeriod)
	viper.SetDefault("recommend.offline.enable_latest_recommend", defaultConfig.Recommend.Offline.EnableLatestRecommend)
	viper.SetDefault("recommend.offline.enable_popular_recommend", defaultConfig.Recommend.Offline.EnablePopularRecommend)
	viper.SetDefault("recommend.offline.enable_user_based_recommend", defaultConfig.Recommend.Offline.EnableUserBasedRecommend)
	viper.SetDefault("recommend.offline.enable_item_based_recommend", defaultConfig.Recommend.Offline.EnableItemBasedRecommend)
	viper.SetDefault("recommend.offline.enable_collaborative_recommend", defaultConfig.Recommend.Offline.EnableColRecommend)
	viper.SetDefault("recommend.offline.enable_click_through_prediction", defaultConfig.Recommend.Offline.EnableClickThroughPrediction)
	// [recommend.online]
	viper.SetDefault("recommend.online.fallback_recommend", defaultConfig.Recommend.Online.FallbackRecommend)
	viper.SetDefault("recommend.online.num_feedback_fallback_item_based", defaultConfig.Recommend.Online.NumFeedbackFallbackItemBased)
}

type configBinding struct {
	key string
	env string
}

// LoadConfig loads configuration from toml file.
func LoadConfig(path string) (*Config, error) {
	// set default config
	setDefault()

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
	return &conf, nil
}
