// Copyright 2025 gorse Project Authors
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

package mock

import (
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/sashabaranov/go-openai"
	"net"
	"net/http"
)

type OpenAIServer struct {
	listener   net.Listener
	httpServer *http.Server
	authToken  string
	ready      chan struct{}

	mockChatCompletion string
	mockEmbeddings     []float32
}

func NewOpenAIServer() *OpenAIServer {
	s := &OpenAIServer{}
	ws := new(restful.WebService)
	ws.Path("/v1").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML)
	ws.Route(ws.POST("chat/completions").
		Reads(openai.ChatCompletionRequest{}).
		Writes(openai.ChatCompletionResponse{}).
		To(s.chatCompletion))
	ws.Route(ws.POST("embeddings").
		Reads(openai.EmbeddingRequest{}).
		Writes(openai.EmbeddingResponse{}).
		To(s.embeddings))
	container := restful.NewContainer()
	container.Add(ws)
	s.httpServer = &http.Server{Handler: container}
	s.authToken = "ollama"
	s.ready = make(chan struct{})
	return s
}

func (s *OpenAIServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", "")
	if err != nil {
		return err
	}
	close(s.ready)
	return s.httpServer.Serve(s.listener)
}

func (s *OpenAIServer) BaseURL() string {
	return fmt.Sprintf("http://%s/v1", s.listener.Addr().String())
}

func (s *OpenAIServer) AuthToken() string {
	return s.authToken
}

func (s *OpenAIServer) Ready() {
	<-s.ready
}

func (s *OpenAIServer) Close() error {
	return s.httpServer.Close()
}

func (s *OpenAIServer) ChatCompletion(mock string) {
	s.mockChatCompletion = mock
}

func (s *OpenAIServer) Embeddings(embeddings []float32) {
	s.mockEmbeddings = embeddings
}

func (s *OpenAIServer) chatCompletion(req *restful.Request, resp *restful.Response) {
	var r openai.ChatCompletionRequest
	err := req.ReadEntity(&r)
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, err)
		return
	}
	_ = resp.WriteEntity(openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: s.mockChatCompletion,
			},
		}},
	})
}

func (s *OpenAIServer) embeddings(req *restful.Request, resp *restful.Response) {
	var r openai.EmbeddingRequest
	err := req.ReadEntity(&r)
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, err)
		return
	}
	_ = resp.WriteEntity(openai.EmbeddingResponse{
		Data: []openai.Embedding{{
			Embedding: s.mockEmbeddings,
		}},
	})
}
