// Copyright 2025 gorse Project Authors
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
	"crypto/tls"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/valkey-io/valkey-go"
	"go.uber.org/zap"
)

func init() {
	Register([]string{storage.ValkeyPrefix, storage.ValkeysPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		host, port, username, password, db, useTLS, err := parseValkeyURL(path)
		if err != nil {
			return nil, errors.Trace(err)
		}
		option := valkey.ClientOption{
			InitAddress: []string{fmt.Sprintf("%s:%d", host, port)},
			SelectDB:    db,
		}
		if username != "" || password != "" {
			option.Username = username
			option.Password = password
		}
		if useTLS {
			option.TLSConfig = &tls.Config{}
		}
		client, err := valkey.NewClient(option)
		if err != nil {
			return nil, errors.Trace(err)
		}
		database := &Valkey{
			client:           client,
			isCluster:        false,
			maxSearchResults: storage.NewOptions(opts...).MaxSearchResults,
		}
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		return database, nil
	})
	Register([]string{storage.ValkeyClusterPrefix, storage.ValkeysClusterPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		addresses, username, password, useTLS, err := parseValkeyClusterURL(path)
		if err != nil {
			return nil, errors.Trace(err)
		}
		option := valkey.ClientOption{
			InitAddress: addresses,
		}
		if username != "" || password != "" {
			option.Username = username
			option.Password = password
		}
		if useTLS {
			option.TLSConfig = &tls.Config{}
		}
		client, err := valkey.NewClient(option)
		if err != nil {
			return nil, errors.Trace(err)
		}
		database := &Valkey{
			client:           client,
			isCluster:        true,
			maxSearchResults: storage.NewOptions(opts...).MaxSearchResults,
		}
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		return database, nil
	})
}

// parseValkeyURL parses a valkey:// or valkeys:// URL into connection parameters.
func parseValkeyURL(rawURL string) (host string, port int, username, password string, db int, useTLS bool, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, "", "", 0, false, errors.Trace(err)
	}
	host = parsed.Hostname()
	if host == "" {
		host = "localhost"
	}
	port = 6379
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil {
			return "", 0, "", "", 0, false, errors.Trace(err)
		}
	}
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	if parsed.Path != "" && parsed.Path != "/" {
		dbStr := strings.TrimPrefix(parsed.Path, "/")
		if dbStr != "" {
			db, err = strconv.Atoi(dbStr)
			if err != nil {
				return "", 0, "", "", 0, false, errors.Errorf("invalid database number: %s", dbStr)
			}
		}
	}
	useTLS = parsed.Scheme == "valkeys"
	return host, port, username, password, db, useTLS, nil
}

// parseValkeyClusterURL parses a valkey+cluster:// or valkeys+cluster:// URL.
func parseValkeyClusterURL(rawURL string) (addresses []string, username, password string, useTLS bool, err error) {
	// Replace the cluster prefix with a standard scheme for URL parsing.
	var newURL string
	if strings.HasPrefix(rawURL, storage.ValkeyClusterPrefix) {
		newURL = strings.Replace(rawURL, storage.ValkeyClusterPrefix, storage.ValkeyPrefix, 1)
		useTLS = false
	} else if strings.HasPrefix(rawURL, storage.ValkeysClusterPrefix) {
		newURL = strings.Replace(rawURL, storage.ValkeysClusterPrefix, storage.ValkeysPrefix, 1)
		useTLS = true
	}
	parsed, err := url.Parse(newURL)
	if err != nil {
		return nil, "", "", false, errors.Trace(err)
	}
	host := parsed.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := 6379
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil {
			return nil, "", "", false, errors.Trace(err)
		}
	}
	addresses = append(addresses, fmt.Sprintf("%s:%d", host, port))
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	// Parse additional addresses from query params (addr=host:port).
	for _, addrStr := range parsed.Query()["addr"] {
		if !strings.Contains(addrStr, ":") {
			addrStr = addrStr + ":6379"
		}
		addresses = append(addresses, addrStr)
	}
	return addresses, username, password, useTLS, nil
}

// Valkey cache storage using valkey-go client.
type Valkey struct {
	storage.TablePrefix
	client           valkey.Client
	isCluster        bool
	maxSearchResults int
}

// Close the valkey connection.
func (v *Valkey) Close() error {
	v.client.Close()
	return nil
}

