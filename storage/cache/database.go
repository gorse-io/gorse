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
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/araddon/dateparse"
	"github.com/gorse-io/gorse/storage"
	"github.com/pkg/errors"
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

// Name returns the name of the value.
func (v Value) Name() string {
	return v.name
}

// Value returns the raw string representation of the value.
func (v Value) Value() string {
	return v.value
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

// NewReturnValue creates a return value with the specified existence state.
func NewReturnValue(value string, exists bool) *ReturnValue {
	return &ReturnValue{value: value, exists: exists}
}

// NewReturnError creates a return value containing an error.
func NewReturnError(err error) *ReturnValue {
	return &ReturnValue{err: err}
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
		return time.Time{}, errors.WithStack(err)
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
		return fmt.Errorf("document condition: %w", storage.ErrInvalidArgument)
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

	AddScores(ctx context.Context, collection, subset string, documents []Score) error
	SearchScores(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Score, error)
	DeleteScores(ctx context.Context, collection []string, condition ScoreCondition) error
	UpdateScores(ctx context.Context, collections []string, subset *string, id string, patch ScorePatch) error
	ScanScores(ctx context.Context, callback func(collection, id, subset string, timestamp time.Time) error) error

	// AddGeoIndex creates an empty, independently stored geographic index.
	AddGeoIndex(ctx context.Context, name string) error
	// ListGeoIndex returns ready index names in byte order, including empty indexes.
	ListGeoIndex(ctx context.Context) ([]string, error)
	AddGeoPoints(ctx context.Context, name string, points []GeoPoint) error
	SearchGeoPoints(ctx context.Context, name string, query GeoQuery) ([]GeoResult, error)
	DeleteGeoPoints(ctx context.Context, name string, ids ...string) error
	DeleteGeoIndex(ctx context.Context, name string) error

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

const (
	geoEarthRadius = 6371008.8 // Mean spherical Earth radius, in meters.
	geoBits        = 26
	geoGridSize    = uint32(1 << geoBits)
	geoMaxCell     = int64(1<<52 - 1)
)

// GeoPoint is a named location in degrees on the full sphere.
// IDs are nonempty UTF-8 strings of at most 256 bytes.
type GeoPoint struct {
	Id                  string
	Longitude, Latitude float64
}

// GeoQuery requests the nearest N points. Distances are in meters;
// MaxDistance == 0 means unlimited, otherwise the boundary is inclusive.
type GeoQuery struct {
	Longitude, Latitude float64
	N                   int
	MaxDistance         float64
}

// GeoResult retains the stored coordinates, with an exact spherical distance.
type GeoResult struct {
	GeoPoint
	Distance float64
}

type geoRecord struct {
	GeoPoint
	Cell int64
}

// geoRange is an inclusive interval in the 52-bit Morton index.
type geoRange struct{ Min, Max int64 }

func validateGeoName(name string) error {
	if name == "" || len(name) > 256 || !utf8.ValidString(name) {
		return fmt.Errorf("geography index name must be nonempty UTF-8 of at most 256 bytes (%w)", storage.ErrInvalidArgument)
	}
	return nil
}

func geoCoordinates(longitude, latitude float64) (float64, float64, error) {
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 ||
		math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
		return 0, 0, fmt.Errorf("geography coordinates (%g, %g) are outside the finite sphere (%w)", longitude, latitude, storage.ErrInvalidArgument)
	}
	if longitude == 180 {
		longitude = -180
	}
	if latitude == -90 || latitude == 90 {
		longitude = 0
	}
	if longitude == 0 {
		longitude = 0 // Canonicalize negative zero.
	}
	if latitude == 0 {
		latitude = 0
	}
	return longitude, latitude, nil
}

// prepareGeoIDs validates the entire delete batch before any backend mutation.
func prepareGeoIDs(name string, ids []string) ([]string, error) {
	if err := validateGeoName(name); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || len(id) > 256 || !utf8.ValidString(id) {
			return nil, fmt.Errorf("geography point ID must be nonempty UTF-8 of at most 256 bytes (%w)", storage.ErrInvalidArgument)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result, nil
}

// prepareGeoPoints validates the entire batch before returning any records.
// Duplicate IDs use the last point, preserving the order of last occurrences.
func prepareGeoPoints(name string, points []GeoPoint) ([]geoRecord, error) {
	if err := validateGeoName(name); err != nil {
		return nil, err
	}
	records := make([]geoRecord, len(points))
	last := make(map[string]int, len(points))
	for i, point := range points {
		if point.Id == "" || len(point.Id) > 256 || !utf8.ValidString(point.Id) {
			return nil, fmt.Errorf("geography point %d ID must be nonempty UTF-8 of at most 256 bytes (%w)", i, storage.ErrInvalidArgument)
		}
		longitude, latitude, err := geoCoordinates(point.Longitude, point.Latitude)
		if err != nil {
			return nil, fmt.Errorf("geography point %q (%w)", point.Id, err)
		}
		point.Longitude, point.Latitude = longitude, latitude
		records[i] = geoRecord{GeoPoint: point, Cell: geoMorton(longitude, latitude)}
		last[point.Id] = i
	}
	out := records[:0]
	for i, record := range records {
		if last[record.Id] == i {
			out = append(out, record)
		}
	}
	return out, nil
}

func normalizeGeoQuery(name string, q GeoQuery) (GeoQuery, error) {
	if err := validateGeoName(name); err != nil {
		return GeoQuery{}, err
	}
	longitude, latitude, err := geoCoordinates(q.Longitude, q.Latitude)
	if err != nil {
		return GeoQuery{}, err
	}
	if q.N < 0 || math.IsNaN(q.MaxDistance) || math.IsInf(q.MaxDistance, 0) || q.MaxDistance < 0 {
		return GeoQuery{}, fmt.Errorf("geography query requires nonnegative N and finite nonnegative maximum distance (%w)", storage.ErrInvalidArgument)
	}
	q.Longitude, q.Latitude = longitude, latitude
	return q, nil
}

// Length framing prevents aliases such as ("ab", "c") and ("a", "bc").
func geoNameDigest(parts ...string) string {
	h := sha256.New()
	var size [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(size[:], uint64(len(part)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(part))
	}
	return hex.EncodeToString(h.Sum(nil)[:28])
}

func geoPhysicalName(prefix, name string) string { return "ggeo_" + geoNameDigest(prefix, name) }
func geoCatalogName(prefix string) string        { return "gcat_" + geoNameDigest(prefix) }

func geoQuantize(value, minimum, width float64) uint32 {
	v := (value - minimum) / width * float64(geoGridSize)
	if v <= 0 {
		return 0
	}
	if v >= float64(geoGridSize) {
		return geoGridSize - 1
	}
	return uint32(v)
}

// Longitude supplies the first (most significant) bit of each Morton pair.
func geoMorton(longitude, latitude float64) int64 {
	x, y := geoQuantize(longitude, -180, 360), geoQuantize(latitude, -90, 180)
	var cell int64
	for bit := geoBits - 1; bit >= 0; bit-- {
		cell = cell<<2 | int64((x>>bit&1)<<1|(y>>bit&1))
	}
	return cell
}

type geoRectangle struct{ minX, maxX, minY, maxY uint32 }

// geoCapBounds conservatively bounds a spherical cap, splitting at the dateline.
// A small outward rounding guard protects inclusive boundaries from trig error.
func geoCapBounds(q GeoQuery, radius float64) []geoRectangle {
	angle := radius / geoEarthRadius
	if angle >= math.Pi {
		return []geoRectangle{{0, geoGridSize - 1, 0, geoGridSize - 1}}
	}
	const guard = 1e-10 // Radians; far smaller than an index cell.
	angle = math.Min(math.Pi, angle+guard)
	lat := q.Latitude * math.Pi / 180
	low, high := math.Max(-math.Pi/2, lat-angle), math.Min(math.Pi/2, lat+angle)
	minY := geoQuantize(low*180/math.Pi, -90, 180)
	maxY := geoQuantize(high*180/math.Pi, -90, 180)
	if low <= -math.Pi/2 || high >= math.Pi/2 {
		return []geoRectangle{{0, geoGridSize - 1, minY, maxY}}
	}
	delta := (math.Asin(math.Min(1, math.Sin(angle)/math.Cos(lat))) + guard) * 180 / math.Pi
	west, east := q.Longitude-delta, q.Longitude+delta
	if west < -180 {
		return []geoRectangle{{0, geoQuantize(east, -180, 360), minY, maxY}, {geoQuantize(west+360, -180, 360), geoGridSize - 1, minY, maxY}}
	}
	if east >= 180 {
		return []geoRectangle{{geoQuantize(west, -180, 360), geoGridSize - 1, minY, maxY}, {0, geoQuantize(east-360, -180, 360), minY, maxY}}
	}
	return []geoRectangle{{geoQuantize(west, -180, 360), geoQuantize(east, -180, 360), minY, maxY}}
}

type geoCoverNode struct {
	x, y, size uint32
	prefix     int64
	depth      uint
}

// geoCover visits at most 125 quadtree nodes (below the budget of 128).
// When refinement runs out of budget it retains coarse intersecting parents;
// it never drops a portion of the cap or enumerates a fine-resolution grid.
func geoCover(q GeoQuery, radius float64) []geoRange {
	rectangles := geoCapBounds(q, radius)
	nodes := []geoCoverNode{{size: geoGridSize}}
	visited := 1
	var ranges []geoRange
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		intersects, contained := false, false
		for _, rectangle := range rectangles {
			maxX, maxY := node.x+node.size-1, node.y+node.size-1
			if node.x <= rectangle.maxX && maxX >= rectangle.minX && node.y <= rectangle.maxY && maxY >= rectangle.minY {
				intersects = true
			}
			if node.x >= rectangle.minX && maxX <= rectangle.maxX && node.y >= rectangle.minY && maxY <= rectangle.maxY {
				contained = true
			}
		}
		if !intersects {
			continue
		}
		if contained || node.size == 1 || visited+4 > 128 {
			shift := 2 * (geoBits - node.depth)
			minimum := node.prefix << shift
			ranges = append(ranges, geoRange{minimum, minimum + (int64(1) << shift) - 1})
			continue
		}
		half := node.size / 2
		for child := 0; child < 4; child++ {
			nodes = append(nodes, geoCoverNode{x: node.x + uint32(child>>1)*half, y: node.y + uint32(child&1)*half, size: half, prefix: node.prefix<<2 | int64(child), depth: node.depth + 1})
		}
		visited += 4
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].Min < ranges[j].Min })
	merged := ranges[:0]
	for _, interval := range ranges {
		if len(merged) != 0 && interval.Min <= merged[len(merged)-1].Max+1 {
			merged[len(merged)-1].Max = max(merged[len(merged)-1].Max, interval.Max)
		} else {
			merged = append(merged, interval)
		}
	}
	return merged
}

