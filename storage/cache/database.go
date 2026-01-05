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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
)

const (
	NonPersonalized                  = "non-personalized"
	NonPersonalizedDigest            = "non-personalized_digest"
	NonPersonalizedUpdateTime        = "non-personalized_update_time"
	ItemToItem                       = "item-to-item"
	ItemToItemDigest                 = "item-to-item_digest"
	ItemToItemUpdateTime             = "item-to-item_update_time"
	UserToUser                       = "user-to-user"
	UserToUserDigest                 = "user-to-user_digest"
	UserToUserUpdateTime             = "user-to-user_update_time"
	CollaborativeFiltering           = "collaborative-filtering"
	CollaborativeFilteringDigest     = "collaborative-filtering_digest"
	CollaborativeFilteringUpdateTime = "collaborative-filtering_update_time"
	Recommend                        = "recommend"
	RecommendDigest                  = "recommend_digest"
	RecommendUpdateTime              = "recommend_update_time"

	// ItemCategories is the set of item categories. The format of key:
	//	Global item categories - item_categories
	ItemCategories = "item_categories"

	LastModifyItemTime = "last_modify_item_time" // the latest timestamp that a user related data was modified
	LastModifyUserTime = "last_modify_user_time" // the latest timestamp that an item related data was modified

	// GlobalMeta is global meta information
	GlobalMeta                 = "global_meta"
	NumUsers                   = "num_users"
	NumItems                   = "num_items"
	NumFeedback                = "num_feedback"
	NumPosFeedbacks            = "num_pos_feedbacks"
	NumNegFeedbacks            = "num_neg_feedbacks"
	NumUserLabels              = "num_user_labels"
	NumItemLabels              = "num_item_labels"
	NumTotalPosFeedbacks       = "num_total_pos_feedbacks"
	NumValidPosFeedbacks       = "num_valid_pos_feedbacks"
	NumValidNegFeedbacks       = "num_valid_neg_feedbacks"
	LastFitMatchingModelTime   = "last_fit_matching_model_time"
	LastFitRankingModelTime    = "last_fit_ranking_model_time"
	LastUpdateLatestItemsTime  = "last_update_latest_items_time"  // the latest timestamp that latest items were updated
	LastUpdatePopularItemsTime = "last_update_popular_items_time" // the latest timestamp that popular items were updated
	CFNDCG                     = "cf_ndcg"
	CFPrecision                = "cf_precision"
	CFRecall                   = "cf_recall"
	CTRPrecision               = "ctr_precision"
	CTRRecall                  = "ctr_recall"
	CTRAUC                     = "ctr_auc"
	PositiveFeedbackRatio      = "positive_feedback_ratio"
)

var ItemCache = []string{
	NonPersonalized,
	ItemToItem,
	Recommend,
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
	value  string
	err    error
	exists bool
}

func (r *ReturnValue) String() (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if !r.exists {
		return "", nil
	}
	return r.value, nil
}

func (r *ReturnValue) Integer() (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if !r.exists {
		return 0, nil
	}
	return strconv.Atoi(r.value)
}

func (r *ReturnValue) Time() (time.Time, error) {
	if r.err != nil {
		return time.Time{}, r.err
	}
	if !r.exists {
		return time.Time{}, nil
	}
	t, err := dateparse.ParseAny(r.value)
	if err != nil {
		return time.Time{}, errors.Trace(err)
	}
	return t.In(time.UTC), nil
}

func (r *ReturnValue) Exists() bool {
	return r.exists
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

// Creator creates a database instance.
type Creator func(path, tablePrefix string, opts ...storage.Option) (Database, error)

var creators = make(map[string]Creator)

// Register a database creator.
func Register(prefixes []string, creator Creator) {
	for _, p := range prefixes {
		creators[p] = creator
	}
}

// Open a connection to a database.
func Open(path, tablePrefix string, opts ...storage.Option) (Database, error) {
	for prefix, creator := range creators {
		if strings.HasPrefix(path, prefix) {
			return creator(path, tablePrefix, opts...)
		}
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
