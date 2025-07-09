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

package cache

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/araddon/dateparse"
	"github.com/juju/errors"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidisotel"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/storage"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
	semconv "go.opentelemetry.io/otel/semconv/v1.8.0"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"moul.io/zapgorm2"
)

const (
	CollaborativeFiltering = "collaborative_recommend"

	// OfflineRecommend is sorted set of offline recommendation for each user.
	//  Global recommendation      - offline_recommend/{user_id}
	//  Categorized recommendation - offline_recommend/{user_id}/{category}
	OfflineRecommend           = "offline_recommend"
	OfflineRecommendUpdateTime = "offline_recommend_update_time"

	// OfflineRecommendDigest is digest of offline recommendation configuration.
	//	Recommendation digest      - offline_recommend_digest/{user_id}
	OfflineRecommendDigest = "offline_recommend_digest"

	NonPersonalized           = "non-personalized"
	NonPersonalizedUpdateTime = "non-personalized_update_time"
	Latest                    = "latest"
	Popular                   = "popular"

	ItemToItem           = "item-to-item"
	ItemToItemDigest     = "item-to-item_digest"
	ItemToItemUpdateTime = "item-to-item_update_time"
	UserToUser           = "user-to-user"
	UserToUserDigest     = "user-to-user_digest"
	UserToUserUpdateTime = "user-to-user_update_time"

	// ItemCategories is the set of item categories. The format of key:
	//	Global item categories - item_categories
	ItemCategories = "item_categories"

	LastModifyItemTime          = "last_modify_item_time"           // the latest timestamp that a user related data was modified
	LastModifyUserTime          = "last_modify_user_time"           // the latest timestamp that an item related data was modified
	LastUpdateUserRecommendTime = "last_update_user_recommend_time" // the latest timestamp that a user's recommendation was updated

	// GlobalMeta is global meta information
	GlobalMeta                 = "global_meta"
	DataImported               = "data_imported"
	NumUsers                   = "num_users"
	NumItems                   = "num_items"
	NumUserLabels              = "num_user_labels"
	NumItemLabels              = "num_item_labels"
	NumTotalPosFeedbacks       = "num_total_pos_feedbacks"
	NumValidPosFeedbacks       = "num_valid_pos_feedbacks"
	NumValidNegFeedbacks       = "num_valid_neg_feedbacks"
	LastFitMatchingModelTime   = "last_fit_matching_model_time"
	LastFitRankingModelTime    = "last_fit_ranking_model_time"
	LastUpdateLatestItemsTime  = "last_update_latest_items_time"  // the latest timestamp that latest items were updated
	LastUpdatePopularItemsTime = "last_update_popular_items_time" // the latest timestamp that popular items were updated
	MatchingIndexRecall        = "matching_index_recall"
)

var ItemCache = []string{
	NonPersonalized,
	ItemToItem,
	OfflineRecommend,
}

var (
	ErrObjectNotExist = errors.NotFoundf("object")
	ErrNoDatabase     = errors.NotAssignedf("database")
)

// Key creates key for cache. Empty field will be ignored.
func Key(keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(keys[0])
	for _, key := range keys[1:] {
		if key != "" {
			builder.WriteRune('/')
			builder.WriteString(key)
		}
	}
	return builder.String()
}

type Value struct {
	name  string
	value string
}

func String(name, value string) Value {
	return Value{name: name, value: value}
}

func Integer(name string, value int) Value {
	return Value{name: name, value: strconv.Itoa(value)}
}

func Time(name string, value time.Time) Value {
	return Value{name: name, value: value.String()}
}

type ReturnValue struct {
	value string
	err   error
}

func (r *ReturnValue) String() (string, error) {
	return r.value, r.err
}

func (r *ReturnValue) Integer() (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	return strconv.Atoi(r.value)
}

func (r *ReturnValue) Time() (time.Time, error) {
	if r.err != nil {
		return time.Time{}, r.err
	}
	t, err := dateparse.ParseAny(r.value)
	if err != nil {
		return time.Time{}, errors.Trace(err)
	}
	return t.In(time.UTC), nil
}

type Score struct {
	Id         string
	Score      float64
	IsHidden   bool      `json:"-"`
	Categories []string  `json:"-" gorm:"type:text;serializer:json"`
	Timestamp  time.Time `json:"-"`
}

