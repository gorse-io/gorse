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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sclevine/yj/convert"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestUnmarshal(t *testing.T) {
	data, err := os.ReadFile("config.toml")
	assert.NoError(t, err)
	text := string(data)
	text = strings.Replace(text, "ssl_mode = false", "ssl_mode = true", -1)
	text = strings.Replace(text, "ssl_ca = \"\"", "ssl_ca = \"ca.pem\"", -1)
	text = strings.Replace(text, "ssl_cert = \"\"", "ssl_cert = \"cert.pem\"", -1)
	text = strings.Replace(text, "ssl_key = \"\"", "ssl_key = \"key.pem\"", -1)
	text = strings.Replace(text, "dashboard_user_name = \"\"", "dashboard_user_name = \"admin\"", -1)
	text = strings.Replace(text, "dashboard_password = \"\"", "dashboard_password = \"password\"", -1)
	text = strings.Replace(text, "admin_api_key = \"\"", "admin_api_key = \"super_api_key\"", -1)
	text = strings.Replace(text, "api_key = \"\"", "api_key = \"19260817\"", -1)
	text = strings.Replace(text, "table_prefix = \"\"", "table_prefix = \"gorse_\"", -1)
	text = strings.Replace(text, "cache_table_prefix = \"gorse_\"", "cache_table_prefix = \"gorse_cache_\"", -1)
	text = strings.Replace(text, "data_table_prefix = \"gorse_\"", "data_table_prefix = \"gorse_data_\"", -1)
	text = strings.Replace(text, "http_cors_domains = []", "http_cors_domains = [\".*\"]", -1)
	text = strings.Replace(text, "http_cors_methods = []", "http_cors_methods = [\"GET\",\"PATCH\",\"POST\"]", -1)
	text = strings.Replace(text, "issuer = \"\"", "issuer = \"https://accounts.google.com\"", -1)
	text = strings.Replace(text, "client_id = \"\"", "client_id = \"client_id\"", -1)
	text = strings.Replace(text, "client_secret = \"\"", "client_secret = \"client_secret\"", -1)
	text = strings.Replace(text, "redirect_url = \"\"", "redirect_url = \"http://localhost:8088/callback/oauth2\"", -1)
	r, err := convert.TOML{}.Decode(bytes.NewBufferString(text))
	assert.NoError(t, err)

	encodings := []convert.Encoding{convert.TOML{}, convert.YAML{}, convert.JSON{}}
	for _, encoding := range encodings {
		t.Run(encoding.String(), func(t *testing.T) {
			filePath := filepath.Join(os.TempDir(), fmt.Sprintf("config.%s", strings.ToLower(encoding.String())))
			fp, err := os.Create(filePath)
			assert.NoError(t, err)
			err = encoding.Encode(fp, r)
			assert.NoError(t, err)

			config, err := LoadConfig(filePath, false)
			assert.NoError(t, err)
			// [database]
			assert.Equal(t, "redis://localhost:6379/0", config.Database.CacheStore)
			assert.Equal(t, "mysql://gorse:gorse_pass@tcp(localhost:3306)/gorse", config.Database.DataStore)
			assert.Equal(t, "gorse_", config.Database.TablePrefix)
			assert.Equal(t, "gorse_cache_", config.Database.CacheTablePrefix)
			assert.Equal(t, "gorse_data_", config.Database.DataTablePrefix)
			assert.Equal(t, "READ-UNCOMMITTED", config.Database.MySQL.IsolationLevel)
			// [master]
			assert.Equal(t, 8086, config.Master.Port)
			assert.Equal(t, "0.0.0.0", config.Master.Host)
			assert.Equal(t, true, config.Master.SSLMode)
			assert.Equal(t, "ca.pem", config.Master.SSLCA)
			assert.Equal(t, "cert.pem", config.Master.SSLCert)
			assert.Equal(t, "key.pem", config.Master.SSLKey)
			assert.Equal(t, 8088, config.Master.HttpPort)
			assert.Equal(t, "0.0.0.0", config.Master.HttpHost)
			assert.Equal(t, []string{".*"}, config.Master.HttpCorsDomains)
			assert.Equal(t, []string{"GET", "PATCH", "POST"}, config.Master.HttpCorsMethods)
			assert.Equal(t, 1, config.Master.NumJobs)
			assert.Equal(t, 10*time.Second, config.Master.MetaTimeout)
			assert.Equal(t, "admin", config.Master.DashboardUserName)
			assert.Equal(t, "password", config.Master.DashboardPassword)
			assert.Equal(t, "super_api_key", config.Master.AdminAPIKey)
			// [server]
			assert.Equal(t, 10, config.Server.DefaultN)
			assert.Equal(t, "19260817", config.Server.APIKey)
			assert.Equal(t, 5*time.Second, config.Server.ClockError)
			assert.True(t, config.Server.AutoInsertUser)
			assert.True(t, config.Server.AutoInsertItem)
			assert.Equal(t, 10*time.Second, config.Server.CacheExpire)
			// [recommend]
			assert.Equal(t, 100, config.Recommend.CacheSize)
			assert.Equal(t, 72*time.Hour, config.Recommend.CacheExpire)
			// [recommend.data_source]
			assert.Equal(t, []string{"star", "like"}, config.Recommend.DataSource.PositiveFeedbackTypes)
			assert.Equal(t, []string{"read"}, config.Recommend.DataSource.ReadFeedbackTypes)
			assert.Equal(t, uint(0), config.Recommend.DataSource.PositiveFeedbackTTL)
			assert.Equal(t, uint(0), config.Recommend.DataSource.ItemTTL)
			// [recommend.popular]
			assert.Equal(t, 30*24*time.Hour, config.Recommend.Popular.PopularWindow)
			// [recommend.leaderboards]
			assert.Len(t, config.Recommend.NonPersonalized, 1)
			assert.Equal(t, "most_starred_weekly", config.Recommend.NonPersonalized[0].Name)
			assert.Equal(t, "count(feedback, .FeedbackType == 'star')", config.Recommend.NonPersonalized[0].Score)
			assert.Equal(t, "(now() - item.Timestamp).Hours() < 168", config.Recommend.NonPersonalized[0].Filter)
			// [recommend.collaborative]
			assert.Equal(t, 60*time.Minute, config.Recommend.Collaborative.ModelFitPeriod)
			assert.Equal(t, 360*time.Minute, config.Recommend.Collaborative.ModelSearchPeriod)
			assert.Equal(t, 100, config.Recommend.Collaborative.ModelSearchEpoch)
			assert.Equal(t, 10, config.Recommend.Collaborative.ModelSearchTrials)
			assert.False(t, config.Recommend.Collaborative.EnableModelSizeSearch)
			// [recommend.replacement]
			assert.False(t, config.Recommend.Replacement.EnableReplacement)
			assert.Equal(t, 0.8, config.Recommend.Replacement.PositiveReplacementDecay)
			assert.Equal(t, 0.6, config.Recommend.Replacement.ReadReplacementDecay)
			// [recommend.offline]
			assert.Equal(t, time.Minute, config.Recommend.Offline.CheckRecommendPeriod)
			assert.Equal(t, 24*time.Hour, config.Recommend.Offline.RefreshRecommendPeriod)
			assert.True(t, config.Recommend.Offline.EnableColRecommend)
			assert.False(t, config.Recommend.Offline.EnableItemBasedRecommend)
			assert.True(t, config.Recommend.Offline.EnableUserBasedRecommend)
			assert.False(t, config.Recommend.Offline.EnablePopularRecommend)
			assert.True(t, config.Recommend.Offline.EnableLatestRecommend)
			assert.True(t, config.Recommend.Offline.EnableClickThroughPrediction)
			assert.Equal(t, map[string]float64{"popular": 0.1, "latest": 0.2}, config.Recommend.Offline.ExploreRecommend)
			value, exist := config.Recommend.Offline.GetExploreRecommend("popular")
			assert.Equal(t, true, exist)
			assert.Equal(t, 0.1, value)
			value, exist = config.Recommend.Offline.GetExploreRecommend("latest")
			assert.Equal(t, true, exist)
			assert.Equal(t, 0.2, value)
			_, exist = config.Recommend.Offline.GetExploreRecommend("unknown")
			assert.Equal(t, false, exist)
			// [recommend.online]
			assert.Equal(t, []string{"item_based", "latest"}, config.Recommend.Online.FallbackRecommend)
			assert.Equal(t, 10, config.Recommend.Online.NumFeedbackFallbackItemBased)
			// [tracing]
			assert.False(t, config.Tracing.EnableTracing)
			assert.Equal(t, "jaeger", config.Tracing.Exporter)
			assert.Equal(t, "http://localhost:14268/api/traces", config.Tracing.CollectorEndpoint)
			assert.Equal(t, "always", config.Tracing.Sampler)
			assert.Equal(t, 1.0, config.Tracing.Ratio)
			// [experimental]
			assert.Equal(t, 128, config.Experimental.DeepLearningBatchSize)
			// [oauth2]
			assert.Equal(t, "https://accounts.google.com", config.OIDC.Issuer)
			assert.Equal(t, "client_id", config.OIDC.ClientID)
			assert.Equal(t, "client_secret", config.OIDC.ClientSecret)
			assert.Equal(t, "http://localhost:8088/callback/oauth2", config.OIDC.RedirectURL)
			// [openai]
			assert.Equal(t, "http://localhost:11434/v1", config.OpenAI.BaseURL)
			assert.Equal(t, "ollama", config.OpenAI.AuthToken)
			assert.Equal(t, "qwen2.5", config.OpenAI.ChatCompletionModel)
			assert.Equal(t, 15000, config.OpenAI.ChatCompletionRPM)
			assert.Equal(t, 1200000, config.OpenAI.ChatCompletionTPM)
			assert.Equal(t, "mxbai-embed-large", config.OpenAI.EmbeddingModel)
			assert.Equal(t, 1024, config.OpenAI.EmbeddingDimensions)
			assert.Equal(t, 1800, config.OpenAI.EmbeddingRPM)
			assert.Equal(t, 1200000, config.OpenAI.EmbeddingTPM)
		})
	}
}

