// Copyright 2026 gorse Project Authors
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

package main

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// AdminClient is a client for the Gorse admin API.
type AdminClient struct {
	client *resty.Client
}

type Status struct {
	BinaryVersion          string
	NumServers             int
	NumWorkers             int
	NumUsers               int
	NumItems               int
	NumUserLabels          int
	NumItemLabels          int
	NumTotalPosFeedback    int
	NumValidPosFeedback    int
	NumValidNegFeedback    int
	PopularItemsUpdateTime time.Time
	LatestItemsUpdateTime  time.Time
	MatchingModelFitTime   time.Time
	MatchingModelScore     any
	RankingModelFitTime    time.Time
	RankingModelScore      any
}

type FeedbackIterator struct {
	Cursor   string
	Feedback []Feedback
}

type Node struct {
	UUID       string
	Hostname   string
	Type       string
	Version    string
	UpdateTime time.Time
}

type Progress struct {
	Tracer     string
	Name       string
	Status     string
	Error      string
	Count      int
	Total      int
	StartTime  time.Time
	FinishTime time.Time
}

type Item struct {
	ItemId     string
	IsHidden   bool
	Categories []string
	Timestamp  time.Time
	Labels     any
	Comment    string
}

type User struct {
	UserId  string
	Labels  any
	Comment string
}

type FeedbackKey struct {
	FeedbackType string
	UserId       string
	ItemId       string
}

type Feedback struct {
	FeedbackKey
	Value     float64
	Timestamp time.Time
	Updated   time.Time
	Labels    any
	Comment   string
}

type ScoredItem struct {
	Item
	Score float64
}

type ScoreUser struct {
	User
	Score float64
}

type DumpStats struct {
	Users    int
	Items    int
	Feedback int
	Duration time.Duration
}

func sortFeedback(feedback []Feedback) {
	sort.Slice(feedback, func(i, j int) bool {
		return feedback[i].Timestamp.After(feedback[j].Timestamp)
	})
}

// NewAdminClient creates a new client for the Gorse admin API.
func NewAdminClient(endpoint, apiKey string) *AdminClient {
	client := resty.New()
	client.SetBaseURL(strings.TrimRight(endpoint, "/") + "/api")
	client.SetHeader("X-Api-Key", apiKey)
	return &AdminClient{client: client}
}

func (c *AdminClient) GetCluster() ([]Node, error) {
	return getJSON[[]Node](c, "/dashboard/cluster", nil)
}

func (c *AdminClient) GetTasks() ([]Progress, error) {
	return getJSON[[]Progress](c, "/dashboard/tasks", nil)
}

func (c *AdminClient) GetConfig() (map[string]any, error) {
	return getJSON[map[string]any](c, "/dashboard/config", nil)
}

func (c *AdminClient) GetConfigMap() (map[string]any, error) {
	return getJSON[map[string]any](c, "/dashboard/config", nil)
}

func (c *AdminClient) GetConfigSchema() (map[string]any, error) {
	return getJSON[map[string]any](c, "/dashboard/config/schema", nil)
}

func (c *AdminClient) UpdateConfig(configPatch map[string]any) (map[string]any, error) {
	var result map[string]any
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(configPatch).
		SetResult(&result).
		Post("/dashboard/config")
	if err != nil {
		return result, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return result, newAdminAPIError(resp)
	}
	return result, nil
}

func (c *AdminClient) ResetConfig() (map[string]any, error) {
	var result map[string]any
	resp, err := c.client.R().
		SetResult(&result).
		Delete("/dashboard/config")
	if err != nil {
		return result, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return result, newAdminAPIError(resp)
	}
	return result, nil
}

func getConfigValue(configMap map[string]any, names ...string) (string, any, bool) {
	for _, name := range names {
		value, ok := configMap[name]
		if ok {
			return name, value, true
		}
	}
	return "", nil, false
}

func formatConfigMap(configMap map[string]any) map[string]any {
	formatted := make(map[string]any, len(configMap))
	for key, value := range configMap {
		formatted[key] = formatConfigValue(value)
	}
	return formatted
}

func formatConfigValue(value any) any {
	switch v := value.(type) {
	case time.Duration:
		s := v.String()
		if strings.HasSuffix(s, "m0s") {
			s = s[:len(s)-2]
		}
		if strings.HasSuffix(s, "h0m") {
			s = s[:len(s)-2]
		}
		return s
	case map[string]any:
		return formatConfigMap(v)
	case []any:
		formatted := make([]any, len(v))
		for i, item := range v {
			formatted[i] = formatConfigValue(item)
		}
		return formatted
	default:
		return v
	}
}

func (c *AdminClient) GetCategories() ([]string, error) {
	return getJSON[[]string](c, "/dashboard/categories", nil)
}

func (c *AdminClient) GetStats() (Status, error) {
	return getJSON[Status](c, "/dashboard/stats", nil)
}

func (c *AdminClient) GetFeedback(n int) (FeedbackIterator, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	return getJSON[FeedbackIterator](c, "/feedback", params)
}

