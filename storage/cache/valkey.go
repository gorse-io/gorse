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
	glide "github.com/valkey-io/valkey-glide/go/v2"
	glideconfig "github.com/valkey-io/valkey-glide/go/v2/config"
	"github.com/valkey-io/valkey-glide/go/v2/models"
	glideoptions "github.com/valkey-io/valkey-glide/go/v2/options"
	"github.com/valkey-io/valkey-glide/go/v2/pipeline"
	"go.uber.org/zap"
)

func init() {
	Register([]string{storage.ValkeyPrefix, storage.ValkeysPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		addr, password, db, useTLS, err := parseValkeyURL(path)
		if err != nil {
			return nil, errors.Trace(err)
		}
		cfg := glideconfig.NewClientConfiguration().
			WithAddress(&addr)
		if password != "" {
			cfg.WithCredentials(glideconfig.NewServerCredentialsWithDefaultUsername(password))
		}
		if db > 0 {
			cfg.WithDatabaseId(db)
		}
		if useTLS {
			cfg.WithUseTLS(true)
		}
		client, err := glide.NewClient(cfg)
		if err != nil {
			return nil, errors.Trace(err)
		}
		database := &Valkey{
			standaloneClient: client,
			isCluster:        false,
			maxSearchResults: storage.NewOptions(opts...).MaxSearchResults,
		}
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		return database, nil
	})
	Register([]string{storage.ValkeyClusterPrefix, storage.ValkeysClusterPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		addresses, password, useTLS, err := parseValkeyClusterURL(path)
		if err != nil {
			return nil, errors.Trace(err)
		}
		cfg := glideconfig.NewClusterClientConfiguration()
		for i := range addresses {
			cfg.WithAddress(&addresses[i])
		}
		if password != "" {
			cfg.WithCredentials(glideconfig.NewServerCredentialsWithDefaultUsername(password))
		}
		if useTLS {
			cfg.WithUseTLS(true)
		}
		client, err := glide.NewClusterClient(cfg)
		if err != nil {
			return nil, errors.Trace(err)
		}
		database := &Valkey{
			clusterClient:    client,
			isCluster:        true,
			maxSearchResults: storage.NewOptions(opts...).MaxSearchResults,
		}
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		return database, nil
	})
}

// parseValkeyURL parses a valkey:// or valkeys:// URL into connection parameters.
func parseValkeyURL(rawURL string) (addr glideconfig.NodeAddress, password string, db int, useTLS bool, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return addr, "", 0, false, errors.Trace(err)
	}
	host := parsed.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := 6379
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil {
			return addr, "", 0, false, errors.Trace(err)
		}
	}
	addr = glideconfig.NodeAddress{Host: host, Port: port}
	if parsed.User != nil {
		password, _ = parsed.User.Password()
	}
	if parsed.Path != "" && parsed.Path != "/" {
		dbStr := strings.TrimPrefix(parsed.Path, "/")
		if dbStr != "" {
			db, err = strconv.Atoi(dbStr)
			if err != nil {
				return addr, "", 0, false, errors.Errorf("invalid database number: %s", dbStr)
			}
		}
	}
	useTLS = parsed.Scheme == "valkeys"
	return addr, password, db, useTLS, nil
}

// parseValkeyClusterURL parses a valkey+cluster:// or valkeys+cluster:// URL.
func parseValkeyClusterURL(rawURL string) (addresses []glideconfig.NodeAddress, password string, useTLS bool, err error) {
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
		return nil, "", false, errors.Trace(err)
	}
	host := parsed.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := 6379
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil {
			return nil, "", false, errors.Trace(err)
		}
	}
	addresses = append(addresses, glideconfig.NodeAddress{Host: host, Port: port})
	if parsed.User != nil {
		password, _ = parsed.User.Password()
	}
	// Parse additional addresses from query params (addr=host:port).
	for _, addrStr := range parsed.Query()["addr"] {
		parts := strings.SplitN(addrStr, ":", 2)
		h := parts[0]
		p := 6379
		if len(parts) == 2 {
			p, err = strconv.Atoi(parts[1])
			if err != nil {
				return nil, "", false, errors.Errorf("invalid port in addr param: %s", addrStr)
			}
		}
		addresses = append(addresses, glideconfig.NodeAddress{Host: h, Port: p})
	}
	return addresses, password, useTLS, nil
}

// Valkey cache storage using valkey-glide client.
type Valkey struct {
	storage.TablePrefix
	standaloneClient *glide.Client
	clusterClient    *glide.ClusterClient
	isCluster        bool
	maxSearchResults int
}

