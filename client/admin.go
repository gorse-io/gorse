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

package client

import (
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	client "github.com/gorse-io/gorse-go"
)

// AdminClient is a client for the Gorse admin API.
type AdminClient struct {
	*client.GorseClient
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

type TimeSeriesPoint struct {
	Name      string
	Timestamp time.Time
	Value     float64
}

// NewAdminClient creates a new client for the Gorse admin API.
func NewAdminClient(endpoint, apiKey string) *AdminClient {
	endpoint = strings.TrimRight(endpoint, "/")
	adminClient := resty.New()
	adminClient.SetBaseURL(endpoint + "/api")
	adminClient.SetHeader("X-Api-Key", apiKey)
	return &AdminClient{
		GorseClient: client.NewGorseClient(endpoint, apiKey),
		client:      adminClient,
	}
}

func (c *AdminClient) GetCluster() ([]Node, error) {
	return get[[]Node](c, "/dashboard/cluster", nil)
}

func (c *AdminClient) GetTasks() ([]Progress, error) {
	return get[[]Progress](c, "/dashboard/tasks", nil)
}

func (c *AdminClient) GetConfig() (map[string]any, error) {
	return get[map[string]any](c, "/dashboard/config", nil)
}

func (c *AdminClient) GetConfigMap() (map[string]any, error) {
	return get[map[string]any](c, "/dashboard/config", nil)
}

func (c *AdminClient) GetConfigSchema() (map[string]any, error) {
	return get[map[string]any](c, "/dashboard/config/schema", nil)
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

func (c *AdminClient) GetCategories() ([]string, error) {
	return get[[]string](c, "/dashboard/categories", nil)
}

func (c *AdminClient) GetStats() (Status, error) {
	return get[Status](c, "/dashboard/stats", nil)
}

func (c *AdminClient) GetTimeseries(name, begin, end, duration string) ([]TimeSeriesPoint, error) {
	params := url.Values{}
	if begin != "" {
		params.Set("begin", begin)
	}
	if end != "" {
		params.Set("end", end)
	}
	if duration != "" {
		params.Set("duration", duration)
	}
	return get[[]TimeSeriesPoint](c, "/dashboard/timeseries/"+url.PathEscape(name), params)
}

func (c *AdminClient) GetFeedback(n int) (FeedbackIterator, error) {
	params := url.Values{}
	if n != 0 {
		params.Set("n", fmt.Sprint(n))
	}
	return get[FeedbackIterator](c, "/feedback", params)
}

func (c *AdminClient) GetTypedFeedback(feedbackType string, n int) (FeedbackIterator, error) {
	params := url.Values{}
	if n != 0 {
		params.Set("n", fmt.Sprint(n))
	}
	return get[FeedbackIterator](c, "/feedback/"+url.PathEscape(feedbackType), params)
}

func (c *AdminClient) GetUserItemFeedback(userID, itemID string) ([]Feedback, error) {
	return get[[]Feedback](c, "/feedback/"+url.PathEscape(userID)+"/"+url.PathEscape(itemID), nil)
}

func (c *AdminClient) GetTypedUserItemFeedback(feedbackType, userID, itemID string) (Feedback, error) {
	return get[Feedback](c, "/feedback/"+url.PathEscape(feedbackType)+"/"+url.PathEscape(userID)+"/"+url.PathEscape(itemID), nil)
}

func (c *AdminClient) GetUserFeedback(userID string) ([]Feedback, error) {
	return get[[]Feedback](c, "/user/"+url.PathEscape(userID)+"/feedback", nil)
}

func (c *AdminClient) GetTypedUserFeedback(userID, feedbackType string) ([]Feedback, error) {
	return get[[]Feedback](c, "/user/"+url.PathEscape(userID)+"/feedback/"+url.PathEscape(feedbackType), nil)
}

func (c *AdminClient) GetItemFeedback(itemID string) ([]Feedback, error) {
	return get[[]Feedback](c, "/item/"+url.PathEscape(itemID)+"/feedback/", nil)
}

func (c *AdminClient) GetTypedItemFeedback(itemID, feedbackType string) ([]Feedback, error) {
	return get[[]Feedback](c, "/item/"+url.PathEscape(itemID)+"/feedback/"+url.PathEscape(feedbackType), nil)
}

func (c *AdminClient) GetLatest(n int, categories []string) ([]ScoredItem, error) {
	params := url.Values{}
	if n != 0 {
		params.Set("n", fmt.Sprint(n))
	}
	for _, category := range categories {
		params.Add("category", category)
	}
	return get[[]ScoredItem](c, "/dashboard/latest", params)
}

func (c *AdminClient) GetNonPersonalized(name string, n int, userID string, categories []string) ([]ScoredItem, error) {
	params := url.Values{}
	if n != 0 {
		params.Set("n", fmt.Sprint(n))
	}
	if userID != "" {
		params.Set("user-id", userID)
	}
	for _, category := range categories {
		params.Add("category", category)
	}
	return get[[]ScoredItem](c, "/dashboard/non-personalized/"+url.PathEscape(name), params)
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
	if n != 0 {
		params.Set("n", fmt.Sprint(n))
	}
	for _, category := range categories {
		params.Add("category", category)
	}
	return get[[]ScoredItem](c, path, params)
}

func (c *AdminClient) GetItemToItem(name, itemID string, n int, categories []string) ([]ScoredItem, error) {
	params := url.Values{}
	if n != 0 {
		params.Set("n", fmt.Sprint(n))
	}
	for _, category := range categories {
		params.Add("category", category)
	}
	return get[[]ScoredItem](c, "/dashboard/item-to-item/"+url.PathEscape(name)+"/"+url.PathEscape(itemID), params)
}

func (c *AdminClient) GetUserToUser(name, userID string, n int) ([]ScoreUser, error) {
	params := url.Values{}
	if n != 0 {
		params.Set("n", fmt.Sprint(n))
	}
	return get[[]ScoreUser](c, "/dashboard/user-to-user/"+url.PathEscape(name)+"/"+url.PathEscape(userID), params)
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

func get[T any](c *AdminClient, path string, params url.Values) (T, error) {
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

func newAdminAPIError(resp *resty.Response) error {
	return fmt.Errorf("API request failed: status=%d body=%s", resp.StatusCode(), resp.String())
}
