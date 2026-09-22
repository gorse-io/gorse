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

package jev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientEvaluate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var request struct {
			Model     string                     `json:"model"`
			State     map[string]any             `json:"state"`
			Questions map[string]json.RawMessage `json:"questions"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, DefaultModel, request.Model)
		require.Equal(t, "Help! My payouts have been failing for 3 days.", request.State["message"])
		require.JSONEq(t, `{
			"type":"noul",
			"instructions":"Does this message convey urgency?",
			"criteria":{"true":"Explicitly time-sensitive","false":"No urgency expressed"}
		}`, string(request.Questions["is_urgent"]))
		require.JSONEq(t, `{
			"type":"choice",
			"instructions":"Which team should handle this?",
			"criteria":{"billing":"Payments","technical":"Bugs"}
		}`, string(request.Questions["department"]))
		require.JSONEq(t, `{
			"type":"score",
			"instructions":"How frustrated is the customer?",
			"criteria":["Calm","Frustrated","Very angry"]
		}`, string(request.Questions["frustration"]))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"model":"typesafe/jev-1.13-20260917",
			"answers":{
				"department":{"type":"choice","choice":"billing","probabilities":{"billing":0.89,"technical":0.11},"confidence":0.83},
				"frustration":{"type":"score","score":1.05,"legend":{"0":"Calm","1":"Frustrated","2":"Very angry"},"probabilities":{"0":0,"1":0.95,"2":0.05},"confidence":0.92},
				"is_urgent":{"type":"noul","noul":0.96}
			},
			"usage":{"input_tokens":427,"output_tokens":73,"cost":0.000017934},
			"id":"gen-dec-123",
			"provider":"TypeSafe"
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient("test-key", WithEndpoint(server.URL), WithHTTPClient(server.Client()))
	response, err := client.Evaluate(context.Background(), map[string]any{
		"message": "Help! My payouts have been failing for 3 days.",
	}, Questions{
		"is_urgent": NoulQuestion{
			Instructions: "Does this message convey urgency?",
			Criteria: &NoulCriteria{
				True:  "Explicitly time-sensitive",
				False: "No urgency expressed",
			},
		},
		"department": ChoiceQuestion{
			Instructions: "Which team should handle this?",
			Criteria: map[string]any{
				"billing":   "Payments",
				"technical": "Bugs",
			},
		},
		"frustration": ScoreQuestion{
			Instructions: "How frustrated is the customer?",
			Criteria:     []any{"Calm", "Frustrated", "Very angry"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "typesafe/jev-1.13-20260917", response.Model)
	require.Equal(t, "billing", response.Answers["department"].Choice)
	require.Equal(t, 0.89, response.Answers["department"].Probabilities["billing"])
	require.Equal(t, 0.83, *response.Answers["department"].Confidence)
	require.Equal(t, 1.05, *response.Answers["frustration"].Score)
	require.Equal(t, "Very angry", response.Answers["frustration"].Legend["2"])
	require.Equal(t, 0.96, *response.Answers["is_urgent"].Noul)
	require.Equal(t, 427, response.Usage.InputTokens)
	require.Equal(t, 0.000017934, response.Usage.Cost)
	require.Equal(t, "gen-dec-123", response.ID)
	require.Equal(t, "TypeSafe", response.Provider)
}

func TestClientEvaluateWithModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model string `json:"model"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "jev-1.13.0", request.Model)
		_, err := w.Write([]byte(`{"model":"jev-1.13.0","answers":{},"usage":{}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient("test-key", WithEndpoint(server.URL), WithModel("jev-1.13.0"))
	_, err := client.Evaluate(context.Background(), "state", Questions{})
	require.NoError(t, err)
}

func TestClientEvaluateStructuredScoreLegend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{
			"model":"jev-latest",
			"answers":{
				"severity":{
					"type":"score",
					"score":0.25,
					"legend":{"0":{"label":"low"},"1":["high","critical"]},
					"probabilities":{"0":0.75,"1":0.25},
					"confidence":0.5
				}
			},
			"usage":{}
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient("test-key", WithEndpoint(server.URL))
	response, err := client.Evaluate(context.Background(), "state", Questions{
		"severity": ScoreQuestion{
			Instructions: "How severe is this?",
			Criteria: []any{
				map[string]any{"label": "low"},
				[]any{"high", "critical"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"label": "low"}, response.Answers["severity"].Legend["0"])
	require.Equal(t, []any{"high", "critical"}, response.Answers["severity"].Legend["1"])
}

func TestClientEvaluateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, err := w.Write([]byte(`{"error":"invalid question"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient("test-key", WithEndpoint(server.URL))
	_, err := client.Evaluate(context.Background(), "state", Questions{})

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusUnprocessableEntity, apiErr.StatusCode)
	require.JSONEq(t, `{"error":"invalid question"}`, string(apiErr.Body))
	require.EqualError(t, err, `jev request failed with status 422: {"error":"invalid question"}`)
}

func TestClientEvaluateInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`not json`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient("test-key", WithEndpoint(server.URL))
	_, err := client.Evaluate(context.Background(), "state", Questions{})
	require.Error(t, err)

	var syntaxErr *json.SyntaxError
	require.True(t, errors.As(err, &syntaxErr))
}

