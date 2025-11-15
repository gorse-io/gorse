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
	"context"
	"crypto/md5"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/expr-lang/expr/parser"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	"github.com/go-viper/mapstructure/v2"
	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.8.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func init() {
	viper.SetOptions(viper.WithDecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		StringToFeedbackTypeHookFunc(),
	)))
}

// Config is the configuration for the engine.
type Config struct {
	Database  DatabaseConfig  `mapstructure:"database"`
	Master    MasterConfig    `mapstructure:"master"`
	Server    ServerConfig    `mapstructure:"server"`
	Recommend RecommendConfig `mapstructure:"recommend"`
	Tracing   TracingConfig   `mapstructure:"tracing"`
	OIDC      OIDCConfig      `mapstructure:"oidc"`
	OpenAI    OpenAIConfig    `mapstructure:"openai"`
	S3        S3Config        `mapstructure:"s3"`
}

// DatabaseConfig is the configuration for the database.
type DatabaseConfig struct {
	DataStore        string      `mapstructure:"data_store" validate:"required,data_store"`   // database for data store
	CacheStore       string      `mapstructure:"cache_store" validate:"required,cache_store"` // database for cache store
	TablePrefix      string      `mapstructure:"table_prefix"`
	DataTablePrefix  string      `mapstructure:"data_table_prefix"`
	CacheTablePrefix string      `mapstructure:"cache_table_prefix"`
	MySQL            MySQLConfig `mapstructure:"mysql"`
}

type MySQLConfig struct {
	IsolationLevel string `mapstructure:"isolation_level" validate:"oneof=READ-UNCOMMITTED READ-COMMITTED REPEATABLE-READ SERIALIZABLE"`
}

// MasterConfig is the configuration for the master.
type MasterConfig struct {
	Port              int           `mapstructure:"port" validate:"gte=0"`        // master port
	Host              string        `mapstructure:"host"`                         // master host
	SSLMode           bool          `mapstructure:"ssl_mode"`                     // enable SSL mode
	SSLCA             string        `mapstructure:"ssl_ca"`                       // SSL CA file
	SSLCert           string        `mapstructure:"ssl_cert"`                     // SSL certificate file
	SSLKey            string        `mapstructure:"ssl_key"`                      // SSL key file
	HttpPort          int           `mapstructure:"http_port" validate:"gte=0"`   // HTTP port
	HttpHost          string        `mapstructure:"http_host"`                    // HTTP host
	HttpCorsDomains   []string      `mapstructure:"http_cors_domains"`            // add allowed cors domains
	HttpCorsMethods   []string      `mapstructure:"http_cors_methods"`            // add allowed cors methods
	NumJobs           int           `mapstructure:"n_jobs" validate:"gt=0"`       // number of working jobs
	MetaTimeout       time.Duration `mapstructure:"meta_timeout" validate:"gt=0"` // cluster meta timeout (second)
	DashboardUserName string        `mapstructure:"dashboard_user_name"`          // dashboard user name
	DashboardPassword string        `mapstructure:"dashboard_password"`           // dashboard password
	DashboardRedacted bool          `mapstructure:"dashboard_redacted"`
	AdminAPIKey       string        `mapstructure:"admin_api_key"`
}

// ServerConfig is the configuration for the server.
type ServerConfig struct {
	APIKey         string        `mapstructure:"api_key"`                      // default number of returned items
	DefaultN       int           `mapstructure:"default_n" validate:"gt=0"`    // secret key for RESTful APIs (SSL required)
	ClockError     time.Duration `mapstructure:"clock_error" validate:"gte=0"` // clock error in the cluster in seconds
	AutoInsertUser bool          `mapstructure:"auto_insert_user"`             // insert new users while inserting feedback
	AutoInsertItem bool          `mapstructure:"auto_insert_item"`             // insert new items while inserting feedback
	CacheExpire    time.Duration `mapstructure:"cache_expire" validate:"gt=0"` // server-side cache expire time
}

