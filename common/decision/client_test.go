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

package decision

import (
	"context"
	"encoding/json"
	"errors"
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
		require.Equal(t, "test-model", request.Model)
		require.Equal(t, "Help! My payouts have been failing for 3 days.", request.State["message"])
		require.JSONEq(t, `{
			"type":"noul",
			"instructions":"Does this message convey urgency?",
			"criteria":{"true":"Explicitly time-sensitive","false":"No urgency expressed"}
		}`, string(request.Questions["is_urgent"]))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"model":"typesafe/jev-1.13-20260917",
			"answers":{
				"is_urgent":{"type":"noul","noul":0.96}
			},
			"usage":{"input_tokens":427,"output_tokens":73,"cost":0.000017934},
			"id":"gen-dec-123",
			"provider":"TypeSafe"
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient("test-key", WithEndpoint(server.URL), WithModel("test-model"), WithHTTPClient(server.Client()))
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
	})
	require.NoError(t, err)
	require.Equal(t, "typesafe/jev-1.13-20260917", response.Model)
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