// Ping the valkey server.
func (v *Valkey) Ping() error {
	ctx := context.Background()
	return v.client.Do(ctx, v.client.B().Ping().Build()).Error()
}

// Init creates the valkey-search index for document storage.
func (v *Valkey) Init() error {
	ctx := context.Background()
	// List existing indices via FT._LIST.
	result, err := v.client.Do(ctx, v.client.B().Arbitrary("FT._LIST").Build()).ToArray()
	if err != nil {
		return errors.Trace(err)
	}
	indices := make([]string, 0, len(result))
	for _, r := range result {
		if s, err := r.ToString(); err == nil {
			indices = append(indices, s)
		}
	}
	if lo.Contains(indices, v.DocumentTable()) {
		return nil
	}
	// Create the index.
	err = v.client.Do(ctx, v.client.B().Arbitrary("FT.CREATE").
		Keys(v.DocumentTable()).
		Args(
			"ON", "HASH",
			"PREFIX", "1", v.DocumentTable()+":",
			"SCHEMA",
			"collection", "TAG",
			"subset", "TAG",
			"id", "TAG",
			"score", "NUMERIC",
			"is_hidden", "NUMERIC",
			"categories", "TAG", "SEPARATOR", ";",
			"timestamp", "NUMERIC",
		).Build()).Error()
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// Scan iterates over all keys with the table prefix.
func (v *Valkey) Scan(work func(string) error) error {
	ctx := context.Background()
	var cursor uint64
	for {
		entry, err := v.client.Do(ctx, v.client.B().Scan().Cursor(cursor).Match(string(v.TablePrefix)+"*").Count(100).Build()).AsScanEntry()
		if err != nil {
			return errors.Trace(err)
		}
		for _, key := range entry.Elements {
			if err = work(key[len(v.TablePrefix):]); err != nil {
				return errors.Trace(err)
			}
		}
		cursor = entry.Cursor
		if cursor == 0 {
			return nil
		}
	}
}

// Purge deletes all keys with the table prefix.
func (v *Valkey) Purge() error {
	ctx := context.Background()
	var cursor uint64
	for {
		entry, err := v.client.Do(ctx, v.client.B().Scan().Cursor(cursor).Match(string(v.TablePrefix)+"*").Count(100).Build()).AsScanEntry()
		if err != nil {
			return errors.Trace(err)
		}
		if len(entry.Elements) > 0 {
			if v.isCluster {
				for _, key := range entry.Elements {
					if err = v.client.Do(ctx, v.client.B().Del().Key(key).Build()).Error(); err != nil {
						return errors.Trace(err)
					}
				}
			} else {
				if err = v.client.Do(ctx, v.client.B().Del().Key(entry.Elements...).Build()).Error(); err != nil {
					return errors.Trace(err)
				}
			}
		}
		cursor = entry.Cursor
		if cursor == 0 {
			return nil
		}
	}
}

// Set stores values in Valkey.
func (v *Valkey) Set(ctx context.Context, values ...Value) error {
	if len(values) == 0 {
		return nil
	}
	for _, val := range values {
		if err := v.client.Do(ctx, v.client.B().Set().Key(v.Key(val.name)).Value(val.value).Build()).Error(); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Get returns a value from Valkey.
func (v *Valkey) Get(ctx context.Context, key string) *ReturnValue {
	result, err := v.client.Do(ctx, v.client.B().Get().Key(v.Key(key)).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return &ReturnValue{value: "", exists: false}
		}
		return &ReturnValue{err: err, exists: false}
	}
	return &ReturnValue{value: result, exists: true}
}

// Delete removes a key from Valkey.
func (v *Valkey) Delete(ctx context.Context, key string) error {
	return v.client.Do(ctx, v.client.B().Del().Key(v.Key(key)).Build()).Error()
}

// Push adds a message to a sorted set queue with timestamp as score.
func (v *Valkey) Push(ctx context.Context, name, message string) error {
	return v.client.Do(ctx, v.client.B().Zadd().Key(v.Key(name)).ScoreMember().ScoreMember(float64(time.Now().UnixNano()), message).Build()).Error()
}

// Pop removes and returns the message with the lowest score from the queue.
func (v *Valkey) Pop(ctx context.Context, name string) (string, error) {
	result, err := v.client.Do(ctx, v.client.B().Zpopmin().Key(v.Key(name)).Count(1).Build()).AsZScores()
	if err != nil {
		return "", errors.Trace(err)
	}
	if len(result) == 0 {
		return "", io.EOF
	}
	return result[0].Member, nil
}

// Remain returns the number of messages in the queue.
func (v *Valkey) Remain(ctx context.Context, name string) (int64, error) {
	return v.client.Do(ctx, v.client.B().Zcard().Key(v.Key(name)).Build()).AsInt64()
}

func (v *Valkey) documentKey(collection, subset, value string) string {
	return v.DocumentTable() + ":" + collection + ":" + subset + ":" + value
}

// AddScores adds score documents to Valkey using hash storage.
func (v *Valkey) AddScores(ctx context.Context, collection, subset string, documents []Score) error {
	for _, document := range documents {
		key := v.documentKey(collection, subset, document.Id)
		cmd := v.client.B().Hset().Key(key).FieldValue().
			FieldValue("collection", collection).
			FieldValue("subset", subset).
			FieldValue("id", document.Id).
			FieldValue("score", strconv.FormatFloat(document.Score, 'g', -1, 64)).
			FieldValue("is_hidden", formatBool(document.IsHidden)).
			FieldValue("categories", encodeCategories(document.Categories)).
			FieldValue("timestamp", strconv.FormatInt(document.Timestamp.UnixMicro(), 10)).
			Build()
		if err := v.client.Do(ctx, cmd).Error(); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func formatBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// SearchScores searches for score documents using FT.SEARCH.
func (v *Valkey) SearchScores(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Score, error) {
	var builder strings.Builder
	fmt.Fprintf(&builder, "@collection:{ %s } @is_hidden:[0 0]", escapeTag(collection))
	if subset != "" {
		fmt.Fprintf(&builder, " @subset:{ %s }", escapeTag(subset))
	}
	for _, q := range query {
		fmt.Fprintf(&builder, " @categories:{ %s }", escapeTag(encodeCategory(q)))
	}
	fetchLimit := 10000
	if end != -1 {
		fetchLimit = end
	}
	cmd := v.client.B().Arbitrary("FT.SEARCH").
		Keys(v.DocumentTable()).
		Args(builder.String(), "SORTBY", "score", "DESC", "LIMIT", "0", strconv.Itoa(fetchLimit)).
		Build()
	result, err := v.client.Do(ctx, cmd).ToArray()
	if err != nil {
		return nil, errors.Trace(err)
	}
	documents, err := parseFTSearchResult(result)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if begin > 0 || end != -1 {
		if begin >= len(documents) {
			return []Score{}, nil
		}
		endIdx := len(documents)
		if end != -1 && end < endIdx {
			endIdx = end
		}
		documents = documents[begin:endIdx]
	}
	return documents, nil
}

// UpdateScores updates score documents matching the query.
func (v *Valkey) UpdateScores(ctx context.Context, collections []string, subset *string, id string, patch ScorePatch) error {
	if len(collections) == 0 {
		return nil
	}
	if patch.Score == nil && patch.IsHidden == nil && patch.Categories == nil {
		return nil
	}
	var builder strings.Builder
	escapedCollections := make([]string, len(collections))
	for i, c := range collections {
		escapedCollections[i] = escapeTag(c)
	}
	fmt.Fprintf(&builder, "@collection:{ %s }", strings.Join(escapedCollections, " | "))
	fmt.Fprintf(&builder, " @id:{ %s }", escapeTag(id))
	if subset != nil {
		fmt.Fprintf(&builder, " @subset:{ %s }", escapeTag(*subset))
	}
	limit := v.maxSearchResults
	if limit <= 0 {
		limit = 10000
	}

	// Two-phase update: collect keys first, then mutate.
	keys := make([]string, 0)
	keySet := make(map[string]struct{})
	offset := 0
	for {
		cmd := v.client.B().Arbitrary("FT.SEARCH").
			Keys(v.DocumentTable()).
			Args(builder.String(), "SORTBY", "score", "DESC", "LIMIT", strconv.Itoa(offset), strconv.Itoa(limit)).
			Build()
		result, err := v.client.Do(ctx, cmd).ToArray()
		if err != nil {
			return errors.Trace(err)
		}
		if offset == 0 {
			total := parseFTSearchTotal(result)
			if total > limit {
				cmd = v.client.B().Arbitrary("FT.SEARCH").
					Keys(v.DocumentTable()).
					Args(builder.String(), "SORTBY", "score", "DESC", "LIMIT", "0", strconv.Itoa(total)).
					Build()
				result, err = v.client.Do(ctx, cmd).ToArray()
				if err != nil {
					return errors.Trace(err)
				}
			}
		}
		docKeys := parseFTSearchKeys(result)
		if len(docKeys) == 0 {
			break
		}
		newKeys := 0
		for _, k := range docKeys {
			if _, exists := keySet[k]; !exists {
				keySet[k] = struct{}{}
				keys = append(keys, k)
				newKeys++
			}
		}
		offset += len(docKeys)
		if len(docKeys) < limit || newKeys == 0 {
			break
		}
	}

	for _, key := range keys {
		cmd := v.client.B().Hset().Key(key).FieldValue()
		if patch.Score != nil {
			cmd = cmd.FieldValue("score", strconv.FormatFloat(*patch.Score, 'g', -1, 64))
		}
		if patch.IsHidden != nil {
			cmd = cmd.FieldValue("is_hidden", formatBool(*patch.IsHidden))
		}
		if patch.Categories != nil {
			cmd = cmd.FieldValue("categories", encodeCategories(patch.Categories))
		}
		if err := v.client.Do(ctx, cmd.Build()).Error(); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// DeleteScores deletes score documents matching the condition.
func (v *Valkey) DeleteScores(ctx context.Context, collections []string, condition ScoreCondition) error {
	if err := condition.Check(); err != nil {
		return errors.Trace(err)
	}
	var builder strings.Builder
	escapedCollections := make([]string, len(collections))
	for i, c := range collections {
		escapedCollections[i] = escapeTag(c)
	}
	fmt.Fprintf(&builder, "@collection:{ %s }", strings.Join(escapedCollections, " | "))
	if condition.Subset != nil {
		fmt.Fprintf(&builder, " @subset:{ %s }", escapeTag(*condition.Subset))
	}
	if condition.Id != nil {
		fmt.Fprintf(&builder, " @id:{ %s }", escapeTag(*condition.Id))
	}
	if condition.Before != nil {
		fmt.Fprintf(&builder, " @timestamp:[-inf (%d]", condition.Before.UnixMicro())
	}
	const maxDeleteIterations = 100
	for iteration := 0; iteration < maxDeleteIterations; iteration++ {
		cmd := v.client.B().Arbitrary("FT.SEARCH").
			Keys(v.DocumentTable()).
			Args(builder.String(), "SORTBY", "score", "DESC", "LIMIT", "0", "10000").
			Build()
		result, err := v.client.Do(ctx, cmd).ToArray()
		if err != nil {
			return errors.Trace(err)
		}
		docKeys := parseFTSearchKeys(result)
		total := parseFTSearchTotal(result)
		if len(docKeys) == 0 {
			break
		}
		if v.isCluster {
			for _, key := range docKeys {
				if err = v.client.Do(ctx, v.client.B().Del().Key(key).Build()).Error(); err != nil {
					return errors.Trace(err)
				}
			}
		} else {
			if err = v.client.Do(ctx, v.client.B().Del().Key(docKeys...).Build()).Error(); err != nil {
				return errors.Trace(err)
			}
		}
		if total == len(docKeys) {
			break
		}
	}
	return nil
}

// ScanScores iterates over all score documents.
func (v *Valkey) ScanScores(ctx context.Context, callback func(collection string, id string, subset string, timestamp time.Time) error) error {
	var cursor uint64
	for {
		entry, err := v.client.Do(ctx, v.client.B().Scan().Cursor(cursor).Match(v.DocumentTable()+":*").Count(100).Build()).AsScanEntry()
		if err != nil {
			return errors.Trace(err)
		}
		for _, key := range entry.Elements {
			row, err := v.client.Do(ctx, v.client.B().Hgetall().Key(key).Build()).AsStrMap()
			if err != nil {
				return errors.Trace(err)
			}
			usec, err := strconv.ParseInt(row["timestamp"], 10, 64)
			if err != nil {
				return errors.Trace(err)
			}
			if err = callback(row["collection"], row["id"], row["subset"], time.UnixMicro(usec).In(time.UTC)); err != nil {
				return errors.Trace(err)
			}
		}
		cursor = entry.Cursor
		if cursor == 0 {
			return nil
		}
	}
}

// AddTimeSeriesPoints adds time series points using sorted set + hash.
func (v *Valkey) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	grouped := groupTimeSeriesPoints(points)
	cmds := make(valkey.Commands, 0, len(grouped)*2)
	for name, sd := range grouped {
		indexKey := v.PointsTable() + ":ts_index:" + name
		dataKey := v.PointsTable() + ":ts_data:" + name
		// ZADD
		zaddCmd := v.client.B().Zadd().Key(indexKey).ScoreMember()
		for member, score := range sd.zaddMembers {
			zaddCmd = zaddCmd.ScoreMember(score, member)
		}
		cmds = append(cmds, zaddCmd.Build())
		// HSET
		hsetCmd := v.client.B().Hset().Key(dataKey).FieldValue()
		for field, value := range sd.hsetFields {
			hsetCmd = hsetCmd.FieldValue(field, value)
		}
		cmds = append(cmds, hsetCmd.Build())
	}
	for _, resp := range v.client.DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// timeSeriesGroup holds grouped ZADD members and HSET fields for a single time series.
type timeSeriesGroup struct {
	zaddMembers map[string]float64
	hsetFields  map[string]string
}

// groupTimeSeriesPoints groups time series points by name for batched writes.
func groupTimeSeriesPoints(points []TimeSeriesPoint) map[string]*timeSeriesGroup {
	grouped := make(map[string]*timeSeriesGroup)
	for _, point := range points {
		tsMs := point.Timestamp.UnixMilli()
		tsMsStr := strconv.FormatInt(tsMs, 10)
		sd, ok := grouped[point.Name]
		if !ok {
			sd = &timeSeriesGroup{
				zaddMembers: make(map[string]float64),
				hsetFields:  make(map[string]string),
			}
			grouped[point.Name] = sd
		}
		sd.zaddMembers[tsMsStr] = float64(tsMs)
		sd.hsetFields[tsMsStr] = strconv.FormatFloat(point.Value, 'g', -1, 64)
	}
	return grouped
}

// GetTimeSeriesPoints retrieves time series points with Go-side bucket aggregation.
func (v *Valkey) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time, duration time.Duration) ([]TimeSeriesPoint, error) {
	indexKey := v.PointsTable() + ":ts_index:" + name
	dataKey := v.PointsTable() + ":ts_data:" + name
	beginMs := begin.UnixMilli()
	endMs := end.UnixMilli()

	// Fetch all timestamps in range from sorted set.
	members, err := v.client.Do(ctx, v.client.B().Zrangebyscore().Key(indexKey).
		Min(strconv.FormatInt(beginMs, 10)).
		Max(strconv.FormatInt(endMs, 10)).
		Withscores().Build()).AsZScores()
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(members) == 0 {
		return nil, nil
	}

	// Fetch corresponding values from hash.
	fields := make([]string, len(members))
	for i, m := range members {
		fields[i] = m.Member
	}
	hmgetResults, err := v.client.Do(ctx, v.client.B().Hmget().Key(dataKey).Field(fields...).Build()).ToArray()
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Build timestamp-value pairs.
	type tsValue struct {
		timestampMs int64
		value       float64
	}
	tsValues := make([]tsValue, 0, len(members))
	for i, m := range members {
		valStr, err := hmgetResults[i].ToString()
		if err != nil {
			continue // nil result
		}
		tsMs, err := strconv.ParseInt(m.Member, 10, 64)
		if err != nil {
			log.Logger().Warn("failed to parse timestamp", zap.String("member", m.Member), zap.Error(err))
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			log.Logger().Warn("failed to parse value", zap.String("value", valStr), zap.Error(err))
			continue
		}
		tsValues = append(tsValues, tsValue{timestampMs: tsMs, value: val})
	}

	// Go-side bucket aggregation.
	durationMs := duration.Milliseconds()
	if durationMs <= 0 {
		durationMs = 1
	}
	type bucket struct {
		bucketMs  int64
		lastTsMs  int64
		lastValue float64
	}
	buckets := make(map[int64]*bucket)
	for _, tv := range tsValues {
		bk := (tv.timestampMs / durationMs) * durationMs
		existing, ok := buckets[bk]
		if !ok || tv.timestampMs > existing.lastTsMs {
			buckets[bk] = &bucket{bucketMs: bk, lastTsMs: tv.timestampMs, lastValue: tv.value}
		}
	}

	sortedBuckets := make([]*bucket, 0, len(buckets))
	for _, b := range buckets {
		sortedBuckets = append(sortedBuckets, b)
	}
	sort.Slice(sortedBuckets, func(i, j int) bool {
		return sortedBuckets[i].bucketMs < sortedBuckets[j].bucketMs
	})

	points := make([]TimeSeriesPoint, 0, len(sortedBuckets))
	for _, b := range sortedBuckets {
		points = append(points, TimeSeriesPoint{
			Name:      name,
			Timestamp: time.UnixMilli(b.bucketMs).UTC(),
			Value:     b.lastValue,
		})
	}
	return points, nil
}

// --- FT.SEARCH result parsing helpers ---

// parseFTSearchTotal extracts the total count from an FT.SEARCH result array.
// valkey-go returns: [total_int64, key1, [field1, val1, ...], key2, [field2, val2, ...], ...]
func parseFTSearchTotal(result []valkey.ValkeyMessage) int {
	if len(result) == 0 {
		return 0
	}
	total, err := result[0].AsInt64()
	if err != nil {
		return 0
	}
	return int(total)
}

// parseFTSearchKeys extracts document keys from an FT.SEARCH result array.
func parseFTSearchKeys(result []valkey.ValkeyMessage) []string {
	if len(result) < 2 {
		return nil
	}
	var keys []string
	for i := 1; i < len(result); i += 2 {
		key, err := result[i].ToString()
		if err == nil {
			keys = append(keys, key)
		}
	}
	return keys
}

// parseFTSearchResult parses an FT.SEARCH result into Score documents.
func parseFTSearchResult(result []valkey.ValkeyMessage) ([]Score, error) {
	if len(result) < 2 {
		return nil, nil
	}
	documents := make([]Score, 0)
	for i := 1; i < len(result); i += 2 {
		if i+1 >= len(result) {
			break
		}
		fields, err := parseFieldArray(result[i+1])
		if err != nil {
			continue
		}
		doc, err := scoreFromFieldMap(fields)
		if err != nil {
			return nil, err
		}
		documents = append(documents, doc)
	}
	// Sort by score descending to match the FT.SEARCH SORTBY score DESC.
	sort.Slice(documents, func(i, j int) bool {
		return documents[i].Score > documents[j].Score
	})
	return documents, nil
}

// parseFieldArray converts a ValkeyMessage field array [field1, val1, field2, val2, ...] into a map.
func parseFieldArray(msg valkey.ValkeyMessage) (map[string]string, error) {
	arr, err := msg.ToArray()
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for j := 0; j+1 < len(arr); j += 2 {
		key, err1 := arr[j].ToString()
		val, err2 := arr[j+1].ToString()
		if err1 == nil && err2 == nil {
			m[key] = val
		}
	}
	return m, nil
}

// scoreFromFieldMap converts a field map into a Score struct.
func scoreFromFieldMap(fields map[string]string) (Score, error) {
	var doc Score
	doc.Id = fields["id"]
	score, err := strconv.ParseFloat(fields["score"], 64)
	if err != nil {
		return doc, errors.Trace(err)
	}
	doc.Score = score
	isHidden, err := strconv.ParseInt(fields["is_hidden"], 10, 64)
	if err != nil {
		return doc, errors.Trace(err)
	}
	doc.IsHidden = isHidden != 0
	categories, err := decodeCategories(fields["categories"])
	if err != nil {
		return doc, errors.Trace(err)
	}
	doc.Categories = categories
	timestamp, err := strconv.ParseInt(fields["timestamp"], 10, 64)
	if err != nil {
		return doc, errors.Trace(err)
	}
	doc.Timestamp = time.UnixMicro(timestamp).In(time.UTC)
	return doc, nil
}

// valkeyTagEscaper escapes all TAG special characters for Valkey Search queries.
var valkeyTagEscaper = strings.NewReplacer(
	`\`, `\\`,
	`{`, `\{`,
	`}`, `\}`,
	`|`, `\|`,
	`*`, `\*`,
	`(`, `\(`,
	`)`, `\)`,
	`~`, `\~`,
	`@`, `\@`,
	`"`, `\"`,
	`'`, `\'`,
	`-`, `\-`,
	`:`, `\:`,
	`.`, `\.`,
	`/`, `\/`,
	`+`, `\+`,
)

// escapeTag escapes a value for use in Valkey Search TAG queries.
func escapeTag(s string) string {
	return valkeyTagEscaper.Replace(s)
}