// RecommendConfig is the configuration of recommendation setup.
type RecommendConfig struct {
	CacheSize       int                     `mapstructure:"cache_size" validate:"gt=0"`
	CacheExpire     time.Duration           `mapstructure:"cache_expire" validate:"gt=0"`
	ContextSize     int                     `mapstructure:"context_size" validate:"gt=0"`
	ActiveUserTTL   int                     `mapstructure:"active_user_ttl" validate:"gte=0"`
	DataSource      DataSourceConfig        `mapstructure:"data_source"`
	NonPersonalized []NonPersonalizedConfig `mapstructure:"non-personalized" validate:"dive"`
	ItemToItem      []ItemToItemConfig      `mapstructure:"item-to-item" validate:"dive"`
	UserToUser      []UserToUserConfig      `mapstructure:"user-to-user" validate:"dive"`
	Collaborative   CollaborativeConfig     `mapstructure:"collaborative"`
	External        []ExternalConfig        `mapstructure:"external" validate:"dive"`
	Replacement     ReplacementConfig       `mapstructure:"replacement"`
	Ranker          RankerConfig            `mapstructure:"ranker"`
	Fallback        FallbackConfig          `mapstructure:"fallback"`
}

func (r *RecommendConfig) ListRecommenders() []string {
	recommenders := make([]string, 0)
	for _, rec := range r.NonPersonalized {
		recommenders = append(recommenders, rec.FullName())
	}
	for _, rec := range r.ItemToItem {
		recommenders = append(recommenders, rec.FullName())
	}
	for _, rec := range r.UserToUser {
		recommenders = append(recommenders, rec.FullName())
	}
	for _, rec := range r.External {
		recommenders = append(recommenders, rec.FullName())
	}
	recommenders = append(recommenders, r.Collaborative.FullName())
	recommenders = append(recommenders, "latest")
	return recommenders
}

func (r *RecommendConfig) Hash() string {
	recommenders := mapset.NewSet(r.Ranker.Recommenders...)
	if recommenders.IsEmpty() {
		recommenders.Append((r.ListRecommenders())...)
	}
	var digests []string
	for _, rec := range r.NonPersonalized {
		if recommenders.Contains(rec.FullName()) {
			digests = append(digests, rec.Hash())
		}
	}
	for _, rec := range r.ItemToItem {
		if recommenders.Contains(rec.FullName()) {
			digests = append(digests, rec.Hash(r))
		}
	}
	for _, rec := range r.UserToUser {
		if recommenders.Contains(rec.FullName()) {
			digests = append(digests, rec.Hash(r))
		}
	}
	for _, rec := range r.External {
		if recommenders.Contains(rec.FullName()) {
			digests = append(digests, rec.Hash())
		}
	}
	if recommenders.Contains(r.Collaborative.FullName()) {
		digests = append(digests, r.Collaborative.Hash(r))
	}
	if recommenders.Contains("latest") {
		digests = append(digests, "latest")
	}
	return util.MD5(digests...)
}

func StringToFeedbackTypeHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{},
	) (interface{}, error) {
		if f.Kind() == reflect.String && t == reflect.TypeOf(expression.FeedbackTypeExpression{}) {
			var expr expression.FeedbackTypeExpression
			if err := expr.FromString(data.(string)); err != nil {
				return nil, errors.Trace(err)
			}
			return expr, nil // only convert string to FeedbackType
		}
		return data, nil
	}
}

type DataSourceConfig struct {
	PositiveFeedbackTypes []expression.FeedbackTypeExpression `mapstructure:"positive_feedback_types"`                // positive feedback type
	ReadFeedbackTypes     []expression.FeedbackTypeExpression `mapstructure:"read_feedback_types"`                    // feedback type for read event
	PositiveFeedbackTTL   uint                                `mapstructure:"positive_feedback_ttl" validate:"gte=0"` // time-to-live of positive feedbacks
	ItemTTL               uint                                `mapstructure:"item_ttl" validate:"gte=0"`              // item-to-live of items
}

type NonPersonalizedConfig struct {
	Name   string `mapstructure:"name" json:"name"`
	Score  string `mapstructure:"score" json:"score" validate:"required,item_expr"`
	Filter string `mapstructure:"filter" json:"filter" validate:"item_expr"`
}