func SortDocuments(documents []Score) {
	sort.Slice(documents, func(i, j int) bool {
		return documents[i].Score > documents[j].Score
	})
}

func ConvertDocumentsToValues(documents []Score) []string {
	values := make([]string, len(documents))
	for i := range values {
		values[i] = documents[i].Id
	}
	return values
}

// DocumentAggregator is used to keep the compatibility with the old recommender system and will be removed in the future.
// In old recommender system, the recommendation is genereated per category.
// In the new recommender system, the recommendation is generated globally.
type DocumentAggregator struct {
	Documents map[string]*Score
	Timestamp time.Time
}

func NewDocumentAggregator(timestamp time.Time) *DocumentAggregator {
	return &DocumentAggregator{
		Documents: make(map[string]*Score),
		Timestamp: timestamp,
	}
}

func (aggregator *DocumentAggregator) Add(category string, values []string, scores []float64) {
	for i, value := range values {
		if _, ok := aggregator.Documents[value]; !ok {
			aggregator.Documents[value] = &Score{
				Id:         value,
				Score:      scores[i],
				Categories: []string{category},
				Timestamp:  aggregator.Timestamp,
			}
		} else {
			aggregator.Documents[value].Score = math.Max(aggregator.Documents[value].Score, scores[i])
			aggregator.Documents[value].Categories = append(aggregator.Documents[value].Categories, category)
		}
	}
}

func (aggregator *DocumentAggregator) ToSlice() []Score {
	documents := make([]Score, 0, len(aggregator.Documents))
	for _, document := range aggregator.Documents {
		sort.Strings(document.Categories)
		documents = append(documents, *document)
	}
	return documents
}

type ScoreCondition struct {
	Subset *string
	Id     *string
	Before *time.Time
}

func (condition *ScoreCondition) Check() error {
	if condition.Id == nil && condition.Before == nil && condition.Subset == nil {
		return errors.NotValidf("document condition")
	}
	return nil
}

type ScorePatch struct {
	IsHidden   *bool
	Categories []string
	Score      *float64
}

type TimeSeriesPoint struct {
	Name      string    `gorm:"primaryKey"`
	Timestamp time.Time `gorm:"primaryKey"`
	Value     float64
}

// Database is the common interface for cache store.
type Database interface {
	Close() error
	Ping() error
	Init() error
	Scan(work func(string) error) error
	Purge() error

	Set(ctx context.Context, values ...Value) error
	Get(ctx context.Context, name string) *ReturnValue
	Delete(ctx context.Context, name string) error

	GetSet(ctx context.Context, key string) ([]string, error)
	SetSet(ctx context.Context, key string, members ...string) error
	AddSet(ctx context.Context, key string, members ...string) error
	RemSet(ctx context.Context, key string, members ...string) error

	Push(ctx context.Context, name, value string) error
	Pop(ctx context.Context, name string) (string, error)
	Remain(ctx context.Context, name string) (int64, error)

	AddScores(ctx context.Context, collection, subset string, documents []Score) error
	SearchScores(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Score, error)
	DeleteScores(ctx context.Context, collection []string, condition ScoreCondition) error
	UpdateScores(ctx context.Context, collections []string, subset *string, id string, patch ScorePatch) error
	ScanScores(ctx context.Context, callback func(collection, id, subset string, timestamp time.Time) error) error

	AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error
	GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time, duration time.Duration) ([]TimeSeriesPoint, error)
}

