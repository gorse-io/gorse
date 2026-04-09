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

package data

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
)


func init() {
	Register([]string{storage.ElasticPrefix, storage.ElasticsPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		// Parse URL and convert to Elasticsearch config
		esURL := path
		if strings.HasPrefix(path, storage.ElasticPrefix) {
			esURL = "http://" + strings.TrimPrefix(path, storage.ElasticPrefix)
		} else if strings.HasPrefix(path, storage.ElasticsPrefix) {
			esURL = "https://" + strings.TrimPrefix(path, storage.ElasticsPrefix)
		}

		parsedURL, err := url.Parse(esURL)
		if err != nil {
			return nil, errors.Trace(err)
		}

		// Extract database name from path
		dbName := strings.TrimPrefix(parsedURL.Path, "/")
		if dbName == "" {
			dbName = "gorse"
		}

		// Build Elasticsearch config
		cfg := elasticsearch.Config{
			Addresses: []string{fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)},
		}

		// Extract username and password if present
		if parsedURL.User != nil {
			cfg.Username = parsedURL.User.Username()
			password, hasPassword := parsedURL.User.Password()
			if hasPassword {
				cfg.Password = password
			}
		}

		// Create Elasticsearch client
		client, err := elasticsearch.NewClient(cfg)
		if err != nil {
			return nil, errors.Trace(err)
		}

		database := &Elasticsearch{
			TablePrefix: storage.TablePrefix(tablePrefix),
			client:      client,
			dbName:      dbName,
		}
		return database, nil
	})
}

// Elasticsearch is the data storage based on Elasticsearch.
type Elasticsearch struct {
	storage.TablePrefix
	client *elasticsearch.Client
	dbName string
}

// Optimize is used by ClickHouse only.
func (db *Elasticsearch) Optimize() error {
	return nil
}

// Init indices in Elasticsearch.
func (db *Elasticsearch) Init() error {
	ctx := context.Background()

	// Create indices with mappings
	indices := []struct {
		name    string
		mapping string
	}{
		{
			name: db.UsersTable(),
			mapping: `{
				"mappings": {
					"properties": {
						"user_id": {"type": "keyword"},
						"labels": {"type": "object", "enabled": true},
						"comment": {"type": "text"}
					}
				}
			}`,
		},
		{
			name: db.ItemsTable(),
			mapping: `{
				"mappings": {
					"properties": {
						"item_id": {"type": "keyword"},
						"is_hidden": {"type": "boolean"},
						"categories": {"type": "keyword"},
						"timestamp": {"type": "date"},
						"labels": {"type": "object", "enabled": true},
						"comment": {"type": "text"}
					}
				}
			}`,
		},
		{
			name: db.FeedbackTable(),
			mapping: `{
				"mappings": {
					"properties": {
						"feedback_type": {"type": "keyword"},
						"user_id": {"type": "keyword"},
						"item_id": {"type": "keyword"},
						"value": {"type": "float"},
						"timestamp": {"type": "date"},
						"updated": {"type": "date"},
						"comment": {"type": "text"}
					}
				}
			}`,
		},
	}

	for _, index := range indices {
		// Check if index exists
		req := esapi.IndicesExistsRequest{
			Index: []string{index.name},
		}
		res, err := req.Do(ctx, db.client)
		if err != nil {
			return errors.Trace(err)
		}
		res.Body.Close()

		if res.StatusCode == http.StatusNotFound {
			// Create index with mapping
			createReq := esapi.IndicesCreateRequest{
				Index: index.name,
				Body:  bytes.NewReader([]byte(index.mapping)),
			}
			createRes, err := createReq.Do(ctx, db.client)
			if err != nil {
				return errors.Trace(err)
			}
			createRes.Body.Close()
			if createRes.IsError() {
				return errors.Errorf("failed to create index %s: %s", index.name, createRes.String())
			}
		}
	}
	return nil
}

func (db *Elasticsearch) Ping() error {
	ctx := context.Background()
	req := esapi.InfoRequest{}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return errors.Trace(err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return errors.Errorf("failed to ping Elasticsearch: %s", res.String())
	}
	return nil
}

// Close connection to Elasticsearch.
func (db *Elasticsearch) Close() error {
	// Elasticsearch client doesn't have a Close method
	return nil
}

func (db *Elasticsearch) Purge() error {
	ctx := context.Background()
	indices := []string{db.UsersTable(), db.ItemsTable(), db.FeedbackTable()}
	for _, index := range indices {
		req := esapi.DeleteByQueryRequest{
			Index: []string{index},
			Body:  bytes.NewReader([]byte(`{"query": {"match_all": {}}}`)),
		}
		res, err := req.Do(ctx, db.client)
		if err != nil {
			return errors.Trace(err)
		}
		res.Body.Close()
		if res.IsError() {
			return errors.Errorf("failed to purge index %s: %s", index, res.String())
		}
	}
	return nil
}

