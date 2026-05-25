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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/server"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/meta"
)

// AdminClient is a client for the Gorse admin API.
type AdminClient struct {
	client *resty.Client
}

// NewAdminClient creates a new client for the Gorse admin API.
func NewAdminClient(endpoint, apiKey string) *AdminClient {
	client := resty.New()
	client.SetBaseURL(strings.TrimRight(endpoint, "/") + "/api")
	client.SetHeader("X-Api-Key", apiKey)
	return &AdminClient{client: client}
}

func (c *AdminClient) GetCluster() ([]*meta.Node, error) {
	return getJSON[[]*meta.Node](c, "/dashboard/cluster", nil)
}

func (c *AdminClient) GetTasks() ([]monitor.Progress, error) {
	return getJSON[[]monitor.Progress](c, "/dashboard/tasks", nil)
}

func (c *AdminClient) GetConfig() (map[string]any, error) {
	return getJSON[map[string]any](c, "/dashboard/config", nil)
}

func (c *AdminClient) UpdateConfig(configPatch map[string]any) (config.Config, error) {
	return postJSON[config.Config](c, "/dashboard/config", configPatch)
}

func (c *AdminClient) ResetConfig() (map[string]any, error) {
	return deleteJSON[map[string]any](c, "/dashboard/config")
}

func (c *AdminClient) GetCategories() ([]string, error) {
	return getJSON[[]string](c, "/dashboard/categories", nil)
}

func (c *AdminClient) GetStats() (master.Status, error) {
	return getJSON[master.Status](c, "/dashboard/stats", nil)
}

func (c *AdminClient) GetTimeseries(name, begin, end, duration string) ([]cache.TimeSeriesPoint, error) {
	params := url.Values{}
	addStringParam(params, "begin", begin)
	addStringParam(params, "end", end)
	addStringParam(params, "duration", duration)
	return getJSON[[]cache.TimeSeriesPoint](c, "/dashboard/timeseries/"+url.PathEscape(name), params)
}

func (c *AdminClient) GetFeedback(n int) (server.FeedbackIterator, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	return getJSON[server.FeedbackIterator](c, "/feedback", params)
}

func (c *AdminClient) GetTypedFeedback(feedbackType string, n int) (server.FeedbackIterator, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	return getJSON[server.FeedbackIterator](c, "/feedback/"+url.PathEscape(feedbackType), params)
}

func (c *AdminClient) GetUserItemFeedback(userID, itemID string) ([]data.Feedback, error) {
	return getJSON[[]data.Feedback](c, "/feedback/"+url.PathEscape(userID)+"/"+url.PathEscape(itemID), nil)
}

func (c *AdminClient) GetTypedUserItemFeedback(feedbackType, userID, itemID string) (data.Feedback, error) {
	return getJSON[data.Feedback](c, "/feedback/"+url.PathEscape(feedbackType)+"/"+url.PathEscape(userID)+"/"+url.PathEscape(itemID), nil)
}

func (c *AdminClient) GetUserFeedback(userID string) ([]data.Feedback, error) {
	return getJSON[[]data.Feedback](c, "/user/"+url.PathEscape(userID)+"/feedback", nil)
}

func (c *AdminClient) GetTypedUserFeedback(userID, feedbackType string) ([]data.Feedback, error) {
	return getJSON[[]data.Feedback](c, "/user/"+url.PathEscape(userID)+"/feedback/"+url.PathEscape(feedbackType), nil)
}

func (c *AdminClient) GetItemFeedback(itemID string) ([]data.Feedback, error) {
	return getJSON[[]data.Feedback](c, "/item/"+url.PathEscape(itemID)+"/feedback/", nil)
}

func (c *AdminClient) GetTypedItemFeedback(itemID, feedbackType string) ([]data.Feedback, error) {
	return getJSON[[]data.Feedback](c, "/item/"+url.PathEscape(itemID)+"/feedback/"+url.PathEscape(feedbackType), nil)
}

func (c *AdminClient) GetLatest(n, offset int, categories []string) ([]master.ScoredItem, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	addIntParam(params, "offset", offset)
	addStringArrayParam(params, "category", categories)
	return getJSON[[]master.ScoredItem](c, "/dashboard/latest", params)
}

