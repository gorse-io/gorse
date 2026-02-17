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

package dashscope

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/emicklei/go-restful/v3"
)

const (
	RerankEndpoint = "https://dashscope.aliyuncs.com/api/v1/services/rerank/text-rerank/text-rerank"
)

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    RerankEndpoint,
		httpClient: &http.Client{},
	}
}

// SetBaseURL is used for testing to override the default API endpoint with a mock server URL.
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

type RerankRequest struct {
	Model      string      `json:"model"`
	Input      Input       `json:"input"`
	Parameters *Parameters `json:"parameters,omitempty"`
}

type Input struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type Parameters struct {
	TopN            int  `json:"top_n,omitempty"`
	ReturnDocuments bool `json:"return_documents,omitempty"`
}

type RerankResponse struct {
	Output    RerankOutput `json:"output"`
	Usage     Usage        `json:"usage"`
	RequestID string       `json:"request_id"`
	Code      string       `json:"code,omitempty"`
	Message   string       `json:"message,omitempty"`
}

type RerankOutput struct {
	Results []RerankResult `json:"results"`
}

type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

type Usage struct {
	TotalTokens int `json:"total_tokens"`
}

func (c *Client) Rerank(ctx context.Context, req RerankRequest) (*RerankResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(data))
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp RerankResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Message != "" {
			return nil, fmt.Errorf("dashscope error: %s (code: %s)", errorResp.Message, errorResp.Code)
		}
		return nil, fmt.Errorf("dashscope request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	var rerankResp RerankResponse
	if err := json.Unmarshal(body, &rerankResp); err != nil {
		return nil, err
	}

	return &rerankResp, nil
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
	ws.Path("/api/v1/services/rerank/text-rerank/text-rerank").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.POST("").
		Reads(RerankRequest{}).
		Writes(RerankResponse{}).
		To(s.rerank))
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

func (s *MockServer) URL() string {
	return fmt.Sprintf("http://%s/api/v1/services/rerank/text-rerank/text-rerank", s.listener.Addr().String())
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

func (s *MockServer) rerank(req *restful.Request, resp *restful.Response) {
	var r RerankRequest
	err := req.ReadEntity(&r)
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, err)
		return
	}

	results := make([]RerankResult, len(r.Input.Documents))
	for i := range r.Input.Documents {
		results[i] = RerankResult{
			Index:          i,
			RelevanceScore: 1.0 / float64(i+1),
		}
	}

	_ = resp.WriteEntity(RerankResponse{
		Output: RerankOutput{
			Results: results,
		},
		Usage: Usage{
			TotalTokens: 100,
		},
		RequestID: "test-request-id",
	})
}