// BatchInsertItems insert items into Elasticsearch.
func (db *Elasticsearch) BatchInsertItems(ctx context.Context, items []Item) error {
	if len(items) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, item := range items {
		// Create bulk action
		action := fmt.Sprintf(`{"index": {"_index": "%s", "_id": "%s"}}`, db.ItemsTable(), item.ItemId)
		buf.WriteString(action + "\n")

		// Create document body
		doc, err := json.Marshal(item)
		if err != nil {
			return errors.Trace(err)
		}
		buf.Write(doc)
		buf.WriteString("\n")
	}

	req := esapi.BulkRequest{
		Body: bytes.NewReader(buf.Bytes()),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return errors.Trace(err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return errors.Errorf("failed to bulk insert items: %s", res.String())
	}
	return nil
}

func (db *Elasticsearch) BatchGetItems(ctx context.Context, itemIds []string, opts GetOptions) ([]Item, error) {
	if len(itemIds) == 0 {
		return nil, nil
	}

	// Build query
	query := map[string]any{
		"query": map[string]any{
			"terms": map[string]any{
				"item_id": itemIds,
			},
		},
	}

	// Add filters
	if len(opts.Categories) > 0 {
		query["query"] = map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					{"terms": map[string]any{"item_id": itemIds}},
					{"terms": map[string]any{"categories": opts.Categories}},
				},
			},
		}
	}
	if opts.SkipHidden {
		if boolQuery, ok := query["query"].(map[string]any); ok {
			if m, ok := boolQuery["bool"].(map[string]any); ok {
				m["must"] = append(m["must"].([]map[string]any), map[string]any{
					"term": map[string]any{"is_hidden": false},
				})
			}
		} else {
			query["query"] = map[string]any{
				"bool": map[string]any{
					"must": []map[string]any{
						{"terms": map[string]any{"item_id": itemIds}},
						{"term": map[string]any{"is_hidden": false}},
					},
				},
			}
		}
	}
	if opts.After != nil {
		if boolQuery, ok := query["query"].(map[string]any); ok {
			if m, ok := boolQuery["bool"].(map[string]any); ok {
				m["must"] = append(m["must"].([]map[string]any), map[string]any{
					"range": map[string]any{"timestamp": map[string]any{"gt": *opts.After}},
				})
			}
		} else {
			query["query"] = map[string]any{
				"bool": map[string]any{
					"must": []map[string]any{
						{"terms": map[string]any{"item_id": itemIds}},
						{"range": map[string]any{"timestamp": map[string]any{"gt": *opts.After}}},
					},
				},
			}
		}
	}

	// Add projection for ReturnId
	if opts.ReturnId {
		query["_source"] = []string{"item_id"}
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, errors.Trace(err)
	}

	req := esapi.SearchRequest{
		Index: []string{db.ItemsTable()},
		Body:  bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, errors.Trace(err)
	}

	items := make([]Item, 0)
	if hits, ok := result["hits"].(map[string]any); ok {
		if hitList, ok := hits["hits"].([]any); ok {
			for _, hit := range hitList {
				if hitMap, ok := hit.(map[string]any); ok {
					if source, ok := hitMap["_source"].(map[string]any); ok {
						item := db.parseItem(source)
						items = append(items, item)
					}
				}
			}
		}
	}
	return items, nil
}

func (db *Elasticsearch) parseItem(source map[string]any) Item {
	item := Item{}
	if v, ok := source["item_id"].(string); ok {
		item.ItemId = v
	}
	if v, ok := source["is_hidden"].(bool); ok {
		item.IsHidden = v
	}
	if v, ok := source["categories"].([]any); ok {
		item.Categories = make([]string, len(v))
		for i, c := range v {
			if s, ok := c.(string); ok {
				item.Categories[i] = s
			}
		}
	}
	if v, ok := source["timestamp"].(string); ok {
		item.Timestamp, _ = time.Parse(time.RFC3339Nano, v)
	}
	item.Labels = source["labels"]
	if v, ok := source["comment"].(string); ok {
		item.Comment = v
	}
	return item
}

func (db *Elasticsearch) parseUser(source map[string]any) User {
	user := User{}
	if v, ok := source["user_id"].(string); ok {
		user.UserId = v
	}
	user.Labels = source["labels"]
	if v, ok := source["comment"].(string); ok {
		user.Comment = v
	}
	return user
}

func (db *Elasticsearch) parseFeedback(source map[string]any) Feedback {
	feedback := Feedback{}
	if v, ok := source["feedback_type"].(string); ok {
		feedback.FeedbackType = v
	}
	if v, ok := source["user_id"].(string); ok {
		feedback.UserId = v
	}
	if v, ok := source["item_id"].(string); ok {
		feedback.ItemId = v
	}
	if v, ok := source["value"].(float64); ok {
		feedback.Value = v
	}
	if v, ok := source["timestamp"].(string); ok {
		feedback.Timestamp, _ = time.Parse(time.RFC3339Nano, v)
	}
	if v, ok := source["updated"].(string); ok {
		feedback.Updated, _ = time.Parse(time.RFC3339Nano, v)
	}
	if v, ok := source["comment"].(string); ok {
		feedback.Comment = v
	}
	return feedback
}