func (config *NonPersonalizedConfig) FullName() string {
	return "non-personalized/" + config.Name
}

func (config *NonPersonalizedConfig) Hash() string {
	hash := md5.New()
	hash.Write([]byte(config.Name))
	hash.Write([]byte(config.Score))
	hash.Write([]byte(config.Filter))
	return hex.EncodeToString(hash.Sum(nil)[:])
}

type ItemToItemConfig struct {
	Name   string `mapstructure:"name" json:"name"`
	Type   string `mapstructure:"type" json:"type" validate:"oneof=embedding tags users chat auto"`
	Column string `mapstructure:"column" json:"column" validate:"item_expr"`
	Prompt string `mapstructure:"prompt" json:"prompt"`
}

func (config *ItemToItemConfig) FullName() string {
	return "item-to-item/" + config.Name
}

func (config *ItemToItemConfig) Hash(cfg *RecommendConfig) string {
	hash := md5.New()
	hash.Write([]byte(config.Name))
	hash.Write([]byte(config.Type))
	hash.Write([]byte(config.Column))
	if config.Type == "users" {
		for _, expr := range cfg.DataSource.PositiveFeedbackTypes {
			hash.Write([]byte(expr.String()))
		}
	}
	return hex.EncodeToString(hash.Sum(nil)[:])
}

type UserToUserConfig struct {
	Name   string `mapstructure:"name" json:"name"`
	Type   string `mapstructure:"type" json:"type" validate:"oneof=embedding tags items auto"`
	Column string `mapstructure:"column" json:"column" validate:"item_expr"`
}

func (config *UserToUserConfig) FullName() string {
	return "user-to-user/" + config.Name
}

func (config *UserToUserConfig) Hash(cfg *RecommendConfig) string {
	hash := md5.New()
	hash.Write([]byte(config.Name))
	hash.Write([]byte(config.Type))
	hash.Write([]byte(config.Column))
	if config.Type == "items" {
		for _, expr := range cfg.DataSource.PositiveFeedbackTypes {
			hash.Write([]byte(expr.String()))
		}
	}
	return hex.EncodeToString(hash.Sum(nil)[:])
}

type CollaborativeConfig struct {
	ModelFitPeriod        time.Duration       `mapstructure:"model_fit_period" validate:"gt=0"`
	ModelSearchPeriod     time.Duration       `mapstructure:"model_search_period" validate:"gt=0"`
	ModelSearchEpoch      int                 `mapstructure:"model_search_epoch" validate:"gt=0"`
	ModelSearchTrials     int                 `mapstructure:"model_search_trials" validate:"gt=0"`
	EnableModelSizeSearch bool                `mapstructure:"enable_model_size_search"`
	EarlyStopping         EarlyStoppingConfig `mapstructure:"early_stopping"`
}

func (config *CollaborativeConfig) FullName() string {
	return "collaborative"
}

func (config *CollaborativeConfig) Hash(cfg *RecommendConfig) string {
	hash := md5.New()
	for _, expr := range cfg.DataSource.PositiveFeedbackTypes {
		hash.Write([]byte(expr.String()))
	}
	return hex.EncodeToString(hash.Sum(nil)[:])
}

type EarlyStoppingConfig struct {
	Patience int `mapstructure:"patience"`
}

type ExternalConfig struct {
	Name   string `mapstructure:"name" json:"name"`
	Script string `mapstructure:"script" json:"script"`
}

func (config *ExternalConfig) FullName() string {
	return "external/" + config.Name
}

func (config *ExternalConfig) Hash() string {
	hash := md5.New()
	hash.Write([]byte(config.Name))
	hash.Write([]byte(config.Script))
	return hex.EncodeToString(hash.Sum(nil)[:])
}

type ReplacementConfig struct {
	EnableReplacement        bool    `mapstructure:"enable_replacement"`
	PositiveReplacementDecay float64 `mapstructure:"positive_replacement_decay" validate:"gt=0"`
	ReadReplacementDecay     float64 `mapstructure:"read_replacement_decay" validate:"gt=0"`
}

