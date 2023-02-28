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

func (c *GorseClient) ListFeedbacks(ctx context.Context, feedbackType, userId string) ([]Feedback, error) {
	return request[[]Feedback, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/user/"+userId+"/feedback/"+feedbackType), nil)
}

func (c *GorseClient) GetRecommend(ctx context.Context, userId string, category string, n int) ([]string, error) {
	return request[[]string, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/recommend/%s/%s?n=%d", userId, category, n), nil)
}

func (c *GorseClient) SessionRecommend(ctx context.Context, feedbacks []Feedback, n int) ([]Score, error) {
	return request[[]Score](ctx, c, "POST", c.entryPoint+fmt.Sprintf("/api/session/recommend?n=%d", n), feedbacks)
}

func (c *GorseClient) GetNeighbors(ctx context.Context, itemId string, n int) ([]Score, error) {
	return request[[]Score, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/item/%s/neighbors?n=%d", itemId, n), nil)
}

func (c *GorseClient) InsertUser(ctx context.Context, user User) (RowAffected, error) {
	return request[RowAffected](ctx, c, "POST", c.entryPoint+"/api/user", user)
}

func (c *GorseClient) UpdateUser(ctx context.Context, userId string, user UserPatch) (RowAffected, error) {
	return request[RowAffected](ctx, c, "PATCH", fmt.Sprintf("%s/api/user/%s", c.entryPoint, userId), user)
}

func (c *GorseClient) GetUser(ctx context.Context, userId string) (User, error) {
	return request[User, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/user/%s", userId), nil)
}

func (c *GorseClient) DeleteUser(ctx context.Context, userId string) (RowAffected, error) {
	return request[RowAffected, any](ctx, c, "DELETE", c.entryPoint+fmt.Sprintf("/api/user/%s", userId), nil)
}

func (c *GorseClient) InsertItem(ctx context.Context, item Item) (RowAffected, error) {
	return request[RowAffected](ctx, c, "POST", c.entryPoint+"/api/item", item)
}

func (c *GorseClient) UpdateItem(ctx context.Context, itemId string, item ItemPatch) (RowAffected, error) {
	return request[RowAffected](ctx, c, "PATCH", fmt.Sprintf("%s/api/item/%s", c.entryPoint, itemId), item)
}

func (c *GorseClient) GetItem(ctx context.Context, itemId string) (Item, error) {
	return request[Item, any](ctx, c, "GET", c.entryPoint+fmt.Sprintf("/api/item/%s", itemId), nil)
}

func (c *GorseClient) DeleteItem(ctx context.Context, itemId string) (RowAffected, error) {
	return request[RowAffected, any](ctx, c, "DELETE", c.entryPoint+fmt.Sprintf("/api/item/%s", itemId), nil)
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