// ModifyItem modify an item in Elasticsearch.
func (db *Elasticsearch) ModifyItem(ctx context.Context, itemId string, patch ItemPatch) error {
	doc := map[string]any{}
	if patch.IsHidden != nil {
		doc["is_hidden"] = patch.IsHidden
	}
	if patch.Categories != nil {
		doc["categories"] = patch.Categories
	}
	if patch.Comment != nil {
		doc["comment"] = patch.Comment
	}
	if patch.Labels != nil {
		doc["labels"] = patch.Labels
	}
	if patch.Timestamp != nil {
		doc["timestamp"] = patch.Timestamp
	}

	body, err := json.Marshal(map[string]any{"doc": doc})
	if err != nil {
		return errors.Trace(err)
	}

	req := esapi.UpdateRequest{
		Index:      db.ItemsTable(),
		DocumentID: itemId,
		Body:       bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return errors.Trace(err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return errors.Errorf("failed to modify item %s: %s", itemId, res.String())
	}
	return nil
}

// DeleteItem deletes an item from Elasticsearch.
func (db *Elasticsearch) DeleteItem(ctx context.Context, itemId string) error {
	req := esapi.DeleteRequest{
		Index:      db.ItemsTable(),
		DocumentID: itemId,
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return errors.Trace(err)
	}
	defer res.Body.Close()

	// Delete associated feedback
	query := map[string]any{
		"query": map[string]any{
			"term": map[string]any{"item_id": itemId},
		},
	}
	body, err := json.Marshal(query)
	if err != nil {
		return errors.Trace(err)
	}
	deleteReq := esapi.DeleteByQueryRequest{
		Index: []string{db.FeedbackTable()},
		Body:  bytes.NewReader(body),
	}
	deleteRes, err := deleteReq.Do(ctx, db.client)
	if err != nil {
		return errors.Trace(err)
	}
	deleteRes.Body.Close()
	return nil
}

// GetItem returns an item from Elasticsearch.
func (db *Elasticsearch) GetItem(ctx context.Context, itemId string) (item Item, err error) {
	req := esapi.GetRequest{
		Index:      db.ItemsTable(),
		DocumentID: itemId,
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return item, errors.Trace(err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return item, errors.Annotate(ErrItemNotExist, itemId)
	}

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return item, errors.Trace(err)
	}

	if source, ok := result["_source"].(map[string]any); ok {
		item = db.parseItem(source)
	}
	return item, nil
}

// GetItems returns items from Elasticsearch.
func (db *Elasticsearch) GetItems(ctx context.Context, cursor string, n int, timeLimit *time.Time) (string, []Item, error) {
	buf, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	cursorItem := string(buf)

	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{},
			},
		},
		"sort": []map[string]any{
			{"item_id": "asc"},
		},
		"size": n,
	}

	if cursorItem != "" {
		query["search_after"] = []string{cursorItem}
	}

	if timeLimit != nil {
		must := query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
		must = append(must, map[string]any{
			"range": map[string]any{"timestamp": map[string]any{"gt": *timeLimit}},
		})
		query["query"].(map[string]any)["bool"].(map[string]any)["must"] = must
	}

	body, err := json.Marshal(query)
	if err != nil {
		return "", nil, errors.Trace(err)
	}

	req := esapi.SearchRequest{
		Index: []string{db.ItemsTable()},
		Body:  bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return "", nil, errors.Trace(err)
	}

	items := make([]Item, 0)
	if hits, ok := result["hits"].(map[string]any); ok {
		if hitList, ok := hits["hits"].([]any); ok {
			for _, hit := range hitList {
				if hitMap, ok := hit.(map[string]any); ok {
					if source, ok := hitMap["_source"].(map[string]any); ok {
						item := db.parseItem(source)
						items = append(items, item)
					}
				}
			}
		}
	}

	if len(items) == n {
		cursor = items[n-1].ItemId
	} else {
		cursor = ""
	}
	return base64.StdEncoding.EncodeToString([]byte(cursor)), items, nil
}

// GetLatestItems returns the latest items from Elasticsearch.
func (db *Elasticsearch) GetLatestItems(ctx context.Context, n int, categories []string, after *time.Time) ([]Item, error) {
	must := []map[string]any{}
	mustNot := []map[string]any{
		{"term": map[string]any{"is_hidden": true}},
	}

	if len(categories) > 0 {
		must = append(must, map[string]any{
			"terms": map[string]any{"categories": categories},
		})
	}

	if after != nil {
		must = append(must, map[string]any{
			"range": map[string]any{"timestamp": map[string]any{"gt": *after}},
		})
	}

	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must":     must,
				"must_not": mustNot,
			},
		},
		"sort": []map[string]any{
			{"timestamp": "desc"},
		},
		"size": n,
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, errors.Trace(err)
	}

	req := esapi.SearchRequest{
		Index: []string{db.ItemsTable()},
		Body:  bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, errors.Trace(err)
	}

	items := make([]Item, 0)
	if hits, ok := result["hits"].(map[string]any); ok {
		if hitList, ok := hits["hits"].([]any); ok {
			for _, hit := range hitList {
				if hitMap, ok := hit.(map[string]any); ok {
					if source, ok := hitMap["_source"].(map[string]any); ok {
						item := db.parseItem(source)
						items = append(items, item)
					}
				}
			}
		}
	}
	return items, nil
}