type RankerConfig struct {
	CheckRecommendPeriod   time.Duration       `mapstructure:"check_recommend_period" validate:"gt=0"`
	RefreshRecommendPeriod time.Duration       `mapstructure:"refresh_recommend_period" validate:"gt=0"`
	Recommenders           []string            `mapstructure:"recommenders"`
	EarlyStopping          EarlyStoppingConfig `mapstructure:"early_stopping"`
}

type FallbackConfig struct {
	Recommenders []string `mapstructure:"recommenders"`
}

type TracingConfig struct {
	EnableTracing     bool    `mapstructure:"enable_tracing"`
	Exporter          string  `mapstructure:"exporter" validate:"oneof=jaeger zipkin otlp otlphttp"`
	CollectorEndpoint string  `mapstructure:"collector_endpoint"`
	Sampler           string  `mapstructure:"sampler"`
	Ratio             float64 `mapstructure:"ratio"`
}

type OIDCConfig struct {
	Enable       bool   `mapstructure:"enable"`
	Issuer       string `mapstructure:"issuer"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url" validate:"omitempty,endswith=/callback/oauth2"`
}

type OpenAIConfig struct {
	BaseURL             string `mapstructure:"base_url"`
	AuthToken           string `mapstructure:"auth_token"`
	ChatCompletionModel string `mapstructure:"chat_completion_model"`
	ChatCompletionRPM   int    `mapstructure:"chat_completion_rpm"`
	ChatCompletionTPM   int    `mapstructure:"chat_completion_tpm"`
	EmbeddingModel      string `mapstructure:"embedding_model"`
	EmbeddingDimensions int    `mapstructure:"embedding_dimensions"`
	EmbeddingRPM        int    `mapstructure:"embedding_rpm"`
	EmbeddingTPM        int    `mapstructure:"embedding_tpm"`
	LogFile             string `mapstructure:"log_file"`
}

type S3Config struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	Bucket          string `mapstructure:"bucket"`
	Prefix          string `mapstructure:"prefix"`
}

func (s *S3Config) ToJSON() string {
	return string(lo.Must1(json.Marshal(s)))
}

func GetDefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			MySQL: MySQLConfig{
				IsolationLevel: "READ-UNCOMMITTED",
			},
		},
		Master: MasterConfig{
			Port:            8086,
			Host:            "0.0.0.0",
			HttpPort:        8088,
			HttpHost:        "0.0.0.0",
			HttpCorsDomains: []string{".*"},
			HttpCorsMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
			NumJobs:         1,
			MetaTimeout:     10 * time.Second,
		},
		Server: ServerConfig{
			DefaultN:       10,
			ClockError:     5 * time.Second,
			AutoInsertUser: true,
			AutoInsertItem: true,
			CacheExpire:    10 * time.Second,
		},
		Recommend: RecommendConfig{
			CacheSize:   100,
			CacheExpire: 72 * time.Hour,
			ContextSize: 100,
			Collaborative: CollaborativeConfig{
				ModelFitPeriod:    60 * time.Minute,
				ModelSearchPeriod: 180 * time.Minute,
				ModelSearchEpoch:  100,
				ModelSearchTrials: 10,
			},
			Replacement: ReplacementConfig{
				EnableReplacement:        false,
				PositiveReplacementDecay: 0.8,
				ReadReplacementDecay:     0.6,
			},
			Ranker: RankerConfig{
				CheckRecommendPeriod:   time.Minute,
				RefreshRecommendPeriod: 120 * time.Hour,
			},
			Fallback: FallbackConfig{
				Recommenders: []string{"latest"},
			},
		},
		Tracing: TracingConfig{
			Exporter: "jaeger",
			Sampler:  "always",
		},
	}
}

//go:embed config.toml
var ConfigTOML string

func (config *Config) Now() *time.Time {
	return lo.ToPtr(time.Now().Add(config.Server.ClockError))
}