func geoDistance(q GeoQuery, p GeoPoint) float64 {
	lat1, lat2 := q.Latitude*math.Pi/180, p.Latitude*math.Pi/180
	dlat, dlon := (p.Latitude-q.Latitude)*math.Pi/180, (p.Longitude-q.Longitude)*math.Pi/180
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dlon/2)*math.Sin(dlon/2)
	a = math.Max(0, math.Min(1, a))
	// Keep rounding at an antipode within the same whole-globe limit used
	// by the radius expansion, so the final round cannot omit that point.
	return math.Min(math.Pi*geoEarthRadius, 2*geoEarthRadius*math.Atan2(math.Sqrt(a), math.Sqrt(1-a)))
}

func geoResultLess(a, b GeoResult) bool {
	if a.Distance != b.Distance {
		return a.Distance < b.Distance
	}
	return a.Id < b.Id
}

// This max-heap and its positions map hold only the current top N IDs.
// Duplicate observations from concurrently moving records cannot duplicate IDs
// in the result. There is deliberately no snapshot guarantee during writes.
type geoResultHeap struct {
	results   []GeoResult
	positions map[string]int
}

func (h geoResultHeap) Len() int           { return len(h.results) }
func (h geoResultHeap) Less(i, j int) bool { return geoResultLess(h.results[j], h.results[i]) }
func (h geoResultHeap) Swap(i, j int) {
	h.results[i], h.results[j] = h.results[j], h.results[i]
	h.positions[h.results[i].Id], h.positions[h.results[j].Id] = i, j
}
func (h *geoResultHeap) Push(value any) {
	result := value.(GeoResult)
	h.positions[result.Id] = len(h.results)
	h.results = append(h.results, result)
}
func (h *geoResultHeap) Pop() any {
	i := len(h.results) - 1
	result := h.results[i]
	h.results[i] = GeoResult{}
	h.results = h.results[:i]
	delete(h.positions, result.Id)
	return result
}