func (c *AdminClient) GetTypedFeedback(feedbackType string, n int) (FeedbackIterator, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	return getJSON[FeedbackIterator](c, "/feedback/"+url.PathEscape(feedbackType), params)
}

func (c *AdminClient) GetUserItemFeedback(userID, itemID string) ([]Feedback, error) {
	return getJSON[[]Feedback](c, "/feedback/"+url.PathEscape(userID)+"/"+url.PathEscape(itemID), nil)
}

func (c *AdminClient) GetTypedUserItemFeedback(feedbackType, userID, itemID string) (Feedback, error) {
	return getJSON[Feedback](c, "/feedback/"+url.PathEscape(feedbackType)+"/"+url.PathEscape(userID)+"/"+url.PathEscape(itemID), nil)
}

func (c *AdminClient) GetUserFeedback(userID string) ([]Feedback, error) {
	return getJSON[[]Feedback](c, "/user/"+url.PathEscape(userID)+"/feedback", nil)
}

func (c *AdminClient) GetTypedUserFeedback(userID, feedbackType string) ([]Feedback, error) {
	return getJSON[[]Feedback](c, "/user/"+url.PathEscape(userID)+"/feedback/"+url.PathEscape(feedbackType), nil)
}

func (c *AdminClient) GetItemFeedback(itemID string) ([]Feedback, error) {
	return getJSON[[]Feedback](c, "/item/"+url.PathEscape(itemID)+"/feedback/", nil)
}

func (c *AdminClient) GetTypedItemFeedback(itemID, feedbackType string) ([]Feedback, error) {
	return getJSON[[]Feedback](c, "/item/"+url.PathEscape(itemID)+"/feedback/"+url.PathEscape(feedbackType), nil)
}

func (c *AdminClient) GetLatest(n int, categories []string) ([]ScoredItem, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	addStringArrayParam(params, "category", categories)
	return getJSON[[]ScoredItem](c, "/dashboard/latest", params)
}

func (c *AdminClient) GetNonPersonalized(name string, n int, userID string, categories []string) ([]ScoredItem, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	addStringParam(params, "user-id", userID)
	addStringArrayParam(params, "category", categories)
	return getJSON[[]ScoredItem](c, "/dashboard/non-personalized/"+url.PathEscape(name), params)
}

func (c *AdminClient) GetRecommend(userID, recommender, name string, n int, categories []string) ([]ScoredItem, error) {
	path := "/dashboard/recommend/" + url.PathEscape(userID)
	if recommender != "" {
		path += "/" + url.PathEscape(recommender)
	}
	if name != "" {
		path += "/" + url.PathEscape(name)
	}
	params := url.Values{}
	addIntParam(params, "n", n)
	addStringArrayParam(params, "category", categories)
	return getJSON[[]ScoredItem](c, path, params)
}

func (c *AdminClient) GetItemToItem(name, itemID string, n int, categories []string) ([]ScoredItem, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	addStringArrayParam(params, "category", categories)
	return getJSON[[]ScoredItem](c, "/dashboard/item-to-item/"+url.PathEscape(name)+"/"+url.PathEscape(itemID), params)
}

func (c *AdminClient) GetUserToUser(name, userID string, n int) ([]ScoreUser, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	return getJSON[[]ScoreUser](c, "/dashboard/user-to-user/"+url.PathEscape(name)+"/"+url.PathEscape(userID), params)
}

func (c *AdminClient) Restore(reader io.Reader) (*DumpStats, error) {
	result := new(DumpStats)
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/octet-stream").
		SetBody(reader).
		SetResult(result).
		Post("/restore")
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return nil, newAdminAPIError(resp)
	}
	return result, nil
}

func (c *AdminClient) Dump(output io.Writer) error {
	resp, err := c.client.R().
		SetDoNotParseResponse(true).
		Get("/dump")
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.RawBody().Close()
	if resp.IsError() {
		body, _ := io.ReadAll(resp.RawBody())
		return fmt.Errorf("API request failed: status=%d body=%s", resp.StatusCode(), string(body))
	}
	if _, err = io.Copy(output, resp.RawBody()); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	return nil
}

func getJSON[T any](c *AdminClient, path string, params url.Values) (T, error) {
	var result T
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	resp, err := c.client.R().
		SetResult(&result).
		Get(path)
	if err != nil {
		return result, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return result, newAdminAPIError(resp)
	}
	return result, nil
}

func addIntParam(params url.Values, name string, value int) {
	if value != 0 {
		params.Set(name, fmt.Sprint(value))
	}
}

func addStringParam(params url.Values, name, value string) {
	if value != "" {
		params.Set(name, value)
	}
}

func addStringArrayParam(params url.Values, name string, values []string) {
	for _, value := range values {
		params.Add(name, value)
	}
}

func newAdminAPIError(resp *resty.Response) error {
	return fmt.Errorf("API request failed: status=%d body=%s", resp.StatusCode(), resp.String())
}