func TestClientEvaluateInvalidAnswer(t *testing.T) {
	tests := []struct {
		name   string
		answer string
	}{
		{name: "unknown type", answer: `{"type":"unknown"}`},
		{name: "noul missing value", answer: `{"type":"noul"}`},
		{name: "noul with choice field", answer: `{"type":"noul","noul":0.5,"choice":"yes"}`},
		{name: "choice missing confidence", answer: `{"type":"choice","choice":"yes","probabilities":{"yes":1}}`},
		{name: "choice with score field", answer: `{"type":"choice","choice":"yes","probabilities":{"yes":1},"confidence":1,"score":1}`},
		{name: "score missing legend", answer: `{"type":"score","score":1,"probabilities":{"1":1},"confidence":1}`},
		{name: "score with noul field", answer: `{"type":"score","score":1,"legend":{"1":"high"},"probabilities":{"1":1},"confidence":1,"noul":1}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, err := fmt.Fprintf(w, `{"model":"jev-latest","answers":{"result":%s},"usage":{}}`, test.answer)
				require.NoError(t, err)
			}))
			defer server.Close()

			client := NewClient("test-key", WithEndpoint(server.URL))
			_, err := client.Evaluate(context.Background(), "state", Questions{})
			require.Error(t, err)
		})
	}
}

func TestClientEvaluateValidatesQuestions(t *testing.T) {
	tests := []struct {
		name      string
		questions Questions
	}{
		{
			name:      "nil question",
			questions: Questions{"question": nil},
		},
		{
			name:      "nil question pointer",
			questions: Questions{"question": (*NoulQuestion)(nil)},
		},
		{
			name: "choice has no options",
			questions: Questions{
				"question": ChoiceQuestion{Criteria: map[string]any{}},
			},
		},
		{
			name: "choice too many options",
			questions: Questions{
				"question": ChoiceQuestion{Criteria: makeCriteria(256)},
			},
		},
		{
			name: "score too few levels",
			questions: Questions{
				"question": ScoreQuestion{Criteria: []any{"one"}},
			},
		},
		{
			name: "score too many levels",
			questions: Questions{
				"question": ScoreQuestion{Criteria: make([]any, 11)},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++
				_, err := w.Write([]byte(`{"model":"jev-latest","answers":{},"usage":{}}`))
				require.NoError(t, err)
			}))
			defer server.Close()

			client := NewClient("test-key", WithEndpoint(server.URL))
			_, err := client.Evaluate(context.Background(), "state", test.questions)
			require.Error(t, err)
			require.Zero(t, requests)
		})
	}
}

func TestClientEvaluateRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(strings.Repeat("x", maxResponseBodySize+1)))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient("test-key", WithEndpoint(server.URL))
	_, err := client.Evaluate(context.Background(), "state", Questions{})
	require.EqualError(t, err, "jev response exceeds maximum size")
}

func TestClientWithNilHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"model":"jev-latest","answers":{},"usage":{}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient("test-key", nil, WithEndpoint(server.URL), WithHTTPClient(nil))
	_, err := client.Evaluate(context.Background(), "state", Questions{})
	require.NoError(t, err)
}

func makeCriteria(count int) map[string]any {
	criteria := make(map[string]any, count)
	for i := range count {
		criteria[fmt.Sprintf("option-%d", i)] = nil
	}
	return criteria
}