// GetItemStream read items from Elasticsearch by stream.
func (db *Elasticsearch) GetItemStream(ctx context.Context, batchSize int, timeLimit *time.Time) (chan []Item, chan error) {
	itemChan := make(chan []Item, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(itemChan)
		defer close(errChan)

		query := map[string]any{
			"query": map[string]any{
				"match_all": map[string]any{},
			},
			"sort": []map[string]any{
				{"item_id": "asc"},
			},
			"size": batchSize,
		}

		if timeLimit != nil {
			query["query"] = map[string]any{
				"range": map[string]any{"timestamp": map[string]any{"gt": *timeLimit}},
			}
		}

		var searchAfter []any
		for {
			if searchAfter != nil {
				query["search_after"] = searchAfter
			}

			body, err := json.Marshal(query)
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}

			req := esapi.SearchRequest{
				Index: []string{db.ItemsTable()},
				Body:  bytes.NewReader(body),
			}
			res, err := req.Do(context.Background(), db.client)
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}

			var result map[string]any
			if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
				res.Body.Close()
				errChan <- errors.Trace(err)
				return
			}
			res.Body.Close()

			items := make([]Item, 0)
			if hits, ok := result["hits"].(map[string]any); ok {
				if hitList, ok := hits["hits"].([]any); ok {
					for _, hit := range hitList {
						if hitMap, ok := hit.(map[string]any); ok {
							if source, ok := hitMap["_source"].(map[string]any); ok {
								item := db.parseItem(source)
								items = append(items, item)
								// Update search_after
							}
						}
					}
					if len(hitList) > 0 {
						lastHit := hitList[len(hitList)-1].(map[string]any)
						if sort, ok := lastHit["sort"].([]any); ok {
							searchAfter = sort
						}
					}
				}
			}

			if len(items) == 0 {
				break
			}

			itemChan <- items

			if len(items) < batchSize {
				break
			}
		}
		errChan <- nil
	}()
	return itemChan, errChan
}

// GetItemFeedback returns feedback of an item from Elasticsearch.
func (db *Elasticsearch) GetItemFeedback(ctx context.Context, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					{"term": map[string]any{"item_id": itemId}},
					{"range": map[string]any{"timestamp": map[string]any{"lte": time.Now()}}},
				},
			},
		},
		"size": 10000,
	}

	if len(feedbackTypes) > 0 {
		must := query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
		must = append(must, map[string]any{
			"terms": map[string]any{"feedback_type": feedbackTypes},
		})
		query["query"].(map[string]any)["bool"].(map[string]any)["must"] = must
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, errors.Trace(err)
	}

	req := esapi.SearchRequest{
		Index: []string{db.FeedbackTable()},
		Body:  bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, errors.Trace(err)
	}

	feedbacks := make([]Feedback, 0)
	if hits, ok := result["hits"].(map[string]any); ok {
		if hitList, ok := hits["hits"].([]any); ok {
			for _, hit := range hitList {
				if hitMap, ok := hit.(map[string]any); ok {
					if source, ok := hitMap["_source"].(map[string]any); ok {
						feedback := db.parseFeedback(source)
						feedbacks = append(feedbacks, feedback)
					}
				}
			}
		}
	}
	return feedbacks, nil
}