func TestSetDefault(t *testing.T) {
	setDefault()
	viper.SetConfigType("toml")
	err := viper.ReadConfig(strings.NewReader(""))
	assert.NoError(t, err)
	var config Config
	err = viper.Unmarshal(&config)
	assert.NoError(t, err)
	assert.Equal(t, GetDefaultConfig(), &config)
}

type environmentVariable struct {
	key   string
	value string
}

func TestBindEnv(t *testing.T) {
	variables := []environmentVariable{
		{"GORSE_CACHE_STORE", "redis://<cache_store>"},
		{"GORSE_DATA_STORE", "mysql://<data_store>"},
		{"GORSE_TABLE_PREFIX", "gorse_"},
		{"GORSE_DATA_TABLE_PREFIX", "gorse_data_"},
		{"GORSE_CACHE_TABLE_PREFIX", "gorse_cache_"},
		{"GORSE_MASTER_PORT", "123"},
		{"GORSE_MASTER_HOST", "<master_host>"},
		{"GORSE_MASTER_SSL_MODE", "true"},
		{"GORSE_MASTER_SSL_CA", "ca.pem"},
		{"GORSE_MASTER_SSL_CERT", "cert.pem"},
		{"GORSE_MASTER_SSL_KEY", "key.pem"},
		{"GORSE_MASTER_HTTP_PORT", "456"},
		{"GORSE_MASTER_HTTP_HOST", "<master_http_host>"},
		{"GORSE_MASTER_JOBS", "789"},
		{"GORSE_DASHBOARD_USER_NAME", "user_name"},
		{"GORSE_DASHBOARD_PASSWORD", "password"},
		{"GORSE_DASHBOARD_AUTH_SERVER", "http://127.0.0.1:8888"},
		{"GORSE_DASHBOARD_REDACTED", "true"},
		{"GORSE_ADMIN_API_KEY", "<admin_api_key>"},
		{"GORSE_SERVER_API_KEY", "<server_api_key>"},
		{"GORSE_OIDC_ENABLE", "true"},
		{"GORSE_OIDC_ISSUER", "https://accounts.google.com"},
		{"GORSE_OIDC_CLIENT_ID", "client_id"},
		{"GORSE_OIDC_CLIENT_SECRET", "client_secret"},
		{"GORSE_OIDC_REDIRECT_URL", "http://localhost:8088/callback/oauth2"},
	}
	for _, variable := range variables {
		t.Setenv(variable.key, variable.value)
	}

	config, err := LoadConfig("config.toml", false)
	assert.NoError(t, err)
	assert.Equal(t, "redis://<cache_store>", config.Database.CacheStore)
	assert.Equal(t, "mysql://<data_store>", config.Database.DataStore)
	assert.Equal(t, "gorse_", config.Database.TablePrefix)
	assert.Equal(t, "gorse_cache_", config.Database.CacheTablePrefix)
	assert.Equal(t, "gorse_data_", config.Database.DataTablePrefix)
	assert.Equal(t, 123, config.Master.Port)
	assert.Equal(t, "<master_host>", config.Master.Host)
	assert.Equal(t, true, config.Master.SSLMode)
	assert.Equal(t, "ca.pem", config.Master.SSLCA)
	assert.Equal(t, "cert.pem", config.Master.SSLCert)
	assert.Equal(t, "key.pem", config.Master.SSLKey)
	assert.Equal(t, 456, config.Master.HttpPort)
	assert.Equal(t, "<master_http_host>", config.Master.HttpHost)
	assert.Equal(t, 789, config.Master.NumJobs)
	assert.Equal(t, "user_name", config.Master.DashboardUserName)
	assert.Equal(t, "password", config.Master.DashboardPassword)
	assert.Equal(t, true, config.Master.DashboardRedacted)
	assert.Equal(t, "<admin_api_key>", config.Master.AdminAPIKey)
	assert.Equal(t, "<server_api_key>", config.Server.APIKey)
	assert.Equal(t, true, config.OIDC.Enable)
	assert.Equal(t, "https://accounts.google.com", config.OIDC.Issuer)
	assert.Equal(t, "client_id", config.OIDC.ClientID)
	assert.Equal(t, "client_secret", config.OIDC.ClientSecret)
	assert.Equal(t, "http://localhost:8088/callback/oauth2", config.OIDC.RedirectURL)

	// check default values
	assert.Equal(t, 100, config.Recommend.CacheSize)
}