func (config *TracingConfig) NewTracerProvider() (trace.TracerProvider, error) {
	if !config.EnableTracing {
		return trace.NewNoopTracerProvider(), nil
	}

	var exporter tracesdk.SpanExporter
	var err error
	switch config.Exporter {
	case "jaeger":
		exporter, err = jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.CollectorEndpoint)))
		if err != nil {
			return nil, errors.Trace(err)
		}
	case "zipkin":
		exporter, err = zipkin.New(config.CollectorEndpoint)
		if err != nil {
			return nil, errors.Trace(err)
		}
	case "otlp":
		client := otlptracegrpc.NewClient(otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(config.CollectorEndpoint))
		exporter, err = otlptrace.New(context.TODO(), client)
		if err != nil {
			return nil, errors.Trace(err)
		}
	case "otlphttp":
		client := otlptracehttp.NewClient(otlptracehttp.WithInsecure(), otlptracehttp.WithEndpoint(config.CollectorEndpoint))
		exporter, err = otlptrace.New(context.TODO(), client)
		if err != nil {
			return nil, errors.Trace(err)
		}
	default:
		return nil, errors.NotSupportedf("exporter %s", config.Exporter)
	}

	var sampler tracesdk.Sampler
	switch config.Sampler {
	case "always":
		sampler = tracesdk.AlwaysSample()
	case "never":
		sampler = tracesdk.NeverSample()
	case "ratio":
		sampler = tracesdk.TraceIDRatioBased(config.Ratio)
	default:
		return nil, errors.NotSupportedf("sampler %s", config.Sampler)
	}

	return tracesdk.NewTracerProvider(
		tracesdk.WithSampler(sampler),
		tracesdk.WithBatcher(exporter),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("gorse"),
		)),
	), nil
}

func (config *TracingConfig) Equal(other TracingConfig) bool {
	if config == nil {
		return false
	}
	return config.EnableTracing == other.EnableTracing &&
		config.Exporter == other.Exporter &&
		config.CollectorEndpoint == other.CollectorEndpoint &&
		config.Sampler == other.Sampler &&
		config.Ratio == other.Ratio
}