// BatchInsertUsers inserts users into Elasticsearch.
func (db *Elasticsearch) BatchInsertUsers(ctx context.Context, users []User) error {
	if len(users) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, user := range users {
		action := fmt.Sprintf(`{"index": {"_index": "%s", "_id": "%s"}}`, db.UsersTable(), user.UserId)
		buf.WriteString(action + "\n")

		doc, err := json.Marshal(user)
		if err != nil {
			return errors.Trace(err)
		}
		buf.Write(doc)
		buf.WriteString("\n")
	}

	req := esapi.BulkRequest{
		Body: bytes.NewReader(buf.Bytes()),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return errors.Trace(err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return errors.Errorf("failed to bulk insert users: %s", res.String())
	}
	return nil
}

// ModifyUser modify a user in Elasticsearch.
func (db *Elasticsearch) ModifyUser(ctx context.Context, userId string, patch UserPatch) error {
	doc := map[string]any{}
	if patch.Labels != nil {
		doc["labels"] = patch.Labels
	}
	if patch.Comment != nil {
		doc["comment"] = patch.Comment
	}

	body, err := json.Marshal(map[string]any{"doc": doc})
	if err != nil {
		return errors.Trace(err)
	}

	req := esapi.UpdateRequest{
		Index:      db.UsersTable(),
		DocumentID: userId,
		Body:       bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return errors.Trace(err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return errors.Errorf("failed to modify user %s: %s", userId, res.String())
	}
	return nil
}

// DeleteUser deletes a user from Elasticsearch.
func (db *Elasticsearch) DeleteUser(ctx context.Context, userId string) error {
	req := esapi.DeleteRequest{
		Index:      db.UsersTable(),
		DocumentID: userId,
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return errors.Trace(err)
	}
	res.Body.Close()

	// Delete associated feedback
	query := map[string]any{
		"query": map[string]any{
			"term": map[string]any{"user_id": userId},
		},
	}
	body, err := json.Marshal(query)
	if err != nil {
		return errors.Trace(err)
	}
	deleteReq := esapi.DeleteByQueryRequest{
		Index: []string{db.FeedbackTable()},
		Body:  bytes.NewReader(body),
	}
	deleteRes, err := deleteReq.Do(ctx, db.client)
	if err != nil {
		return errors.Trace(err)
	}
	deleteRes.Body.Close()
	return nil
}

// GetUser returns a user from Elasticsearch.
func (db *Elasticsearch) GetUser(ctx context.Context, userId string) (user User, err error) {
	req := esapi.GetRequest{
		Index:      db.UsersTable(),
		DocumentID: userId,
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return user, errors.Trace(err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return user, errors.Annotate(ErrUserNotExist, userId)
	}

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return user, errors.Trace(err)
	}

	if source, ok := result["_source"].(map[string]any); ok {
		user = db.parseUser(source)
	}
	return user, nil
}

// GetUsers returns users from Elasticsearch.
func (db *Elasticsearch) GetUsers(ctx context.Context, cursor string, n int) (string, []User, error) {
	buf, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	cursorUser := string(buf)

	query := map[string]any{
		"query": map[string]any{
			"match_all": map[string]any{},
		},
		"sort": []map[string]any{
			{"user_id": "asc"},
		},
		"size": n,
	}

	if cursorUser != "" {
		query["search_after"] = []string{cursorUser}
	}

	body, err := json.Marshal(query)
	if err != nil {
		return "", nil, errors.Trace(err)
	}

	req := esapi.SearchRequest{
		Index: []string{db.UsersTable()},
		Body:  bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return "", nil, errors.Trace(err)
	}

	users := make([]User, 0)
	if hits, ok := result["hits"].(map[string]any); ok {
		if hitList, ok := hits["hits"].([]any); ok {
			for _, hit := range hitList {
				if hitMap, ok := hit.(map[string]any); ok {
					if source, ok := hitMap["_source"].(map[string]any); ok {
						user := db.parseUser(source)
						users = append(users, user)
					}
				}
			}
		}
	}

	if len(users) == n {
		cursor = users[n-1].UserId
	} else {
		cursor = ""
	}
	return base64.StdEncoding.EncodeToString([]byte(cursor)), users, nil
}

// GetUserStream reads users from Elasticsearch by stream.
func (db *Elasticsearch) GetUserStream(ctx context.Context, batchSize int) (chan []User, chan error) {
	userChan := make(chan []User, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(userChan)
		defer close(errChan)

		query := map[string]any{
			"query": map[string]any{
				"match_all": map[string]any{},
			},
			"sort": []map[string]any{
				{"user_id": "asc"},
			},
			"size": batchSize,
		}

		var searchAfter []any
		for {
			if searchAfter != nil {
				query["search_after"] = searchAfter
			}

			body, err := json.Marshal(query)
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}

			req := esapi.SearchRequest{
				Index: []string{db.UsersTable()},
				Body:  bytes.NewReader(body),
			}
			res, err := req.Do(context.Background(), db.client)
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}

			var result map[string]any
			if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
				res.Body.Close()
				errChan <- errors.Trace(err)
				return
			}
			res.Body.Close()

			users := make([]User, 0)
			if hits, ok := result["hits"].(map[string]any); ok {
				if hitList, ok := hits["hits"].([]any); ok {
					for _, hit := range hitList {
						if hitMap, ok := hit.(map[string]any); ok {
							if source, ok := hitMap["_source"].(map[string]any); ok {
								user := db.parseUser(source)
								users = append(users, user)
							}
						}
					}
					if len(hitList) > 0 {
						lastHit := hitList[len(hitList)-1].(map[string]any)
						if sort, ok := lastHit["sort"].([]any); ok {
							searchAfter = sort
						}
					}
				}
			}

			if len(users) == 0 {
				break
			}

			userChan <- users

			if len(users) < batchSize {
				break
			}
		}
		errChan <- nil
	}()
	return userChan, errChan
}

