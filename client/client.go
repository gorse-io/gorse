// Copyright 2022 gorse Project Authors
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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type GorseClient struct {
	entryPoint string
	apiKey     string
	httpClient http.Client
}

func NewGorseClient(entryPoint, apiKey string) *GorseClient {
	return &GorseClient{
		entryPoint: entryPoint,
		apiKey:     apiKey,
		httpClient: http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

func (c *GorseClient) InsertFeedback(ctx context.Context, feedbacks []Feedback) (RowAffected, error) {
	return request[RowAffected](ctx, c, "POST", c.entryPoint+"/api/feedback", feedbacks)
}

func (c *GorseClient) PutFeedback(ctx context.Context, feedbacks []Feedback) (RowAffected, error) {
	return request[RowAffected](ctx, c, "PUT", c.entryPoint+"/api/feedback", feedbacks)
}

func (c *GorseClient) GetFeedback(ctx context.Context, cursor string, n int) (Feedbacks, error) {
	return request[Feedbacks, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/feedback?cursor=%s&n=%d", cursor, n), nil)
}

func (c *GorseClient) GetFeedbacksWithType(ctx context.Context, feedbackType, cursor string, n int) (Feedbacks, error) {
	return request[Feedbacks, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/feedback/%s?cursor=%s&n=%d", feedbackType, cursor, n), nil)
}

func (c *GorseClient) GetFeedbackWithUserItem(ctx context.Context, userId, itemId string) ([]Feedback, error) {
	return request[[]Feedback, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/feedback/%s/%s", userId, itemId), nil)
}

func (c *GorseClient) GetFeedbackWithTypeUserItem(ctx context.Context, feedbackType, userId, itemId string) (Feedback, error) {
	return request[Feedback, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/feedback/%s/%s/%s", feedbackType, userId, itemId), nil)
}

func (c *GorseClient) DelFeedback(ctx context.Context, feedbackType, userId, itemId string) (Feedback, error) {
	return request[Feedback, any](ctx, c, "DELETE", c.entryPoint+fmt.Sprintf("/api/feedback/%s/%s/%s", feedbackType, userId, itemId), nil)
}

func (c *GorseClient) DelFeedbackWithUserItem(ctx context.Context, userId, itemId string) ([]Feedback, error) {
	return request[[]Feedback, any](ctx, c, "DELETE", c.entryPoint+fmt.Sprintf("/api/feedback/%s/%s", userId, itemId), nil)
}

func (c *GorseClient) GetItemFeedbacks(ctx context.Context, itemId string) ([]Feedback, error) {
	return request[[]Feedback, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/item/%s/feedback", itemId), nil)
}

func (c *GorseClient) GetItemFeedbacksWithType(ctx context.Context, itemId, feedbackType string) ([]Feedback, error) {
	return request[[]Feedback, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/item/%s/feedback/%s", itemId, feedbackType), nil)
}

func (c *GorseClient) GetUserFeedbacks(ctx context.Context, userId string) ([]Feedback, error) {
	return request[[]Feedback, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/user/%s/feedback", userId), nil)
}

func (c *GorseClient) GetUserFeedbacksWithType(ctx context.Context, userId, feedbackType string) ([]Feedback, error) {
	return request[[]Feedback, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/user/%s/feedback/%s", userId, feedbackType), nil)
}

// Deprecated: GetUserFeedbacksWithType instead
func (c *GorseClient) ListFeedbacks(ctx context.Context, feedbackType, userId string) ([]Feedback, error) {
	return request[[]Feedback, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/user/%s/feedback/%s", userId, feedbackType), nil)
}

func (c *GorseClient) GetItemLatest(ctx context.Context, userid string, n, offset int) ([]Score, error) {
	return request[[]Score, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/latest?user-id=%s&n=%d&offset=%d", userid, n, offset), nil)
}

func (c *GorseClient) GetItemLatestWithCategory(ctx context.Context, userid, category string, n, offset int) ([]Score, error) {
	return request[[]Score, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/latest?user-id=%s&category=%s&n=%d&offset=%d", userid, category, n, offset), nil)
}

func (c *GorseClient) GetItemPopular(ctx context.Context, userid string, n, offset int) ([]Score, error) {
	return request[[]Score, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/popular?user-id=%s&n=%d&offset=%d", userid, n, offset), nil)
}

func (c *GorseClient) GetItemPopularWithCategory(ctx context.Context, userid, category string, n, offset int) ([]Score, error) {
	return request[[]Score, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/popular/%s?user-id=%s&n=%d&offset=%d", category, userid, n, offset), nil)
}

func (c *GorseClient) GetItemRecommend(ctx context.Context, userId string, categories []string, writeBackType, writeBackDelay string, n, offset int) ([]string, error) {
	var queryCategories string
	if len(categories) > 0 {
		queryCategories = "&category=" + strings.Join(categories, "&category=")
	}
	return request[[]string, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/recommend/%s?write-back-type=%s&write-back-delay=%s&n=%d&offset=%d%s", userId, writeBackType, writeBackDelay, n, offset, queryCategories), nil)
}

func (c *GorseClient) GetItemRecommendWithCategory(ctx context.Context, userId, category, writeBackType, writeBackDelay string, n, offset int) ([]string, error) {
	return request[[]string, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/recommend/%s/%s?write-back-type=%s&write-back-delay=%s&n=%d&offset=%d", userId, category, writeBackType, writeBackDelay, n, offset), nil)
}

// Deprecated: GetItemRecommendWithCategory instead
func (c *GorseClient) GetRecommend(ctx context.Context, userId, category string, n int) ([]string, error) {
	return request[[]string, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/recommend/%s/%s?n=%d", userId, category, n), nil)
}

func (c *GorseClient) SessionItemRecommend(ctx context.Context, feedbacks []Feedback, n, offset int) ([]Score, error) {
	return request[[]Score](ctx, c, "POST", c.entryPoint+fmt.Sprintf("/api/session/recommend?n=%d&offset=%d", n, offset), feedbacks)
}

func (c *GorseClient) SessionItemRecommendWithCategory(ctx context.Context, feedbacks []Feedback, category string, n, offset int) ([]Score, error) {
	return request[[]Score](ctx, c, "POST", c.entryPoint+fmt.Sprintf("/api/session/recommend/%s?n=%d&offset=%d", category, n, offset), feedbacks)
}

// Deprecated: SessionItemRecommend instead
func (c *GorseClient) SessionRecommend(ctx context.Context, feedbacks []Feedback, n int) ([]Score, error) {
	return request[[]Score](ctx, c, "POST", c.entryPoint+fmt.Sprintf("/api/session/recommend?n=%d", n), feedbacks)
}

func (c *GorseClient) GetUserNeighbors(ctx context.Context, userId string, n, offset int) ([]Score, error) {
	return request[[]Score, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/user/%s/neighbors?n=%d&offset=%d", userId, n, offset), nil)
}

func (c *GorseClient) GetItemNeighbors(ctx context.Context, itemId string, n, offset int) ([]Score, error) {
	return request[[]Score, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/item/%s/neighbors?n=%d&offset=%d", itemId, n, offset), nil)
}

func (c *GorseClient) GetItemNeighborsWithCategory(ctx context.Context, itemId, category string, n, offset int) ([]Score, error) {
	return request[[]Score, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/item/%s/neighbors/%s?n=%d&offset=%d", itemId, category, n, offset), nil)
}

// Deprecated: GetItemNeighbors instead
func (c *GorseClient) GetNeighbors(ctx context.Context, itemId string, n int) ([]Score, error) {
	return request[[]Score, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/item/%s/neighbors?n=%d", itemId, n), nil)
}

func (c *GorseClient) InsertUser(ctx context.Context, user User) (RowAffected, error) {
	return request[RowAffected](ctx, c, "POST", c.entryPoint+"/api/user", user)
}

func (c *GorseClient) InsertUsers(ctx context.Context, users []User) (RowAffected, error) {
	return request[RowAffected, any](ctx, c, "POST", c.entryPoint+"/api/users", users)
}

func (c *GorseClient) UpdateUser(ctx context.Context, userId string, user UserPatch) (RowAffected, error) {
	return request[RowAffected](ctx, c, "PATCH", c.entryPoint+fmt.Sprintf("/api/user/%s", userId), user)
}

func (c *GorseClient) GetUser(ctx context.Context, userId string) (User, error) {
	return request[User, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/user/%s", userId), nil)
}

func (c *GorseClient) GetUsers(ctx context.Context, cursor string, n int) (Users, error) {
	return request[Users, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/users?cursor=%s&n=%d", cursor, n), nil)
}

func (c *GorseClient) DeleteUser(ctx context.Context, userId string) (RowAffected, error) {
	return request[RowAffected, any](ctx, c, "DELETE", c.entryPoint+fmt.Sprintf("/api/user/%s", userId), nil)
}

func (c *GorseClient) InsertItem(ctx context.Context, item Item) (RowAffected, error) {
	return request[RowAffected](ctx, c, "POST", c.entryPoint+"/api/item", item)
}

func (c *GorseClient) InsertItems(ctx context.Context, items []Item) (RowAffected, error) {
	return request[RowAffected](ctx, c, "POST", c.entryPoint+"/api/items", items)
}

func (c *GorseClient) UpdateItem(ctx context.Context, itemId string, item ItemPatch) (RowAffected, error) {
	return request[RowAffected](ctx, c, "PATCH", c.entryPoint+fmt.Sprintf("/api/item/%s", itemId), item)
}

func (c *GorseClient) GetItem(ctx context.Context, itemId string) (Item, error) {
	return request[Item, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/item/%s", itemId), nil)
}

func (c *GorseClient) GetItems(ctx context.Context, cursor string, n int) (Items, error) {
	return request[Items, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/items?cursor=%s&n=%d", cursor, n), nil)
}

func (c *GorseClient) DeleteItem(ctx context.Context, itemId string) (RowAffected, error) {
	return request[RowAffected, any](ctx, c, "DELETE", c.entryPoint+fmt.Sprintf("/api/item/%s", itemId), nil)
}

func (c *GorseClient) PutItemCategory(ctx context.Context, itemId string, category string) (RowAffected, error) {
	return request[RowAffected, any](ctx, c, "PUT", c.entryPoint+fmt.Sprintf("/api/item/%s/category/%s", itemId, category), nil)
}

func (c *GorseClient) DelItemCategory(ctx context.Context, itemId string, category string) (RowAffected, error) {
	return request[RowAffected, any](ctx, c, "DELETE", c.entryPoint+fmt.Sprintf("/api/item/%s/category/%s", itemId, category), nil)
}

func (c *GorseClient) HealthLive(ctx context.Context) (Health, error) {
	return request[Health, any](ctx, c, "GET", c.entryPoint+"/api/health/live", nil)
}
func (c *GorseClient) HealthReady(ctx context.Context) (Health, error) {
	return request[Health, any](ctx, c, "GET", c.entryPoint+"/api/health/ready", nil)
}

func request[Response any, Body any](ctx context.Context, c *GorseClient, method, url string, body Body) (result Response, err error) {
	bodyByte, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		return result, marshalErr
	}
	var req *http.Request
	req, err = http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(bodyByte)))
	if err != nil {
		return result, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return result, err
	}
	if resp.StatusCode != http.StatusOK {
		return result, ErrorMessage(buf.String())
	}
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return result, err
	}
	return result, err
}
