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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/fatih/color"
	gorse "github.com/gorse-io/gorse-go"
	"github.com/gorse-io/gorse/common/util"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	maxIterations           = 10
	minEmbeddingDimensions  = 8
	fulltextSearchTool      = "FullTextSearch"
	fulltextSearchDefaultN  = 10
	fulltextSearchItemsMaxN = 20
)

var chatCmd = &cobra.Command{
	Use:   "chat [message...]",
	Short: "Chat with the configured LLM",
	Example: `  # Chat with a prompt argument
  gorse-cli chat "Summarize the current recommendation pipeline"

  # Chat with piped input
  cat prompt.txt | gorse-cli chat`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		client := newChatClient(cmd)
		gorseClient := newGorseClient(cmd)

		message := strings.TrimSpace(strings.Join(args, " "))
		if message == "" {
			if file, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(file.Fd())) {
				message = ""
			}
			content, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to read message from stdin:", err)
				os.Exit(1)
			}
			message = strings.TrimSpace(string(content))
		}
		if message == "" {
			fmt.Fprintln(os.Stderr, "no message provided")
			os.Exit(1)
		}
		messages := []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: `Use a ReAct loop internally: reason about whether a tool is needed, call a tool when useful, observe its result, and repeat until you can answer.`},
			{Role: openai.ChatMessageRoleUser, Content: message},
		}

		for range maxIterations {
			stream, err := client.CreateChatCompletionStream(cmd.Context(), openai.ChatCompletionRequest{
				Messages:          messages,
				Tools:             tools,
				ToolChoice:        "auto",
				ParallelToolCalls: false,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to create chat completion stream: %v\n", err)
				os.Exit(1)
			}
			content, toolCalls := readChatCompletionStream(stream)
			if err := stream.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to close chat completion stream: %v\n", err)
				os.Exit(1)
			}
			if len(toolCalls) == 0 {
				return
			}
			messages = append(messages, openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   content,
				ToolCalls: toolCalls,
			})
			for _, toolCall := range toolCalls {
				messages = append(messages, callTool(cmd.Context(), gorseClient, toolCall))
			}
		}

		fmt.Fprintf(os.Stderr, "tool call limit exceeded after %d iterations", maxIterations)
		os.Exit(1)
	},
}

type transport struct {
	apiKey string
}

func (t transport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.apiKey != "" {
		request = request.Clone(request.Context())
		request.Header.Set("X-Api-Key", t.apiKey)
	}
	return http.DefaultTransport.RoundTrip(request)
}

func newChatClient(cmd *cobra.Command) *openai.Client {
	endpoint, apiKey := requireEndpointAndKey(cmd)
	clientConfig := openai.DefaultConfig("")
	clientConfig.BaseURL = lo.Must(url.JoinPath(endpoint, "/api"))
	clientConfig.HTTPClient = &http.Client{
		Transport: transport{apiKey: apiKey},
	}
	return openai.NewClientWithConfig(clientConfig)
}

func readChatCompletionStream(stream *openai.ChatCompletionStream) (content string, toolCalls []openai.ToolCall) {
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			fmt.Println()
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to receive chat completion stream: %v\n", err)
			os.Exit(1)
		}
		for _, choice := range response.Choices {
			if choice.Delta.Content != "" {
				content += choice.Delta.Content
				fmt.Print(choice.Delta.Content)
			}
			for _, delta := range choice.Delta.ToolCalls {
				index := len(toolCalls)
				if delta.Index != nil {
					index = *delta.Index
				}
				for len(toolCalls) <= index {
					toolCalls = append(toolCalls, openai.ToolCall{})
				}
				if delta.ID != "" {
					toolCalls[index].ID = delta.ID
				}
				if delta.Type != "" {
					toolCalls[index].Type = delta.Type
				}
				toolCalls[index].Function.Name += delta.Function.Name
				toolCalls[index].Function.Arguments += delta.Function.Arguments
			}
		}
	}
}

var tools = []openai.Tool{{
	Type: openai.ToolTypeFunction,
	Function: &openai.FunctionDefinition{
		Name:        fulltextSearchTool,
		Description: "Search items in the Gorse item catalog by full-text query.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Full-text search query for items.",
				},
				"n": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("Maximum number of items to return. Defaults to %d and is capped at %d.", fulltextSearchDefaultN, fulltextSearchItemsMaxN),
					"minimum":     1,
					"maximum":     fulltextSearchItemsMaxN,
				},
			},
			"required":             []string{"query"},
			"additionalProperties": false,
		},
	},
}}

func callTool(ctx context.Context, gorseClient *gorse.GorseClient, toolCall openai.ToolCall) openai.ChatCompletionMessage {
	color.HiBlack(toolCall.Function.Name + " " + toolCall.Function.Arguments)
	switch toolCall.Function.Name {
	case fulltextSearchTool:
		return callFulltextSearch(ctx, gorseClient, toolCall)
	default:
		return newToolMessage(toolCall, fmt.Sprintf("unknown tool: %s", toolCall.Function.Name))
	}
}

func callFulltextSearch(ctx context.Context, gorseClient *gorse.GorseClient, toolCall openai.ToolCall) openai.ChatCompletionMessage {
	var args struct {
		Query string `json:"query"`
		N     int    `json:"n,omitempty"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return newToolMessage(toolCall, fmt.Sprintf("invalid SearchItems arguments: %s", err.Error()))
	}
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return newToolMessage(toolCall, "SearchItems query is required. Please provide a valid query.")
	}
	if args.N <= 0 {
		args.N = fulltextSearchDefaultN
	}
	if args.N > fulltextSearchItemsMaxN {
		args.N = fulltextSearchItemsMaxN
	}

	items, err := gorseClient.SearchItems(ctx, args.Query, args.N)
	if err != nil {
		return newToolMessage(toolCall, err.Error())
	}
	for i := range items.Items {
		items.Items[i].Labels = util.RemoveEmbeddings(items.Items[i].Labels, minEmbeddingDimensions)
	}
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d items for query '%s':\n", len(items.Items), args.Query))
	for i, item := range items.Items {
		b, err := json.Marshal(item)
		if err != nil {
			return newToolMessage(toolCall, fmt.Sprintf("failed to encode item %d: %s", i, err.Error()))
		}
		result.Write(b)
		result.WriteByte('\n')
	}
	return newToolMessage(toolCall, result.String())
}

func newToolMessage(toolCall openai.ToolCall, content string) openai.ChatCompletionMessage {
	color.HiBlack(content)
	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    content,
		Name:       toolCall.Function.Name,
		ToolCallID: toolCall.ID,
	}
}