// GetUserFeedback returns feedback of a user from Elasticsearch.
func (db *Elasticsearch) GetUserFeedback(ctx context.Context, userId string, endTime *time.Time, feedbackTypes ...expression.FeedbackTypeExpression) ([]Feedback, error) {
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					{"term": map[string]any{"user_id": userId}},
				},
			},
		},
		"size": 10000,
	}

	if endTime != nil {
		must := query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
		must = append(must, map[string]any{
			"range": map[string]any{"timestamp": map[string]any{"lte": endTime}},
		})
		query["query"].(map[string]any)["bool"].(map[string]any)["must"] = must
	}

	if len(feedbackTypes) > 0 {
		var conditions []map[string]any
		for _, feedbackType := range feedbackTypes {
			condition := map[string]any{
				"term": map[string]any{"feedback_type": feedbackType.FeedbackType},
			}
			switch feedbackType.ExprType {
			case expression.Less:
				condition["range"] = map[string]any{"value": map[string]any{"lt": feedbackType.Value}}
			case expression.LessOrEqual:
				condition["range"] = map[string]any{"value": map[string]any{"lte": feedbackType.Value}}
			case expression.Greater:
				condition["range"] = map[string]any{"value": map[string]any{"gt": feedbackType.Value}}
			case expression.GreaterOrEqual:
				condition["range"] = map[string]any{"value": map[string]any{"gte": feedbackType.Value}}
			}
			conditions = append(conditions, condition)
		}
		query["query"].(map[string]any)["bool"].(map[string]any)["must"] = append(
			query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any),
			map[string]any{"bool": map[string]any{"should": conditions}},
		)
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, errors.Trace(err)
	}

	req := esapi.SearchRequest{
		Index: []string{db.FeedbackTable()},
		Body:  bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, errors.Trace(err)
	}

	feedbacks := make([]Feedback, 0)
	if hits, ok := result["hits"].(map[string]any); ok {
		if hitList, ok := hits["hits"].([]any); ok {
			for _, hit := range hitList {
				if hitMap, ok := hit.(map[string]any); ok {
					if source, ok := hitMap["_source"].(map[string]any); ok {
						feedback := db.parseFeedback(source)
						feedbacks = append(feedbacks, feedback)
					}
				}
			}
		}
	}
	return feedbacks, nil
}

// BatchInsertFeedback inserts feedback into Elasticsearch.
func (db *Elasticsearch) BatchInsertFeedback(ctx context.Context, feedback []Feedback, insertUser, insertItem, overwrite bool) error {
	if len(feedback) == 0 {
		return nil
	}

	// Collect users and items
	users := mapset.NewSet[string]()
	items := mapset.NewSet[string]()
	for _, v := range feedback {
		users.Add(v.UserId)
		items.Add(v.ItemId)
	}

	// Insert users
	userList := users.ToSlice()
	if insertUser {
		var buf bytes.Buffer
		for _, userId := range userList {
			action := fmt.Sprintf(`{"index": {"_index": "%s", "_id": "%s"}}`, db.UsersTable(), userId)
			buf.WriteString(action + "\n")
			doc, _ := json.Marshal(User{UserId: userId})
			buf.Write(doc)
			buf.WriteString("\n")
		}
		req := esapi.BulkRequest{
			Body: bytes.NewReader(buf.Bytes()),
		}
		res, err := req.Do(ctx, db.client)
		if err != nil {
			return errors.Trace(err)
		}
		res.Body.Close()
	} else {
		for _, userId := range userList {
			_, err := db.GetUser(ctx, userId)
			if err != nil {
				if errors.Is(err, errors.NotFound) {
					users.Remove(userId)
					continue
				}
				return errors.Trace(err)
			}
		}
	}

	// Insert items
	itemList := items.ToSlice()
	if insertItem {
		var buf bytes.Buffer
		for _, itemId := range itemList {
			action := fmt.Sprintf(`{"index": {"_index": "%s", "_id": "%s"}}`, db.ItemsTable(), itemId)
			buf.WriteString(action + "\n")
			doc, _ := json.Marshal(Item{ItemId: itemId})
			buf.Write(doc)
			buf.WriteString("\n")
		}
		req := esapi.BulkRequest{
			Body: bytes.NewReader(buf.Bytes()),
		}
		res, err := req.Do(ctx, db.client)
		if err != nil {
			return errors.Trace(err)
		}
		res.Body.Close()
	} else {
		for _, itemId := range itemList {
			_, err := db.GetItem(ctx, itemId)
			if err != nil {
				if errors.Is(err, errors.NotFound) {
					items.Remove(itemId)
					continue
				}
				return errors.Trace(err)
			}
		}
	}

	// Insert feedback
	var buf bytes.Buffer
	for _, f := range feedback {
		if users.Contains(f.UserId) && items.Contains(f.ItemId) {
			f.Updated = f.Timestamp
			docID := f.FeedbackType + "_" + f.UserId + "_" + f.ItemId
			action := fmt.Sprintf(`{"index": {"_index": "%s", "_id": "%s"}}`, db.FeedbackTable(), docID)
			buf.WriteString(action + "\n")
			doc, _ := json.Marshal(f)
			buf.Write(doc)
			buf.WriteString("\n")
		}
	}

	if buf.Len() == 0 {
		return nil
	}

	req := esapi.BulkRequest{
		Body: bytes.NewReader(buf.Bytes()),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return errors.Trace(err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return errors.Errorf("failed to bulk insert feedback: %s", res.String())
	}
	return nil
}

// GetFeedback returns feedback from Elasticsearch.
func (db *Elasticsearch) GetFeedback(ctx context.Context, cursor string, n int, beginTime, endTime *time.Time, feedbackTypes ...string) (string, []Feedback, error) {
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{},
			},
		},
		"sort": []map[string]any{
			{"feedback_type": "asc"},
			{"user_id": "asc"},
			{"item_id": "asc"},
		},
		"size": n,
	}

	// Parse cursor
	if cursor != "" {
		buf, err := base64.StdEncoding.DecodeString(cursor)
		if err != nil {
			return "", nil, errors.Trace(err)
		}
		var fk FeedbackKey
		if err := json.Unmarshal(buf, &fk); err != nil {
			return "", nil, errors.Trace(err)
		}
		query["search_after"] = []string{fk.FeedbackType, fk.UserId, fk.ItemId}
	}

	// Add feedback type filter
	if len(feedbackTypes) > 0 {
		must := query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
		must = append(must, map[string]any{
			"terms": map[string]any{"feedback_type": feedbackTypes},
		})
		query["query"].(map[string]any)["bool"].(map[string]any)["must"] = must
	}

	// Add time filter
	if beginTime != nil || endTime != nil {
		timeFilter := map[string]any{}
		if beginTime != nil {
			timeFilter["gt"] = *beginTime
		}
		if endTime != nil {
			timeFilter["lte"] = *endTime
		}
		must := query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
		must = append(must, map[string]any{
			"range": map[string]any{"timestamp": timeFilter},
		})
		query["query"].(map[string]any)["bool"].(map[string]any)["must"] = must
	}

	body, err := json.Marshal(query)
	if err != nil {
		return "", nil, errors.Trace(err)
	}

	req := esapi.SearchRequest{
		Index: []string{db.FeedbackTable()},
		Body:  bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return "", nil, errors.Trace(err)
	}

	feedbacks := make([]Feedback, 0)
	if hits, ok := result["hits"].(map[string]any); ok {
		if hitList, ok := hits["hits"].([]any); ok {
			for _, hit := range hitList {
				if hitMap, ok := hit.(map[string]any); ok {
					if source, ok := hitMap["_source"].(map[string]any); ok {
						feedback := db.parseFeedback(source)
						feedbacks = append(feedbacks, feedback)
					}
				}
			}
		}
	}

	if len(feedbacks) == n {
		fk := feedbacks[n-1].FeedbackKey
		cursorBytes, _ := json.Marshal(fk)
		cursor = base64.StdEncoding.EncodeToString(cursorBytes)
	} else {
		cursor = ""
	}
	return cursor, feedbacks, nil
}

