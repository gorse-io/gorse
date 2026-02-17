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
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"github.com/stretchr/testify/assert"
)

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

func TestClient_Rerank(t *testing.T) {
	s := NewMockServer()
	go func() {
		err := s.Start()
		assert.ErrorIs(t, err, http.ErrServerClosed)
	}()
	s.Ready()
	defer s.Close()

	client := NewClient(s.APIKey())
	client.SetBaseURL(s.URL())

	req := RerankRequest{
		Model: "gte-rerank",
		Input: Input{
			Query: "What is the capital of France?",
			Documents: []string{
				"Paris is the capital of France.",
				"Lyon is a city in France.",
			},
		},
	}

	resp, err := client.Rerank(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(resp.Output.Results))
	assert.Equal(t, 0, resp.Output.Results[0].Index)
	assert.Equal(t, 1.0, resp.Output.Results[0].RelevanceScore)
	assert.Equal(t, 1, resp.Output.Results[1].Index)
	assert.Equal(t, 0.5, resp.Output.Results[1].RelevanceScore)
}