// Close the valkey connection.
func (v *Valkey) Close() error {
	if v.isCluster {
		v.clusterClient.Close()
	} else {
		v.standaloneClient.Close()
	}
	return nil
}

// Ping the valkey server.
func (v *Valkey) Ping() error {
	ctx := context.Background()
	if v.isCluster {
		_, err := v.clusterClient.Ping(ctx)
		return err
	}
	_, err := v.standaloneClient.Ping(ctx)
	return err
}

// Init creates the valkey-search index for document storage.
func (v *Valkey) Init() error {
	ctx := context.Background()
	// List existing indices via FT._LIST.
	result, err := v.customCommand(ctx, []string{"FT._LIST"})
	if err != nil {
		return errors.Trace(err)
	}
	indices := parseStringSlice(result)
	if lo.Contains(indices, v.DocumentTable()) {
		return nil
	}
	// Create the index.
	_, err = v.customCommand(ctx, []string{
		"FT.CREATE", v.DocumentTable(),
		"ON", "HASH",
		"PREFIX", "1", v.DocumentTable() + ":",
		"SCHEMA",
		"collection", "TAG",
		"subset", "TAG",
		"id", "TAG",
		"score", "NUMERIC",
		"is_hidden", "NUMERIC",
		"categories", "TAG", "SEPARATOR", ";",
		"timestamp", "NUMERIC",
	})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// customCommand executes a custom command, handling both standalone and cluster modes.
func (v *Valkey) customCommand(ctx context.Context, args []string) (any, error) {
	if v.isCluster {
		result, err := v.clusterClient.CustomCommand(ctx, args)
		if err != nil {
			return nil, err
		}
		return result.SingleValue(), nil
	}
	return v.standaloneClient.CustomCommand(ctx, args)
}

// Scan iterates over all keys with the table prefix.
func (v *Valkey) Scan(work func(string) error) error {
	ctx := context.Background()
	if v.isCluster {
		cursor := models.NewClusterScanCursor()
		scanOpts := glideoptions.NewClusterScanOptions().SetMatch(string(v.TablePrefix) + "*")
		for !cursor.IsFinished() {
			result, err := v.clusterClient.ScanWithOptions(ctx, cursor, *scanOpts)
			if err != nil {
				return errors.Trace(err)
			}
			for _, key := range result.Keys {
				if err = work(key[len(v.TablePrefix):]); err != nil {
					return errors.Trace(err)
				}
			}
			cursor = result.Cursor
		}
		return nil
	}
	cursor := models.NewCursor()
	scanOpts := glideoptions.NewScanOptions().SetMatch(string(v.TablePrefix) + "*")
	for {
		result, err := v.standaloneClient.ScanWithOptions(ctx, cursor, *scanOpts)
		if err != nil {
			return errors.Trace(err)
		}
		for _, key := range result.Data {
			if err = work(key[len(v.TablePrefix):]); err != nil {
				return errors.Trace(err)
			}
		}
		if result.Cursor.IsFinished() {
			return nil
		}
		cursor = result.Cursor
	}
}

// Purge deletes all keys with the table prefix.
func (v *Valkey) Purge() error {
	ctx := context.Background()
	if v.isCluster {
		cursor := models.NewClusterScanCursor()
		scanOpts := glideoptions.NewClusterScanOptions().SetMatch(string(v.TablePrefix) + "*")
		for !cursor.IsFinished() {
			result, err := v.clusterClient.ScanWithOptions(ctx, cursor, *scanOpts)
			if err != nil {
				return errors.Trace(err)
			}
			if len(result.Keys) > 0 {
				if _, err = v.clusterClient.Del(ctx, result.Keys); err != nil {
					return errors.Trace(err)
				}
			}
			cursor = result.Cursor
		}
		return nil
	}
	cursor := models.NewCursor()
	scanOpts := glideoptions.NewScanOptions().SetMatch(string(v.TablePrefix) + "*")
	for {
		result, err := v.standaloneClient.ScanWithOptions(ctx, cursor, *scanOpts)
		if err != nil {
			return errors.Trace(err)
		}
		if len(result.Data) > 0 {
			if _, err = v.standaloneClient.Del(ctx, result.Data); err != nil {
				return errors.Trace(err)
			}
		}
		if result.Cursor.IsFinished() {
			return nil
		}
		cursor = result.Cursor
	}
}

// Set stores values in Valkey.
func (v *Valkey) Set(ctx context.Context, values ...Value) error {
	for _, val := range values {
		var err error
		if v.isCluster {
			_, err = v.clusterClient.Set(ctx, v.Key(val.name), val.value)
		} else {
			_, err = v.standaloneClient.Set(ctx, v.Key(val.name), val.value)
		}
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Get returns a value from Valkey.
func (v *Valkey) Get(ctx context.Context, key string) *ReturnValue {
	var result models.Result[string]
	var err error
	if v.isCluster {
		result, err = v.clusterClient.Get(ctx, v.Key(key))
	} else {
		result, err = v.standaloneClient.Get(ctx, v.Key(key))
	}
	if err != nil {
		return &ReturnValue{err: err, exists: false}
	}
	if result.IsNil() {
		return &ReturnValue{value: "", exists: false}
	}
	return &ReturnValue{value: result.Value(), exists: true}
}

// Delete removes a key from Valkey.
func (v *Valkey) Delete(ctx context.Context, key string) error {
	if v.isCluster {
		_, err := v.clusterClient.Del(ctx, []string{v.Key(key)})
		return err
	}
	_, err := v.standaloneClient.Del(ctx, []string{v.Key(key)})
	return err
}

// Push adds a message to a sorted set queue with timestamp as score.
func (v *Valkey) Push(ctx context.Context, name, message string) error {
	members := map[string]float64{message: float64(time.Now().UnixNano())}
	if v.isCluster {
		_, err := v.clusterClient.ZAdd(ctx, v.Key(name), members)
		return err
	}
	_, err := v.standaloneClient.ZAdd(ctx, v.Key(name), members)
	return err
}

// Pop removes and returns the message with the lowest score from the queue.
func (v *Valkey) Pop(ctx context.Context, name string) (string, error) {
	var result map[string]float64
	var err error
	if v.isCluster {
		result, err = v.clusterClient.ZPopMin(ctx, v.Key(name))
	} else {
		result, err = v.standaloneClient.ZPopMin(ctx, v.Key(name))
	}
	if err != nil {
		return "", errors.Trace(err)
	}
	if len(result) == 0 {
		return "", io.EOF
	}
	for member := range result {
		return member, nil
	}
	return "", io.EOF
}

// Remain returns the number of messages in the queue.
func (v *Valkey) Remain(ctx context.Context, name string) (int64, error) {
	if v.isCluster {
		return v.clusterClient.ZCard(ctx, v.Key(name))
	}
	return v.standaloneClient.ZCard(ctx, v.Key(name))
}

func (v *Valkey) documentKey(collection, subset, value string) string {
	return v.DocumentTable() + ":" + collection + ":" + subset + ":" + value
}

// AddScores adds score documents to Valkey using hash storage.
func (v *Valkey) AddScores(ctx context.Context, collection, subset string, documents []Score) error {
	if v.isCluster {
		for _, document := range documents {
			if _, err := v.clusterClient.HSet(ctx, v.documentKey(collection, subset, document.Id), map[string]string{
				"collection": collection,
				"subset":     subset,
				"id":         document.Id,
				"score":      strconv.FormatFloat(document.Score, 'g', -1, 64),
				"is_hidden":  formatBool(document.IsHidden),
				"categories": encodeCategories(document.Categories),
				"timestamp":  strconv.FormatInt(document.Timestamp.UnixMicro(), 10),
			}); err != nil {
				return errors.Trace(err)
			}
		}
		return nil
	}
	batch := pipeline.NewStandaloneBatch(false)
	for _, document := range documents {
		batch.HSet(v.documentKey(collection, subset, document.Id), map[string]string{
			"collection": collection,
			"subset":     subset,
			"id":         document.Id,
			"score":      strconv.FormatFloat(document.Score, 'g', -1, 64),
			"is_hidden":  formatBool(document.IsHidden),
			"categories": encodeCategories(document.Categories),
			"timestamp":  strconv.FormatInt(document.Timestamp.UnixMicro(), 10),
		})
	}
	_, err := v.standaloneClient.Exec(ctx, *batch, true)
	return errors.Trace(err)
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
	fmt.Fprintf(&builder, "@collection:{ %s } @is_hidden:[0 0]", escape(collection))
	if subset != "" {
		fmt.Fprintf(&builder, " @subset:{ %s }", escape(subset))
	}
	for _, q := range query {
		fmt.Fprintf(&builder, " @categories:{ %s }", escape(encodeCategory(q)))
	}
	// Fetch enough results to cover the requested range.
	// We request from offset 0 because the Glide map format loses ordering,
	// and we sort + slice in Go.
	fetchLimit := 10000
	if end != -1 {
		fetchLimit = end
	}
	args := []string{
		"FT.SEARCH", v.DocumentTable(), builder.String(),
		"SORTBY", "score", "DESC",
		"LIMIT", "0", strconv.Itoa(fetchLimit),
	}
	result, err := v.customCommand(ctx, args)
	if err != nil {
		return nil, errors.Trace(err)
	}
	documents, err := parseFTSearchResult(result)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Apply begin/end slicing since Glide map format loses server-side ordering.
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
	fmt.Fprintf(&builder, "@collection:{ %s }", escape(strings.Join(collections, " | ")))
	fmt.Fprintf(&builder, " @id:{ %s }", escape(id))
	if subset != nil {
		fmt.Fprintf(&builder, " @subset:{ %s }", escape(*subset))
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
		args := []string{
			"FT.SEARCH", v.DocumentTable(), builder.String(),
			"SORTBY", "score", "DESC",
			"LIMIT", strconv.Itoa(offset), strconv.Itoa(limit),
		}
		result, err := v.customCommand(ctx, args)
		if err != nil {
			return errors.Trace(err)
		}
		// On the first page, check the total and fetch all at once if needed.
		// This avoids pagination issues with tied scores where the server
		// may return duplicates across pages.
		if offset == 0 {
			total := parseFTSearchTotal(result)
			if total > limit {
				// Re-fetch with the full count to avoid pagination drift.
				args[len(args)-1] = strconv.Itoa(total)
				result, err = v.customCommand(ctx, args)
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

	values := make(map[string]string)
	if patch.Score != nil {
		values["score"] = strconv.FormatFloat(*patch.Score, 'g', -1, 64)
	}
	if patch.IsHidden != nil {
		values["is_hidden"] = formatBool(*patch.IsHidden)
	}
	if patch.Categories != nil {
		values["categories"] = encodeCategories(patch.Categories)
	}
	for _, key := range keys {
		if v.isCluster {
			exists, err := v.clusterClient.Exists(ctx, []string{key})
			if err != nil {
				return errors.Trace(err)
			}
			if exists == 0 {
				continue
			}
			if _, err = v.clusterClient.HSet(ctx, key, values); err != nil {
				return errors.Trace(err)
			}
		} else {
			// Use Watch for optimistic locking on standalone.
			if _, err := v.standaloneClient.Watch(ctx, []string{key}); err != nil {
				return errors.Trace(err)
			}
			exists, err := v.standaloneClient.Exists(ctx, []string{key})
			if err != nil {
				v.standaloneClient.Unwatch(ctx) //nolint:errcheck
				return errors.Trace(err)
			}
			if exists == 0 {
				v.standaloneClient.Unwatch(ctx) //nolint:errcheck
				continue
			}
			if _, err = v.standaloneClient.HSet(ctx, key, values); err != nil {
				v.standaloneClient.Unwatch(ctx) //nolint:errcheck
				return errors.Trace(err)
			}
			v.standaloneClient.Unwatch(ctx) //nolint:errcheck
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
	fmt.Fprintf(&builder, "@collection:{ %s }", escape(strings.Join(collections, " | ")))
	if condition.Subset != nil {
		fmt.Fprintf(&builder, " @subset:{ %s }", escape(*condition.Subset))
	}
	if condition.Id != nil {
		fmt.Fprintf(&builder, " @id:{ %s }", escape(*condition.Id))
	}
	if condition.Before != nil {
		fmt.Fprintf(&builder, " @timestamp:[-inf (%d]", condition.Before.UnixMicro())
	}
	for {
		args := []string{
			"FT.SEARCH", v.DocumentTable(), builder.String(),
			"SORTBY", "score", "DESC",
			"LIMIT", "0", "10000",
		}
		result, err := v.customCommand(ctx, args)
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
				if _, err = v.clusterClient.Del(ctx, []string{key}); err != nil {
					return errors.Trace(err)
				}
			}
		} else {
			batch := pipeline.NewStandaloneBatch(false)
			for _, key := range docKeys {
				batch.Del([]string{key})
			}
			if _, err = v.standaloneClient.Exec(ctx, *batch, true); err != nil {
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
	if v.isCluster {
		cursor := models.NewClusterScanCursor()
		scanOpts := glideoptions.NewClusterScanOptions().SetMatch(v.DocumentTable() + "*")
		for !cursor.IsFinished() {
			result, err := v.clusterClient.ScanWithOptions(ctx, cursor, *scanOpts)
			if err != nil {
				return errors.Trace(err)
			}
			for _, key := range result.Keys {
				row, err := v.clusterClient.HGetAll(ctx, key)
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
			cursor = result.Cursor
		}
		return nil
	}
	cursor := models.NewCursor()
	scanOpts := glideoptions.NewScanOptions().SetMatch(v.DocumentTable() + "*")
	for {
		result, err := v.standaloneClient.ScanWithOptions(ctx, cursor, *scanOpts)
		if err != nil {
			return errors.Trace(err)
		}
		for _, key := range result.Data {
			row, err := v.standaloneClient.HGetAll(ctx, key)
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
		if result.Cursor.IsFinished() {
			return nil
		}
		cursor = result.Cursor
	}
}

// AddTimeSeriesPoints adds time series points using sorted set + hash (Approach A).
// Sorted set key: {prefix}ts_index:{name} — score = timestamp_ms, member = timestamp_ms as string
// Hash key: {prefix}ts_data:{name} — field = timestamp_ms as string, value = float64 as string
func (v *Valkey) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	if v.isCluster {
		for _, point := range points {
			tsMs := point.Timestamp.UnixMilli()
			tsMsStr := strconv.FormatInt(tsMs, 10)
			indexKey := v.PointsTable() + ":ts_index:" + point.Name
			dataKey := v.PointsTable() + ":ts_data:" + point.Name
			if _, err := v.clusterClient.ZAdd(ctx, indexKey, map[string]float64{tsMsStr: float64(tsMs)}); err != nil {
				return errors.Trace(err)
			}
			if _, err := v.clusterClient.HSet(ctx, dataKey, map[string]string{
				tsMsStr: strconv.FormatFloat(point.Value, 'g', -1, 64),
			}); err != nil {
				return errors.Trace(err)
			}
		}
		return nil
	}
	batch := pipeline.NewStandaloneBatch(false)
	for _, point := range points {
		tsMs := point.Timestamp.UnixMilli()
		tsMsStr := strconv.FormatInt(tsMs, 10)
		indexKey := v.PointsTable() + ":ts_index:" + point.Name
		dataKey := v.PointsTable() + ":ts_data:" + point.Name
		batch.ZAdd(indexKey, map[string]float64{tsMsStr: float64(tsMs)})
		batch.HSet(dataKey, map[string]string{
			tsMsStr: strconv.FormatFloat(point.Value, 'g', -1, 64),
		})
	}
	_, err := v.standaloneClient.Exec(ctx, *batch, true)
	return errors.Trace(err)
}

// GetTimeSeriesPoints retrieves time series points with Go-side bucket aggregation.
func (v *Valkey) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time, duration time.Duration) ([]TimeSeriesPoint, error) {
	indexKey := v.PointsTable() + ":ts_index:" + name
	dataKey := v.PointsTable() + ":ts_data:" + name
	beginMs := begin.UnixMilli()
	endMs := end.UnixMilli()

	// Fetch all timestamps in range from sorted set using ZRANGEBYSCORE via ZRangeWithScores.
	rangeQuery := glideoptions.NewRangeByScoreQuery(
		glideoptions.NewInclusiveScoreBoundary(float64(beginMs)),
		glideoptions.NewInclusiveScoreBoundary(float64(endMs)),
	)
	var members []models.MemberAndScore
	var err error
	if v.isCluster {
		members, err = v.clusterClient.ZRangeWithScores(ctx, indexKey, rangeQuery)
	} else {
		members, err = v.standaloneClient.ZRangeWithScores(ctx, indexKey, rangeQuery)
	}
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
	var hmgetResults []models.Result[string]
	if v.isCluster {
		hmgetResults, err = v.clusterClient.HMGet(ctx, dataKey, fields)
	} else {
		hmgetResults, err = v.standaloneClient.HMGet(ctx, dataKey, fields)
	}
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
		if hmgetResults[i].IsNil() {
			continue
		}
		tsMs, err := strconv.ParseInt(m.Member, 10, 64)
		if err != nil {
			log.Logger().Warn("failed to parse timestamp", zap.String("member", m.Member), zap.Error(err))
			continue
		}
		val, err := strconv.ParseFloat(hmgetResults[i].Value(), 64)
		if err != nil {
			log.Logger().Warn("failed to parse value", zap.String("value", hmgetResults[i].Value()), zap.Error(err))
			continue
		}
		tsValues = append(tsValues, tsValue{timestampMs: tsMs, value: val})
	}

	// Go-side bucket aggregation: group by floor(timestamp_ms / duration_ms) * duration_ms,
	// take the last value per bucket (highest timestamp).
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

	// Sort buckets by timestamp ascending.
	sortedBuckets := make([]*bucket, 0, len(buckets))
	for _, b := range buckets {
		sortedBuckets = append(sortedBuckets, b)
	}
	sort.Slice(sortedBuckets, func(i, j int) bool {
		return sortedBuckets[i].bucketMs < sortedBuckets[j].bucketMs
	})

	// Convert to TimeSeriesPoint.
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

// parseStringSlice converts a CustomCommand result to a string slice.
func parseStringSlice(result any) []string {
	if result == nil {
		return nil
	}
	switch v := result.(type) {
	case []any:
		strs := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				strs = append(strs, s)
			}
		}
		return strs
	default:
		return nil
	}
}

// parseFTSearchTotal extracts the total count from an FT.SEARCH result.
// Valkey Glide returns: [int64_total, map[string]interface{}{docKey: map[string]interface{}{fields...}, ...}]
func parseFTSearchTotal(result any) int {
	if result == nil {
		return 0
	}
	arr, ok := result.([]any)
	if !ok || len(arr) == 0 {
		return 0
	}
	switch v := arr[0].(type) {
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// parseFTSearchKeys extracts document keys from an FT.SEARCH result.
// Valkey Glide returns: [int64_total, map[string]interface{}{docKey: fieldsMap, ...}]
func parseFTSearchKeys(result any) []string {
	if result == nil {
		return nil
	}
	arr, ok := result.([]any)
	if !ok || len(arr) < 2 {
		return nil
	}
	// Glide format: arr[1] is a map[string]interface{} where keys are doc keys.
	if docMap, ok := arr[1].(map[string]any); ok {
		keys := make([]string, 0, len(docMap))
		for key := range docMap {
			keys = append(keys, key)
		}
		return keys
	}
	// Fallback: flat array format [total, key1, [fields...], key2, [fields...], ...]
	var keys []string
	for i := 1; i < len(arr); i += 2 {
		if key, ok := arr[i].(string); ok {
			keys = append(keys, key)
		}
	}
	return keys
}

// parseFTSearchResult parses an FT.SEARCH result into Score documents.
// Valkey Glide returns: [int64_total, map[string]interface{}{docKey: map[string]interface{}{fields...}, ...}]
func parseFTSearchResult(result any) ([]Score, error) {
	if result == nil {
		return nil, nil
	}
	arr, ok := result.([]any)
	if !ok || len(arr) < 2 {
		return nil, nil
	}

	// Glide format: arr[1] is a map[string]interface{} where keys are doc keys
	// and values are maps of field name → field value.
	if docMap, ok := arr[1].(map[string]any); ok {
		return parseFTSearchResultFromMap(docMap)
	}

	// Fallback: flat array format [total, key1, [fields...], key2, [fields...], ...]
	documents := make([]Score, 0)
	for i := 1; i < len(arr); i += 2 {
		if i+1 >= len(arr) {
			break
		}
		fields := parseFieldMap(arr[i+1])
		if fields == nil {
			continue
		}
		doc, err := scoreFromFieldMap(fields)
		if err != nil {
			return nil, err
		}
		documents = append(documents, doc)
	}
	return documents, nil
}

// parseFTSearchResultFromMap parses the Glide map format into Score documents.
// The map has doc keys as keys and field maps as values.
// We need to sort by score DESC to match the SORTBY in the query.
func parseFTSearchResultFromMap(docMap map[string]any) ([]Score, error) {
	documents := make([]Score, 0, len(docMap))
	for _, docValue := range docMap {
		fields := parseFieldMap(docValue)
		if fields == nil {
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

// parseFieldMap converts a field array from FT.SEARCH into a map.
// The field array is [field1, value1, field2, value2, ...] or a map.
func parseFieldMap(v any) map[string]string {
	if v == nil {
		return nil
	}
	switch fields := v.(type) {
	case []any:
		m := make(map[string]string)
		for j := 0; j+1 < len(fields); j += 2 {
			key, ok1 := fields[j].(string)
			val, ok2 := fields[j+1].(string)
			if ok1 && ok2 {
				m[key] = val
			}
		}
		return m
	case map[string]any:
		m := make(map[string]string)
		for k, val := range fields {
			m[k] = fmt.Sprint(val)
		}
		return m
	case map[string]string:
		return fields
	default:
		return nil
	}
}