// GetFeedbackStream reads feedback from Elasticsearch by stream.
func (db *Elasticsearch) GetFeedbackStream(ctx context.Context, batchSize int, scanOptions ...ScanOption) (chan []Feedback, chan error) {
	scan := NewScanOptions(scanOptions...)
	feedbackChan := make(chan []Feedback, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(feedbackChan)
		defer close(errChan)

		query := map[string]any{
			"query": map[string]any{
				"bool": map[string]any{
					"must": []map[string]any{},
				},
			},
			"size": batchSize,
		}

		// Add feedback type filter
		if len(scan.FeedbackTypes) > 0 {
			var conditions []map[string]any
			for _, feedbackType := range scan.FeedbackTypes {
				condition := map[string]any{
					"term": map[string]any{"feedback_type": feedbackType.FeedbackType},
				}
				switch feedbackType.ExprType {
				case expression.Less:
					condition["range"] = map[string]any{"value": map[string]any{"lt": feedbackType.Value}}
				case expression.LessOrEqual:
					condition["range"] = map[string]any{"value": map[string]any{"lte": feedbackType.Value}}
				case expression.Greater:
					condition["range"] = map[string]any{"value": map[string]any{"gt": feedbackType.Value}}
				case expression.GreaterOrEqual:
					condition["range"] = map[string]any{"value": map[string]any{"gte": feedbackType.Value}}
				}
				conditions = append(conditions, condition)
			}
			query["query"].(map[string]any)["bool"].(map[string]any)["must"] = append(
				query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any),
				map[string]any{"bool": map[string]any{"should": conditions}},
			)
		}

		// Add time filter
		if scan.BeginTime != nil || scan.EndTime != nil {
			timeFilter := map[string]any{}
			if scan.BeginTime != nil {
				timeFilter["gt"] = *scan.BeginTime
			}
			if scan.EndTime != nil {
				timeFilter["lte"] = *scan.EndTime
			}
			must := query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
			must = append(must, map[string]any{
				"range": map[string]any{"timestamp": timeFilter},
			})
			query["query"].(map[string]any)["bool"].(map[string]any)["must"] = must
		}

		// Add user id filter
		if scan.BeginUserId != nil || scan.EndUserId != nil {
			userFilter := map[string]any{}
			if scan.BeginUserId != nil {
				userFilter["gte"] = *scan.BeginUserId
			}
			if scan.EndUserId != nil {
				userFilter["lte"] = *scan.EndUserId
			}
			must := query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
			must = append(must, map[string]any{
				"range": map[string]any{"user_id": userFilter},
			})
			query["query"].(map[string]any)["bool"].(map[string]any)["must"] = must
		}

		// Add item id filter
		if scan.BeginItemId != nil || scan.EndItemId != nil {
			itemFilter := map[string]any{}
			if scan.BeginItemId != nil {
				itemFilter["gte"] = *scan.BeginItemId
			}
			if scan.EndItemId != nil {
				itemFilter["lte"] = *scan.EndItemId
			}
			must := query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
			must = append(must, map[string]any{
				"range": map[string]any{"item_id": itemFilter},
			})
			query["query"].(map[string]any)["bool"].(map[string]any)["must"] = must
		}

		// Add sort
		if scan.OrderByItemId {
			query["sort"] = []map[string]any{
				{"item_id": "asc"},
			}
		} else {
			query["sort"] = []map[string]any{
				{"feedback_type": "asc"},
				{"user_id": "asc"},
				{"item_id": "asc"},
			}
		}

		var searchAfter []any
		for {
			if searchAfter != nil {
				query["search_after"] = searchAfter
			}

			body, err := json.Marshal(query)
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}

			req := esapi.SearchRequest{
				Index: []string{db.FeedbackTable()},
				Body:  bytes.NewReader(body),
			}
			res, err := req.Do(context.Background(), db.client)
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}

			var result map[string]any
			if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
				res.Body.Close()
				errChan <- errors.Trace(err)
				return
			}
			res.Body.Close()

			feedbacks := make([]Feedback, 0)
			if hits, ok := result["hits"].(map[string]any); ok {
				if hitList, ok := hits["hits"].([]any); ok {
					for _, hit := range hitList {
						if hitMap, ok := hit.(map[string]any); ok {
							if source, ok := hitMap["_source"].(map[string]any); ok {
								feedback := db.parseFeedback(source)
								feedbacks = append(feedbacks, feedback)
							}
						}
					}
					if len(hitList) > 0 {
						lastHit := hitList[len(hitList)-1].(map[string]any)
						if sort, ok := lastHit["sort"].([]any); ok {
							searchAfter = sort
						}
					}
				}
			}

			if len(feedbacks) == 0 {
				break
			}

			feedbackChan <- feedbacks

			if len(feedbacks) < batchSize {
				break
			}
		}
		errChan <- nil
	}()
	return feedbackChan, errChan
}

