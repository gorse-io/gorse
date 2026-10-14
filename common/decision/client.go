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

// Package decision provides a client for the Jev decisions API.
package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Questions maps caller-defined question IDs to Noul questions.
type Questions map[string]NoulQuestion

// NoulCriteria describes the yes and no outcomes of a Noul question.
type NoulCriteria struct {
	True  any `json:"true,omitempty"`
	False any `json:"false,omitempty"`
}

// NoulQuestion asks a yes/no question and returns the probability of yes.
type NoulQuestion struct {
	Instructions any           `json:"instructions"`
	Criteria     *NoulCriteria `json:"criteria,omitempty"`
}

func (q NoulQuestion) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type         string        `json:"type"`
		Instructions any           `json:"instructions"`
		Criteria     *NoulCriteria `json:"criteria,omitempty"`
	}{
		Type:         "noul",
		Instructions: q.Instructions,
		Criteria:     q.Criteria,
	})
}

// Answer contains the probability returned by a Noul question.
type Answer struct {
	Type string   `json:"type"`
	Noul *float64 `json:"noul"`
}

// Usage contains token usage and, when supplied by the provider, cost.
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Cost         float64 `json:"cost,omitempty"`
}

// Response is a Jev decisions response.
type Response struct {
	Model    string            `json:"model"`
	Answers  map[string]Answer `json:"answers"`
	Usage    Usage             `json:"usage"`
	ID       string            `json:"id,omitempty"`
	Provider string            `json:"provider,omitempty"`
}

// APIError reports a non-successful HTTP response from the Jev API.
type APIError struct {
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("jev request failed with status %d: %s", e.StatusCode, strings.TrimSpace(string(e.Body)))
}

// Client calls a Jev decisions endpoint.
type Client struct {
	apiKey   string
	endpoint string
	model    string
}

// NewClient creates a Jev client with an explicit endpoint and model.
func NewClient(apiKey, endpoint, model string) *Client {
	return &Client{
		apiKey:   apiKey,
		endpoint: endpoint,
		model:    model,
	}
}

// Evaluate evaluates all questions against the same state in one request.
func (c *Client) Evaluate(ctx context.Context, state any, questions Questions) (*Response, error) {
	body, err := json.Marshal(struct {
		Model     string    `json:"model"`
		State     any       `json:"state"`
		Questions Questions `json:"questions"`
	}{
		Model:     c.model,
		State:     state,
		Questions: questions,
	})
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &APIError{StatusCode: response.StatusCode, Body: responseBody}
	}

	var result Response
	if err = json.Unmarshal(responseBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
