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

// Package jev provides a client for the Jev decisions API.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// DefaultEndpoint is the TokenRa Jev decisions endpoint.
	DefaultEndpoint = "https://tokenra.io/v1/decisions"
	// DefaultModel is the rolling Jev model alias.
	DefaultModel = "jev-latest"

	maxResponseBodySize = 16 << 20
)

// Question is a Noul, Choice, or Score question.
type Question interface {
	question()
}

// Questions maps caller-defined question IDs to questions.
type Questions map[string]Question

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

func (NoulQuestion) question() {}

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

// ChoiceQuestion asks the model to choose one key from Criteria.
type ChoiceQuestion struct {
	Instructions any            `json:"instructions"`
	Criteria     map[string]any `json:"criteria"`
}

func (ChoiceQuestion) question() {}

func (q ChoiceQuestion) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type         string         `json:"type"`
		Instructions any            `json:"instructions"`
		Criteria     map[string]any `json:"criteria"`
	}{
		Type:         "choice",
		Instructions: q.Instructions,
		Criteria:     q.Criteria,
	})
}

// ScoreQuestion rates state against an ordered rubric.
type ScoreQuestion struct {
	Instructions any   `json:"instructions"`
	Criteria     []any `json:"criteria"`
}

func (ScoreQuestion) question() {}

func (q ScoreQuestion) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type         string `json:"type"`
		Instructions any    `json:"instructions"`
		Criteria     []any  `json:"criteria"`
	}{
		Type:         "score",
		Instructions: q.Instructions,
		Criteria:     q.Criteria,
	})
}

// Answer contains the fields returned by any Jev answer type.
type Answer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]any     `json:"legend,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
}

func (a *Answer) UnmarshalJSON(data []byte) error {
	var wire struct {
		Type          string             `json:"type"`
		Noul          *float64           `json:"noul"`
		Choice        *string            `json:"choice"`
		Score         *float64           `json:"score"`
		Probabilities map[string]float64 `json:"probabilities"`
		Legend        map[string]any     `json:"legend"`
		Confidence    *float64           `json:"confidence"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	*a = Answer{Type: wire.Type}
	switch wire.Type {
	case "noul":
		if wire.Noul == nil {
			return fmt.Errorf("jev noul answer is missing noul")
		}
		if wire.Choice != nil || wire.Score != nil || wire.Probabilities != nil || wire.Legend != nil || wire.Confidence != nil {
			return fmt.Errorf("jev noul answer contains fields from another answer type")
		}
		a.Noul = wire.Noul
	case "choice":
		if wire.Choice == nil || wire.Probabilities == nil || wire.Confidence == nil {
			return fmt.Errorf("jev choice answer is missing required fields")
		}
		if wire.Noul != nil || wire.Score != nil || wire.Legend != nil {
			return fmt.Errorf("jev choice answer contains fields from another answer type")
		}
		a.Choice = *wire.Choice
		a.Probabilities = wire.Probabilities
		a.Confidence = wire.Confidence
	case "score":
		if wire.Score == nil || wire.Probabilities == nil || wire.Legend == nil || wire.Confidence == nil {
			return fmt.Errorf("jev score answer is missing required fields")
		}
		if wire.Noul != nil || wire.Choice != nil {
			return fmt.Errorf("jev score answer contains fields from another answer type")
		}
		a.Score = wire.Score
		a.Probabilities = wire.Probabilities
		a.Legend = wire.Legend
		a.Confidence = wire.Confidence
	default:
		return fmt.Errorf("unknown jev answer type %q", wire.Type)
	}
	return nil
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
	apiKey     string
	endpoint   string
	model      string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithEndpoint sets the full decisions endpoint URL.
func WithEndpoint(endpoint string) Option {
	return func(client *Client) {
		client.endpoint = endpoint
	}
}

// WithModel sets the model sent with every request.
func WithModel(model string) Option {
	return func(client *Client) {
		client.model = model
	}
}

// WithHTTPClient sets the HTTP client used to send requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

// NewClient creates a Jev client using TokenRa and jev-latest by default.
func NewClient(apiKey string, options ...Option) *Client {
	client := &Client{
		apiKey:     apiKey,
		endpoint:   DefaultEndpoint,
		model:      DefaultModel,
		httpClient: http.DefaultClient,
	}
	for _, option := range options {
		if option != nil {
			option(client)
		}
	}
	return client
}

// Evaluate evaluates all questions against the same state in one request.
func (c *Client) Evaluate(ctx context.Context, state any, questions Questions) (*Response, error) {
	if err := validateQuestions(questions); err != nil {
		return nil, err
	}

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

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBodySize+1))
	if err != nil {
		return nil, err
	}
	if len(responseBody) > maxResponseBodySize {
		return nil, fmt.Errorf("jev response exceeds maximum size")
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

func validateQuestions(questions Questions) error {
	for id, question := range questions {
		switch typed := question.(type) {
		case NoulQuestion:
		case *NoulQuestion:
			if typed == nil {
				return fmt.Errorf("jev question %q is nil", id)
			}
		case ChoiceQuestion:
			if err := validateChoiceQuestion(id, len(typed.Criteria)); err != nil {
				return err
			}
		case *ChoiceQuestion:
			if typed == nil {
				return fmt.Errorf("jev question %q is nil", id)
			}
			if err := validateChoiceQuestion(id, len(typed.Criteria)); err != nil {
				return err
			}
		case ScoreQuestion:
			if err := validateScoreQuestion(id, len(typed.Criteria)); err != nil {
				return err
			}
		case *ScoreQuestion:
			if typed == nil {
				return fmt.Errorf("jev question %q is nil", id)
			}
			if err := validateScoreQuestion(id, len(typed.Criteria)); err != nil {
				return err
			}
		case nil:
			return fmt.Errorf("jev question %q is nil", id)
		default:
			return fmt.Errorf("jev question %q has unsupported type %T", id, question)
		}
	}
	return nil
}

func validateChoiceQuestion(id string, options int) error {
	if options < 1 || options > 255 {
		return fmt.Errorf("jev choice question %q must have between 1 and 255 options", id)
	}
	return nil
}

func validateScoreQuestion(id string, levels int) error {
	if levels < 2 || levels > 10 {
		return fmt.Errorf("jev score question %q must have between 2 and 10 levels", id)
	}
	return nil
}