func setDefault() {
	defaultConfig := GetDefaultConfig()
	// [database.mysql]
	viper.SetDefault("database.mysql.isolation_level", defaultConfig.Database.MySQL.IsolationLevel)
	// [master]
	viper.SetDefault("master.port", defaultConfig.Master.Port)
	viper.SetDefault("master.host", defaultConfig.Master.Host)
	viper.SetDefault("master.http_port", defaultConfig.Master.HttpPort)
	viper.SetDefault("master.http_host", defaultConfig.Master.HttpHost)
	viper.SetDefault("master.http_cors_domains", defaultConfig.Master.HttpCorsDomains)
	viper.SetDefault("master.http_cors_methods", defaultConfig.Master.HttpCorsMethods)
	viper.SetDefault("master.n_jobs", defaultConfig.Master.NumJobs)
	viper.SetDefault("master.meta_timeout", defaultConfig.Master.MetaTimeout)
	// [server]
	viper.SetDefault("server.api_key", defaultConfig.Server.APIKey)
	viper.SetDefault("server.default_n", defaultConfig.Server.DefaultN)
	viper.SetDefault("server.clock_error", defaultConfig.Server.ClockError)
	viper.SetDefault("server.auto_insert_user", defaultConfig.Server.AutoInsertUser)
	viper.SetDefault("server.auto_insert_item", defaultConfig.Server.AutoInsertItem)
	viper.SetDefault("server.cache_expire", defaultConfig.Server.CacheExpire)
	// [recommend]
	viper.SetDefault("recommend.cache_size", defaultConfig.Recommend.CacheSize)
	viper.SetDefault("recommend.cache_expire", defaultConfig.Recommend.CacheExpire)
	viper.SetDefault("recommend.context_size", defaultConfig.Recommend.ContextSize)
	// [recommend.collaborative]
	viper.SetDefault("recommend.collaborative.model_fit_period", defaultConfig.Recommend.Collaborative.ModelFitPeriod)
	viper.SetDefault("recommend.collaborative.model_search_period", defaultConfig.Recommend.Collaborative.ModelSearchPeriod)
	viper.SetDefault("recommend.collaborative.model_search_epoch", defaultConfig.Recommend.Collaborative.ModelSearchEpoch)
	viper.SetDefault("recommend.collaborative.model_search_trials", defaultConfig.Recommend.Collaborative.ModelSearchTrials)
	// [recommend.replacement]
	viper.SetDefault("recommend.replacement.enable_replacement", defaultConfig.Recommend.Replacement.EnableReplacement)
	viper.SetDefault("recommend.replacement.positive_replacement_decay", defaultConfig.Recommend.Replacement.PositiveReplacementDecay)
	viper.SetDefault("recommend.replacement.read_replacement_decay", defaultConfig.Recommend.Replacement.ReadReplacementDecay)
	// [recommend.ranker]
	viper.SetDefault("recommend.ranker.check_recommend_period", defaultConfig.Recommend.Ranker.CheckRecommendPeriod)
	viper.SetDefault("recommend.ranker.refresh_recommend_period", defaultConfig.Recommend.Ranker.RefreshRecommendPeriod)
	// [recommend.fallback]
	viper.SetDefault("recommend.fallback", defaultConfig.Recommend.Fallback)
	// [tracing]
	viper.SetDefault("tracing.exporter", defaultConfig.Tracing.Exporter)
	viper.SetDefault("tracing.sampler", defaultConfig.Tracing.Sampler)
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
		{"database.table_prefix", "GORSE_TABLE_PREFIX"},
		{"database.cache_table_prefix", "GORSE_CACHE_TABLE_PREFIX"},
		{"database.data_table_prefix", "GORSE_DATA_TABLE_PREFIX"},
		{"master.port", "GORSE_MASTER_PORT"},
		{"master.host", "GORSE_MASTER_HOST"},
		{"master.ssl_mode", "GORSE_MASTER_SSL_MODE"},
		{"master.ssl_ca", "GORSE_MASTER_SSL_CA"},
		{"master.ssl_cert", "GORSE_MASTER_SSL_CERT"},
		{"master.ssl_key", "GORSE_MASTER_SSL_KEY"},
		{"master.http_port", "GORSE_MASTER_HTTP_PORT"},
		{"master.http_host", "GORSE_MASTER_HTTP_HOST"},
		{"master.n_jobs", "GORSE_MASTER_JOBS"},
		{"master.dashboard_user_name", "GORSE_DASHBOARD_USER_NAME"},
		{"master.dashboard_password", "GORSE_DASHBOARD_PASSWORD"},
		{"master.dashboard_auth_server", "GORSE_DASHBOARD_AUTH_SERVER"},
		{"master.dashboard_redacted", "GORSE_DASHBOARD_REDACTED"},
		{"master.admin_api_key", "GORSE_ADMIN_API_KEY"},
		{"server.api_key", "GORSE_SERVER_API_KEY"},
		{"oidc.enable", "GORSE_OIDC_ENABLE"},
		{"oidc.issuer", "GORSE_OIDC_ISSUER"},
		{"oidc.client_id", "GORSE_OIDC_CLIENT_ID"},
		{"oidc.client_secret", "GORSE_OIDC_CLIENT_SECRET"},
		{"oidc.redirect_url", "GORSE_OIDC_REDIRECT_URL"},
	}
	for _, binding := range bindings {
		err := viper.BindEnv(binding.key, binding.env)
		if err != nil {
			log.Logger().Fatal("failed to bind a Viper key to a ENV variable", zap.Error(err))
		}
	}

	// check if file exist
	if _, err := os.Stat(path); err != nil {
		return nil, errors.Trace(err)
	}

	// load config file
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		return nil, errors.Trace(err)
	}

	// unmarshal config file
	var conf Config
	if err := viper.Unmarshal(&conf); err != nil {
		return nil, errors.Trace(err)
	}

	// validate config file
	if err := conf.Validate(); err != nil {
		return nil, errors.Trace(err)
	}

	// apply table prefix
	if conf.Database.CacheTablePrefix == "" {
		conf.Database.CacheTablePrefix = conf.Database.TablePrefix
	}
	if conf.Database.DataTablePrefix == "" {
		conf.Database.DataTablePrefix = conf.Database.TablePrefix
	}
	return &conf, nil
}