func TestTablePrefixCompat(t *testing.T) {
	data, err := os.ReadFile("config.toml")
	assert.NoError(t, err)
	text := string(data)
	text = strings.Replace(text, "cache_table_prefix = \"\"", "", -1)
	text = strings.Replace(text, "data_table_prefix = \"\"", "", -1)
	text = strings.Replace(text, "table_prefix = \"\"", "table_prefix = \"gorse_\"", -1)
	path := filepath.Join(t.TempDir(), "config.toml")
	err = os.WriteFile(path, []byte(text), os.ModePerm)
	assert.NoError(t, err)

	config, err := LoadConfig(path, false)
	assert.NoError(t, err)
	assert.Equal(t, "gorse_", config.Database.TablePrefix)
	assert.Equal(t, "gorse_", config.Database.CacheTablePrefix)
	assert.Equal(t, "gorse_", config.Database.DataTablePrefix)
}

func TestConfig_OfflineRecommendDigest(t *testing.T) {
	// test explore recommendation
	cfg1, cfg2 := GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.ExploreRecommend = map[string]float64{"a": 0.5, "b": 0.6}
	cfg2.Recommend.Offline.ExploreRecommend = map[string]float64{"a": 0.6, "b": 0.5}
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	// test latest recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableLatestRecommend = true
	cfg2.Recommend.Offline.EnableLatestRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	// test popular recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnablePopularRecommend = true
	cfg2.Recommend.Offline.EnablePopularRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnablePopularRecommend = true
	cfg2.Recommend.Offline.EnablePopularRecommend = true
	cfg1.Recommend.Popular.PopularWindow = 10
	cfg2.Recommend.Popular.PopularWindow = 11
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnablePopularRecommend = false
	cfg2.Recommend.Offline.EnablePopularRecommend = false
	cfg1.Recommend.Popular.PopularWindow = 10
	cfg2.Recommend.Popular.PopularWindow = 11
	assert.Equal(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	// test user-based recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableUserBasedRecommend = true
	cfg2.Recommend.Offline.EnableUserBasedRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	// test item-based recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableItemBasedRecommend = true
	cfg2.Recommend.Offline.EnableItemBasedRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	// test collaborative-filtering recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableColRecommend = true
	cfg2.Recommend.Offline.EnableColRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableColRecommend = true
	cfg2.Recommend.Offline.EnableColRecommend = true
	assert.Equal(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableColRecommend = false
	cfg2.Recommend.Offline.EnableColRecommend = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(WithCollaborative(true)), cfg2.OfflineRecommendDigest())

	// test click-through rate prediction recommendation
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableClickThroughPrediction = true
	cfg2.Recommend.Offline.EnableClickThroughPrediction = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Offline.EnableClickThroughPrediction = false
	cfg2.Recommend.Offline.EnableClickThroughPrediction = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(WithRanking(true)), cfg2.OfflineRecommendDigest())

	// test replacement
	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Replacement.EnableReplacement = true
	cfg2.Recommend.Replacement.EnableReplacement = false
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Replacement.EnableReplacement = true
	cfg2.Recommend.Replacement.EnableReplacement = true
	cfg1.Recommend.Replacement.PositiveReplacementDecay = 0.1
	cfg2.Recommend.Replacement.PositiveReplacementDecay = 0.2
	assert.NotEqual(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())

	cfg1, cfg2 = GetDefaultConfig(), GetDefaultConfig()
	cfg1.Recommend.Replacement.EnableReplacement = false
	cfg2.Recommend.Replacement.EnableReplacement = false
	cfg1.Recommend.Replacement.PositiveReplacementDecay = 0.1
	cfg2.Recommend.Replacement.PositiveReplacementDecay = 0.2
	assert.Equal(t, cfg1.OfflineRecommendDigest(), cfg2.OfflineRecommendDigest())
}

