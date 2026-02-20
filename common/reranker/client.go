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

package reranker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/emicklei/go-restful/v3"
)

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewClient(apiKey, endpoint string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    endpoint,
		httpClient: &http.Client{},
	}
}

type RerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	TopN      int      `json:"top_n,omitempty"`
	Documents []string `json:"documents"`
}

type RerankResponse struct {
	Model   string         `json:"model"`
	Usage   Usage          `json:"usage"`
	Results []RerankResult `json:"results"`
}

type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

type Usage struct {
	TotalTokens int `json:"total_tokens"`
}

type dashScopeRequest struct {
	Model      string              `json:"model"`
	Input      dashScopeInput      `json:"input"`
	Parameters dashScopeParameters `json:"parameters,omitempty"`
}

type dashScopeInput struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type dashScopeParameters struct {
	TopN int `json:"top_n,omitempty"`
}

type dashScopeResponse struct {
	Output    dashScopeOutput `json:"output"`
	Usage     Usage           `json:"usage"`
	RequestID string          `json:"request_id"`
	Code      string          `json:"code,omitempty"`
	Message   string          `json:"message,omitempty"`
}

type dashScopeOutput struct {
	Results []RerankResult `json:"results"`
}

func (c *Client) Rerank(ctx context.Context, req RerankRequest) (*RerankResponse, error) {
	var body []byte
	var err error

	isDashScope := strings.Contains(c.baseURL, "dashscope.aliyuncs.com") || strings.Contains(c.baseURL, "services/rerank")

	if isDashScope {
		body, err = json.Marshal(dashScopeRequest{
			Model: req.Model,
			Input: dashScopeInput{
				Query:     req.Query,
				Documents: req.Documents,
			},
			Parameters: dashScopeParameters{
				TopN: req.TopN,
			},
		})
	} else {
		body, err = json.Marshal(req)
	}

	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rerank request failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	if isDashScope {
		var dsResp dashScopeResponse
		if err := json.Unmarshal(respBody, &dsResp); err != nil {
			return nil, err
		}
		if dsResp.Code != "" {
			return nil, fmt.Errorf("dashscope error: %s (code: %s)", dsResp.Message, dsResp.Code)
		}
		return &RerankResponse{
			Usage:   dsResp.Usage,
			Results: dsResp.Output.Results,
		}, nil
	} else {
		var rerankResp RerankResponse
		if err := json.Unmarshal(respBody, &rerankResp); err != nil {
			return nil, err
		}
		return &rerankResp, nil
	}
}

type MockServer struct {
	listener   net.Listener
	httpServer *http.Server
	apiKey     string
	ready      chan struct{}
}

func NewMockServer() *MockServer {
	s := &MockServer{}
	ws := new(restful.WebService)
	ws.Path("/").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.POST("/api/v1/services/rerank/text-rerank/text-rerank").
		Reads(dashScopeRequest{}).
		Writes(dashScopeResponse{}).
		To(s.dashscope))
	ws.Route(ws.POST("/v1/rerank").
		Reads(RerankRequest{}).
		Writes(RerankResponse{}).
		To(s.jina))
	container := restful.NewContainer()
	container.Add(ws)
	s.httpServer = &http.Server{Handler: container}
	s.apiKey = "dashscope"
	s.ready = make(chan struct{})
	return s
}

func (s *MockServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	close(s.ready)
	return s.httpServer.Serve(s.listener)
}

func (s *MockServer) DashScopeURL() string {
	return fmt.Sprintf("http://%s/api/v1/services/rerank/text-rerank/text-rerank", s.listener.Addr().String())
}

func (s *MockServer) JinaURL() string {
	return fmt.Sprintf("http://%s/v1/rerank", s.listener.Addr().String())
}

func (s *MockServer) APIKey() string {
	return s.apiKey
}

func (s *MockServer) Ready() {
	<-s.ready
}

func (s *MockServer) Close() error {
	return s.httpServer.Close()
}

func (s *MockServer) generateResults(documents []string) []RerankResult {
	results := make([]RerankResult, len(documents))
	for i := range documents {
		results[i] = RerankResult{
			Index:          i,
			RelevanceScore: 1.0 / float64(i+1),
		}
	}
	return results
}

func (s *MockServer) dashscope(req *restful.Request, resp *restful.Response) {
	var r dashScopeRequest
	err := req.ReadEntity(&r)
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, err)
		return
	}

	_ = resp.WriteEntity(dashScopeResponse{
		Output: dashScopeOutput{
			Results: s.generateResults(r.Input.Documents),
		},
		Usage: Usage{
			TotalTokens: 100,
		},
		RequestID: "test-request-id",
	})
}

func (s *MockServer) jina(req *restful.Request, resp *restful.Response) {
	var r RerankRequest
	err := req.ReadEntity(&r)
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, err)
		return
	}

	_ = resp.WriteEntity(RerankResponse{
		Model: r.Model,
		Usage: Usage{
			TotalTokens: 100,
		},
		Results: s.generateResults(r.Documents),
	})
}