func (config *Config) Validate() error {
	// Check non-personalized recommenders
	nonPersonalizedNames := mapset.NewSet[string]()
	for _, nonPersonalized := range config.Recommend.NonPersonalized {
		if nonPersonalizedNames.Contains(nonPersonalized.Name) {
			return errors.Errorf("non-personalized recommender %v is duplicated", nonPersonalized.Name)
		}
		nonPersonalizedNames.Add(nonPersonalized.Name)
	}

	// Check item-to-item recommenders
	itemToItemNames := mapset.NewSet[string]()
	for _, itemToItem := range config.Recommend.ItemToItem {
		if itemToItemNames.Contains(itemToItem.Name) {
			return errors.Errorf("item-to-item recommender %v is duplicated", itemToItem.Name)
		}
		itemToItemNames.Add(itemToItem.Name)
	}

	validate := validator.New()
	if err := validate.RegisterValidation("data_store", func(fl validator.FieldLevel) bool {
		prefixes := []string{
			storage.MongoPrefix,
			storage.MongoSrvPrefix,
			storage.MySQLPrefix,
			storage.PostgresPrefix,
			storage.PostgreSQLPrefix,
			storage.ClickhousePrefix,
			storage.CHHTTPPrefix,
			storage.CHHTTPSPrefix,
			storage.SQLitePrefix,
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(fl.Field().String(), prefix) {
				return true
			}
		}
		return false
	}); err != nil {
		return errors.Trace(err)
	}
	if err := validate.RegisterValidation("cache_store", func(fl validator.FieldLevel) bool {
		prefixes := []string{
			storage.RedisPrefix,
			storage.RedissPrefix,
			storage.MongoPrefix,
			storage.MongoSrvPrefix,
			storage.MySQLPrefix,
			storage.PostgresPrefix,
			storage.PostgreSQLPrefix,
			storage.SQLitePrefix,
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(fl.Field().String(), prefix) {
				return true
			}
		}
		return false
	}); err != nil {
		return errors.Trace(err)
	}
	if err := validate.RegisterValidation("item_expr", func(fl validator.FieldLevel) bool {
		if fl.Field().String() == "" {
			// Empty expression is legal.
			return true
		}
		_, err := parser.Parse(fl.Field().String())
		return err == nil
	}); err != nil {
		return errors.Trace(err)
	}
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		return strings.SplitN(fld.Tag.Get("mapstructure"), ",", 2)[0]
	})
	err := validate.Struct(config)
	if err != nil {
		// translate errors
		trans := ut.New(en.New()).GetFallback()
		if err := en_translations.RegisterDefaultTranslations(validate, trans); err != nil {
			return errors.Trace(err)
		}
		if err := validate.RegisterTranslation("data_store", trans, func(ut ut.Translator) error {
			return ut.Add("data_store", "unsupported data storage backend", true) // see universal-translator for details
		}, func(ut ut.Translator, fe validator.FieldError) string {
			t, _ := ut.T("data_store", fe.Field())
			return t
		}); err != nil {
			return errors.Trace(err)
		}
		if err := validate.RegisterTranslation("cache_store", trans, func(ut ut.Translator) error {
			return ut.Add("cache_store", "unsupported cache storage backend", true) // see universal-translator for details
		}, func(ut ut.Translator, fe validator.FieldError) string {
			t, _ := ut.T("cache_store", fe.Field())
			return t
		}); err != nil {
			return errors.Trace(err)
		}
		if err := validate.RegisterTranslation("item_expr", trans, func(ut ut.Translator) error {
			return ut.Add("item_expr", "invalid item expression", true)
		}, func(ut ut.Translator, fe validator.FieldError) string {
			t, _ := ut.T("item_expr", fe.Field())
			return t
		}); err != nil {
			return errors.Trace(err)
		}
		errs := err.(validator.ValidationErrors)
		for _, e := range errs {
			return errors.New(e.Translate(trans))
		}
	}
	return nil
}