// GetUserItemFeedback returns feedback for a user-item pair from Elasticsearch.
func (db *Elasticsearch) GetUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					{"term": map[string]any{"user_id": userId}},
					{"term": map[string]any{"item_id": itemId}},
				},
			},
		},
	}

	if len(feedbackTypes) > 0 {
		must := query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
		must = append(must, map[string]any{
			"terms": map[string]any{"feedback_type": feedbackTypes},
		})
		query["query"].(map[string]any)["bool"].(map[string]any)["must"] = must
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, errors.Trace(err)
	}

	req := esapi.SearchRequest{
		Index: []string{db.FeedbackTable()},
		Body:  bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, errors.Trace(err)
	}

	feedbacks := make([]Feedback, 0)
	if hits, ok := result["hits"].(map[string]any); ok {
		if hitList, ok := hits["hits"].([]any); ok {
			for _, hit := range hitList {
				if hitMap, ok := hit.(map[string]any); ok {
					if source, ok := hitMap["_source"].(map[string]any); ok {
						feedback := db.parseFeedback(source)
						feedbacks = append(feedbacks, feedback)
					}
				}
			}
		}
	}
	return feedbacks, nil
}

// DeleteUserItemFeedback deletes feedback for a user-item pair from Elasticsearch.
func (db *Elasticsearch) DeleteUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) (int, error) {
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					{"term": map[string]any{"user_id": userId}},
					{"term": map[string]any{"item_id": itemId}},
				},
			},
		},
	}

	if len(feedbackTypes) > 0 {
		must := query["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
		must = append(must, map[string]any{
			"terms": map[string]any{"feedback_type": feedbackTypes},
		})
		query["query"].(map[string]any)["bool"].(map[string]any)["must"] = must
	}

	body, err := json.Marshal(query)
	if err != nil {
		return 0, errors.Trace(err)
	}

	req := esapi.DeleteByQueryRequest{
		Index: []string{db.FeedbackTable()},
		Body:  bytes.NewReader(body),
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return 0, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, errors.Trace(err)
	}

	var deleted int
	if v, ok := result["deleted"].(float64); ok {
		deleted = int(v)
	}
	return deleted, nil
}

func (db *Elasticsearch) CountUsers(ctx context.Context) (int, error) {
	req := esapi.CountRequest{
		Index: []string{db.UsersTable()},
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return 0, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, errors.Trace(err)
	}

	var count int
	if v, ok := result["count"].(float64); ok {
		count = int(v)
	}
	return count, nil
}

func (db *Elasticsearch) CountItems(ctx context.Context) (int, error) {
	req := esapi.CountRequest{
		Index: []string{db.ItemsTable()},
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return 0, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, errors.Trace(err)
	}

	var count int
	if v, ok := result["count"].(float64); ok {
		count = int(v)
	}
	return count, nil
}

func (db *Elasticsearch) CountFeedback(ctx context.Context) (int, error) {
	req := esapi.CountRequest{
		Index: []string{db.FeedbackTable()},
	}
	res, err := req.Do(ctx, db.client)
	if err != nil {
		return 0, errors.Trace(err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, errors.Trace(err)
	}

	var count int
	if v, ok := result["count"].(float64); ok {
		count = int(v)
	}
	return count, nil
}