// Open a connection to a database.
func Open(path, tablePrefix string, opts ...storage.Option) (Database, error) {
	var err error
	if strings.HasPrefix(path, storage.RedisPrefix) || strings.HasPrefix(path, storage.RedissPrefix) ||
		strings.HasPrefix(path, storage.RedisClusterPrefix) || strings.HasPrefix(path, storage.RedissClusterPrefix) {
		// rueidis treat cluster and normal client as the same, so replace all cluster prefix with the normal one
		newURL := path
		if strings.HasPrefix(path, storage.RedisClusterPrefix) {
			newURL = strings.Replace(path, storage.RedisClusterPrefix, storage.RedisPrefix, 1)
		} else if strings.HasPrefix(path, storage.RedissClusterPrefix) {
			newURL = strings.Replace(path, storage.RedissClusterPrefix, storage.RedissPrefix, 1)
		}
		if strings.HasSuffix(newURL, "/") {
			// rueidis not allow empty db in the path, so we have to add a default one to make old conf happy
			newURL = newURL + "0"
		}
		opt, err := rueidis.ParseURL(newURL)
		if err != nil {
			return nil, err
		}
		database := new(Redis)
		database.client, err = rueidis.NewClient(opt)
		if err != nil {
			return nil, err
		}
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		database.client = rueidisotel.WithClient(database.client, rueidisotel.TraceAttrs(semconv.DBSystemRedis))
		return database, nil
	} else if strings.HasPrefix(path, storage.MongoPrefix) || strings.HasPrefix(path, storage.MongoSrvPrefix) {
		// connect to database
		database := new(MongoDB)
		opts := options.Client()
		opts.Monitor = otelmongo.NewMonitor()
		opts.ApplyURI(path)
		if database.client, err = mongo.Connect(context.Background(), opts); err != nil {
			return nil, errors.Trace(err)
		}
		// parse DSN and extract database name
		if cs, err := connstring.ParseAndValidate(path); err != nil {
			return nil, errors.Trace(err)
		} else {
			database.dbName = cs.Database
			database.TablePrefix = storage.TablePrefix(tablePrefix)
		}
		return database, nil
	} else if strings.HasPrefix(path, storage.PostgresPrefix) || strings.HasPrefix(path, storage.PostgreSQLPrefix) {
		database := new(SQLDatabase)
		database.driver = Postgres
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		if database.client, err = otelsql.Open("postgres", path,
			otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
			otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
		); err != nil {
			return nil, errors.Trace(err)
		}
		database.gormDB, err = gorm.Open(postgres.New(postgres.Config{Conn: database.client}), storage.NewGORMConfig(tablePrefix))
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	} else if strings.HasPrefix(path, storage.MySQLPrefix) {
		name := path[len(storage.MySQLPrefix):]
		option := storage.NewOptions(opts...)
		// probe isolation variable name
		isolationVarName, err := storage.ProbeMySQLIsolationVariableName(name)
		if err != nil {
			return nil, errors.Trace(err)
		}
		// append parameters
		if name, err = storage.AppendMySQLParams(name, map[string]string{
			isolationVarName: fmt.Sprintf("'%s'", option.IsolationLevel),
			"parseTime":      "true",
		}); err != nil {
			return nil, errors.Trace(err)
		}
		// connect to database
		database := new(SQLDatabase)
		database.driver = MySQL
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		if database.client, err = otelsql.Open("mysql", name,
			otelsql.WithAttributes(semconv.DBSystemMySQL),
			otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
		); err != nil {
			return nil, errors.Trace(err)
		}
		database.gormDB, err = gorm.Open(mysql.New(mysql.Config{Conn: database.client}), storage.NewGORMConfig(tablePrefix))
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	} else if strings.HasPrefix(path, storage.SQLitePrefix) {
		dataSourceName := path[len(storage.SQLitePrefix):]
		// append parameters
		if dataSourceName, err = storage.AppendURLParams(dataSourceName, []lo.Tuple2[string, string]{
			{"_pragma", "busy_timeout(10000)"},
			{"_pragma", "journal_mode(wal)"},
		}); err != nil {
			return nil, errors.Trace(err)
		}
		// connect to database
		database := new(SQLDatabase)
		database.driver = SQLite
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		if database.client, err = otelsql.Open("sqlite", dataSourceName,
			otelsql.WithAttributes(semconv.DBSystemSqlite),
			otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
		); err != nil {
			return nil, errors.Trace(err)
		}
		gormConfig := storage.NewGORMConfig(tablePrefix)
		gormConfig.Logger = &zapgorm2.Logger{
			ZapLogger:                 log.Logger(),
			LogLevel:                  logger.Warn,
			SlowThreshold:             10 * time.Second,
			SkipCallerLookup:          false,
			IgnoreRecordNotFoundError: false,
		}
		database.gormDB, err = gorm.Open(sqlite.Dialector{Conn: database.client}, gormConfig)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
