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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type GorseClient struct {
	entryPoint string
	apiKey     string
	httpClient http.Client
}

func NewGorseClient(EntryPoint, ApiKey string) *GorseClient {
	return &GorseClient{
		entryPoint: EntryPoint,
		apiKey:     ApiKey,
	}
}

func (c *GorseClient) InsertFeedback(feedbacks []Feedback) (RowAffected, error) {
	return request[RowAffected](c, "POST", c.entryPoint+"/api/feedback", feedbacks)
}

func (c *GorseClient) ListFeedbacks(feedbackType, userId string) ([]Feedback, error) {
	return request[[]Feedback](c, "GET", c.entryPoint+fmt.Sprintf("/api/user/"+userId+"/feedback/"+feedbackType), nil)
}

func (c *GorseClient) GetRecommend(userId string, category string, n int) ([]string, error) {
	return request[[]string](c, "GET", c.entryPoint+fmt.Sprintf("/api/recommend/%s/%s?n=%d", userId, category, n), nil)
}

func (c *GorseClient) SessionRecommend(feedbacks []Feedback, n int) ([]Score, error) {
	return request[[]Score](c, "POST", c.entryPoint+fmt.Sprintf("/api/session/recommend?n=%d", n), feedbacks)
}

func (c *GorseClient) GetNeighbors(itemId string, n int) ([]Score, error) {
	return request[[]Score](c, "GET", c.entryPoint+fmt.Sprintf("/api/item/%s/neighbors?n=%d", itemId, n), nil)
}

func (c *GorseClient) InsertUser(user User) (RowAffected, error) {
	return request[RowAffected](c, "POST", c.entryPoint+"/api/user", user)
}

func (c *GorseClient) GetUser(userId string) (User, error) {
	return request[User](c, "GET", c.entryPoint+fmt.Sprintf("/api/user/%s", userId), nil)
}

func (c *GorseClient) DeleteUser(userId string) (RowAffected, error) {
	return request[RowAffected](c, "DELETE", c.entryPoint+fmt.Sprintf("/api/user/%s", userId), nil)
}

func (c *GorseClient) InsertItem(item Item) (RowAffected, error) {
	return request[RowAffected](c, "POST", c.entryPoint+"/api/item", item)
}

func (c *GorseClient) GetItem(itemId string) (Item, error) {
	return request[Item](c, "GET", c.entryPoint+fmt.Sprintf("/api/item/%s", itemId), nil)
}

func (c *GorseClient) DeleteItem(itemId string) (RowAffected, error) {
	return request[RowAffected](c, "DELETE", c.entryPoint+fmt.Sprintf("/api/item/%s", itemId), nil)
}

func request[Response any](c *GorseClient, method, url string, body interface{}) (result Response, err error) {
	req := &http.Request{}
	if body == nil {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return result, err
		}
	} else {
		bodyByte, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return result, marshalErr
		}
		req, err = http.NewRequest(method, url, strings.NewReader(string(bodyByte)))
		if err != nil {
			return result, err
		}
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