func (c *AdminClient) GetNonPersonalized(name string, n, offset int, userID string, categories []string) ([]master.ScoredItem, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	addIntParam(params, "offset", offset)
	addStringParam(params, "user-id", userID)
	addStringArrayParam(params, "category", categories)
	return getJSON[[]master.ScoredItem](c, "/dashboard/non-personalized/"+url.PathEscape(name), params)
}

func (c *AdminClient) GetRecommend(userID, recommender, name string, n int, categories []string) ([]master.ScoredItem, error) {
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
	return getJSON[[]master.ScoredItem](c, path, params)
}

func (c *AdminClient) GetItemToItem(name, itemID string, n, offset int, categories []string) ([]master.ScoredItem, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	addIntParam(params, "offset", offset)
	addStringArrayParam(params, "category", categories)
	return getJSON[[]master.ScoredItem](c, "/dashboard/item-to-item/"+url.PathEscape(name)+"/"+url.PathEscape(itemID), params)
}

func (c *AdminClient) GetUserToUser(name, userID string, n, offset int) ([]master.ScoreUser, error) {
	params := url.Values{}
	addIntParam(params, "n", n)
	addIntParam(params, "offset", offset)
	return getJSON[[]master.ScoreUser](c, "/dashboard/user-to-user/"+url.PathEscape(name)+"/"+url.PathEscape(userID), params)
}

func (c *AdminClient) GetExternal(script, userID string) ([]string, error) {
	params := url.Values{}
	addStringParam(params, "script", base64.StdEncoding.EncodeToString([]byte(script)))
	addStringParam(params, "user-id", userID)
	return getJSON[[]string](c, "/dashboard/external", params)
}

func (c *AdminClient) GetRankerPrompt(queryTemplate, documentTemplate, userID string) (master.RerankerPrompt, error) {
	params := url.Values{}
	addStringParam(params, "query-template", base64.StdEncoding.EncodeToString([]byte(queryTemplate)))
	addStringParam(params, "document-template", base64.StdEncoding.EncodeToString([]byte(documentTemplate)))
	addStringParam(params, "user-id", userID)
	return getJSON[master.RerankerPrompt](c, "/dashboard/ranker/prompt", params)
}

func (c *AdminClient) Restore(reader io.Reader) (*master.DumpStats, error) {
	return postOctetStream[master.DumpStats](c, "/restore", reader)
}

func (c *AdminClient) Dump(output io.Writer) error {
	return c.download("/dump", output)
}

func getJSON[T any](c *AdminClient, path string, params url.Values) (T, error) {
	var result T
	body, err := c.get(path, params)
	if err != nil {
		return result, err
	}
	if err = decodeJSON(body, &result); err != nil {
		return result, err
	}
	return result, nil
}

func postJSON[T any](c *AdminClient, path string, body any) (T, error) {
	var result T
	respBody, err := c.postJSON(path, body)
	if err != nil {
		return result, err
	}
	if err = decodeJSON(respBody, &result); err != nil {
		return result, err
	}
	return result, nil
}

func deleteJSON[T any](c *AdminClient, path string) (T, error) {
	var result T
	body, err := c.delete(path)
	if err != nil {
		return result, err
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return result, nil
	}
	if err = decodeJSON(body, &result); err != nil {
		return result, err
	}
	return result, nil
}

func postOctetStream[T any](c *AdminClient, path string, reader io.Reader) (*T, error) {
	result := new(T)
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/octet-stream").
		SetBody(reader).
		SetResult(result).
		Post(path)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return nil, newAdminAPIError(resp)
	}
	return result, nil
}

func decodeJSON(body []byte, value any) error {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	return decoder.Decode(value)
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

func (c *AdminClient) get(path string, params url.Values) ([]byte, error) {
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	resp, err := c.client.R().Get(path)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return nil, newAdminAPIError(resp)
	}
	return resp.Body(), nil
}

func (c *AdminClient) postJSON(path string, body any) ([]byte, error) {
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(path)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return nil, newAdminAPIError(resp)
	}
	return resp.Body(), nil
}

func (c *AdminClient) delete(path string) ([]byte, error) {
	resp, err := c.client.R().Delete(path)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return nil, newAdminAPIError(resp)
	}
	return resp.Body(), nil
}

func (c *AdminClient) download(path string, output io.Writer) error {
	resp, err := c.client.R().
		SetDoNotParseResponse(true).
		Get(path)
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

func newAdminAPIError(resp *resty.Response) error {
	return fmt.Errorf("API request failed: status=%d body=%s", resp.StatusCode(), resp.String())
}
