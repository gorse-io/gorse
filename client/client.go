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

func (c *GorseClient) InsertFeedback(feedbacks []Feedback) (*RowAffected, error) {
	body, err := json.Marshal(feedbacks)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST",
		c.entryPoint+"/api/feedback",
		strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	var result RowAffected
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *GorseClient) ListFeedbacks(feedbackType, userId string) (interface{}, error) {
	req, err := http.NewRequest("GET",
		c.entryPoint+"/api/user/"+userId+"/feedback/"+feedbackType, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	result := make([]Feedback, 0)
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GorseClient) GetRecommend(userId string, category string, n int) ([]string, error) {

	req, err := http.NewRequest("GET",
		c.entryPoint+fmt.Sprintf("/api/recommend/%s/%s?n=%d", userId, category, n), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	result := make([]string, 0)
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GorseClient) SessionRecommend(feedbacks []Feedback, n int) ([]Score, error) {

	body, err := json.Marshal(feedbacks)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST",
		c.entryPoint+fmt.Sprintf("/api/session/recommend?n=%d", n),
		strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	result := make([]Score, 0)
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GorseClient) GetNeighbors(itemId string, n int) ([]Score, error) {

	req, err := http.NewRequest("GET",
		c.entryPoint+fmt.Sprintf("/api/item/%s/neighbors?n=%d", itemId, n),
		nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	result := make([]Score, 0)
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GorseClient) InsertUser(user User) (*RowAffected, error) {
	body, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST",
		c.entryPoint+"/api/user",
		strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	var result RowAffected
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *GorseClient) GetUser(userId string) (*User, error) {
	req, err := http.NewRequest("GET",
		c.entryPoint+fmt.Sprintf("/api/user/%s", userId), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	var result User
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil

}

func (c *GorseClient) DeleteUser(userId string) (*RowAffected, error) {
	req, err := http.NewRequest("DELETE",
		c.entryPoint+fmt.Sprintf("/api/user/%s", userId), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	var result RowAffected
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *GorseClient) InsertItem(item Item) (*RowAffected, error) {
	body, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST",
		c.entryPoint+"/api/item",
		strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	var result RowAffected
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *GorseClient) GetItem(itemId string) (*Item, error) {
	req, err := http.NewRequest("GET",
		c.entryPoint+fmt.Sprintf("/api/item/%s", itemId), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	var result Item
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil

}

func (c *GorseClient) DeleteItem(itemId string) (*RowAffected, error) {
	req, err := http.NewRequest("DELETE",
		c.entryPoint+fmt.Sprintf("/api/item/%s", itemId), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrorMessage(buf.String())
	}
	var result RowAffected
	err = json.Unmarshal([]byte(buf.String()), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
