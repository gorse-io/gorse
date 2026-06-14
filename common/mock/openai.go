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
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/emicklei/go-restful/v3"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
)

type OpenAIServer struct {
	listener   net.Listener
	httpServer *http.Server
	authToken  string
	ready      chan struct{}
}

func NewOpenAIServer() *OpenAIServer {
	s := &OpenAIServer{}
	ws := new(restful.WebService)
	ws.Path("/v1").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON, "text/event-stream")
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

func (s *OpenAIServer) chatCompletion(req *restful.Request, resp *restful.Response) {
	var r openai.ChatCompletionRequest
	err := req.ReadEntity(&r)
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, err)
		return
	}
	message := lastChatMessage(r.Messages)
	if !r.Stream && hasChatTool(r.Tools, "SearchItems") && message.Role == openai.ChatMessageRoleUser {
		if query, ok := searchItemsQuery(message.Content); ok {
			_ = resp.WriteEntity(searchItemsToolCallResponse(query))
			return
		}
	}
	if r.Stream && hasChatTool(r.Tools, "SearchItems") && message.Role == openai.ChatMessageRoleUser {
		if query, ok := searchItemsQuery(message.Content); ok {
			writeChatCompletionStreamResponse(resp, searchItemsToolCallStreamResponse(query))
			return
		}
	}

	content := message.Content
	if message.Role == openai.ChatMessageRoleTool {
		content = "SearchItems returned: " + message.Content
	}
	if r.Model == "deepseek-r1" {
		content = "<think>To be or not to be, that is the question.</think>" + content
	}
	if r.Stream {
		for i := 0; i < len(content); i += 8 {
			writeChatCompletionStreamResponse(resp, openai.ChatCompletionStreamResponse{
				Choices: []openai.ChatCompletionStreamChoice{{
					Delta: openai.ChatCompletionStreamChoiceDelta{
						Content: content[i:min(i+8, len(content))],
					},
				}},
			})
		}
	} else {
		_ = resp.WriteEntity(openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{{
				Message: openai.ChatCompletionMessage{
					Content: content,
				},
			}},
		})
	}
}

func searchItemsToolCallResponse(query string) openai.ChatCompletionResponse {
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				ToolCalls: []openai.ToolCall{newSearchItemsToolCall(nil, query)},
			},
			FinishReason: openai.FinishReasonToolCalls,
		}},
	}
}

func searchItemsToolCallStreamResponse(query string) openai.ChatCompletionStreamResponse {
	index := 0
	return openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{{
			Delta: openai.ChatCompletionStreamChoiceDelta{
				ToolCalls: []openai.ToolCall{newSearchItemsToolCall(&index, query)},
			},
			FinishReason: openai.FinishReasonToolCalls,
		}},
	}
}

func newSearchItemsToolCall(index *int, query string) openai.ToolCall {
	arguments, _ := json.Marshal(map[string]any{
		"query": query,
		"n":     10,
	})
	return openai.ToolCall{
		Index: index,
		ID:    "call_search_items",
		Type:  openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "SearchItems",
			Arguments: string(arguments),
		},
	}
}

func writeChatCompletionStreamResponse(resp *restful.Response, response openai.ChatCompletionStreamResponse) {
	buf := bytes.NewBuffer(nil)
	buf.WriteString("data: ")
	encoder := json.NewEncoder(buf)
	_ = encoder.Encode(response)
	buf.WriteString("\n")
	_, _ = resp.Write(buf.Bytes())
}

func lastChatMessage(messages []openai.ChatCompletionMessage) openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return openai.ChatCompletionMessage{}
	}
	return messages[len(messages)-1]
}

func hasChatTool(tools []openai.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Function != nil && tool.Function.Name == name {
			return true
		}
	}
	return false
}

func searchItemsQuery(content string) (string, bool) {
	lowerContent := strings.ToLower(content)
	for _, prefix := range []string{
		"search items for ",
		"search items ",
		"find items for ",
		"find items ",
	} {
		if index := strings.Index(lowerContent, prefix); index >= 0 {
			query := strings.TrimSpace(content[index+len(prefix):])
			return strings.Trim(query, ` "'`), query != ""
		}
	}
	return "", false
}

func (s *OpenAIServer) embeddings(req *restful.Request, resp *restful.Response) {
	// parse request
	var r openai.EmbeddingRequest
	err := req.ReadEntity(&r)
	if err != nil {
		_ = resp.WriteError(http.StatusBadRequest, err)
		return
	}
	input, ok := r.Input.(string)
	if !ok {
		_ = resp.WriteError(http.StatusBadRequest, fmt.Errorf("invalid input type"))
		return
	}

	// write response
	_ = resp.WriteEntity(openai.EmbeddingResponse{
		Data: []openai.Embedding{{
			Embedding: Hash(input),
		}},
	})
}

func Hash(input string) []float32 {
	hasher := md5.New()
	_, err := hasher.Write([]byte(input))
	if err != nil {
		panic(err)
	}
	h := hasher.Sum(nil)
	return lo.Map(h, func(b byte, _ int) float32 {
		return float32(b)
	})
}