// searchGeoPoints requires a validated, canonical query. The caller must check
// index existence even for N == 0. scan must exhaust every supplied inclusive
// range, use bounded pages, propagate callback errors, and honor cancellation.
// Each round resets the top-N heap and scans the entire conservative cover;
// coarse-cell false positives never count toward the radius stopping condition.
// Static data yields exact nearest neighbors, ordered by distance then bytewise ID.
func searchGeoPoints(ctx context.Context, q GeoQuery, scan func(context.Context, []geoRange, func(geoRecord) error) error) ([]GeoResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if q.N == 0 {
		return []GeoResult{}, nil
	}
	limit := math.Pi * geoEarthRadius
	if q.MaxDistance > 0 {
		limit = math.Min(limit, q.MaxDistance)
	}
	radius := math.Min(1000, limit)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		h := &geoResultHeap{positions: make(map[string]int)}
		err := scan(ctx, geoCover(q, radius), func(record geoRecord) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			distance := geoDistance(q, record.GeoPoint)
			if distance > radius {
				return nil
			}
			result := GeoResult{GeoPoint: record.GeoPoint, Distance: distance}
			if i, ok := h.positions[result.Id]; ok {
				if geoResultLess(result, h.results[i]) {
					h.results[i] = result
					heap.Fix(h, i)
				}
			} else if h.Len() < q.N {
				heap.Push(h, result)
			} else if geoResultLess(result, h.results[0]) {
				delete(h.positions, h.results[0].Id)
				h.results[0] = result
				h.positions[result.Id] = 0
				heap.Fix(h, 0)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if h.Len() >= q.N || radius >= limit {
			sort.Slice(h.results, func(i, j int) bool { return geoResultLess(h.results[i], h.results[j]) })
			if h.results == nil {
				return []GeoResult{}, nil
			}
			return h.results, nil
		}
		radius = math.Min(limit, radius*2)
	}
}