func TestItemToItemConfig_Hash(t *testing.T) {
	a := ItemToItemConfig{}
	b := ItemToItemConfig{}
	assert.Equal(t, a.Hash(), b.Hash())

	a = ItemToItemConfig{Name: "a"}
	b = ItemToItemConfig{Name: "b"}
	assert.NotEqual(t, a.Hash(), b.Hash())

	a = ItemToItemConfig{Type: "a"}
	b = ItemToItemConfig{Type: "b"}
	assert.NotEqual(t, a.Hash(), b.Hash())

	a = ItemToItemConfig{Column: "a"}
	b = ItemToItemConfig{Column: "b"}
	assert.NotEqual(t, a.Hash(), b.Hash())
}

type ValidateTestSuite struct {
	suite.Suite
	*Config
}

func (s *ValidateTestSuite) SetupTest() {
	s.Config = GetDefaultConfig()
	s.Database.CacheStore = "redis://localhost:6379/0"
	s.Database.DataStore = "mysql://gorse:gorse_pass@tcp(localhost:3306)/gorse"
}

func (s *ValidateTestSuite) TestDuplicateNonPersonalized() {
	s.Recommend.NonPersonalized = []NonPersonalizedConfig{{
		Name:  "most_starred_weekly",
		Score: "count(feedback, .FeedbackType == 'star')",
	}, {
		Name:  "most_starred_weekly",
		Score: "count(feedback, .FeedbackType == 'star')",
	}}
	s.Error(s.Validate())
}

func (s *ValidateTestSuite) TestDuplicateItemToItem() {
	s.Recommend.ItemToItem = []ItemToItemConfig{{
		Name: "item_to_item",
		Type: "users",
	}, {
		Name: "item_to_item",
		Type: "users",
	}}
	s.Error(s.Validate())
}

func TestValidate(t *testing.T) {
	suite.Run(t, new(ValidateTestSuite))
}
