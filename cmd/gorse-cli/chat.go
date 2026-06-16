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
	"os"
	"strings"

	gorse "github.com/gorse-io/gorse-go"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	chatSearchItemsToolName = "SearchItems"
	chatSearchItemsDefaultN = 10
	chatSearchItemsMaxN     = 20
	chatMaxToolIterations   = 6
)

const chatReActSystemPrompt = `You are a Gorse recommendation assistant.
Use a ReAct loop internally: reason about whether a tool is needed, call a tool when useful, observe its result, and repeat until you can answer.
Use SearchItems when the user asks about items that may exist in the Gorse item catalog.
Do not expose hidden reasoning or raw tool-call JSON unless the user explicitly asks for raw data.`

var chatCmd = &cobra.Command{
	Use:   "chat [message...]",
	Short: "Chat with the configured LLM",
	Example: `  # Chat with a prompt argument
  gorse-cli chat "Summarize the current recommendation pipeline"

  # Chat with piped input
  cat prompt.txt | gorse-cli chat`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		message := strings.TrimSpace(strings.Join(args, " "))
		if message == "" {
			if file, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(file.Fd())) {
				message = ""
			}
			content, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				fatal(cmd, err.Error())
			}
			message = strings.TrimSpace(string(content))
		}
		if message == "" {
			fatal(cmd,
				"missing chat message.",
				"Pass a prompt:",
				"  gorse-cli chat \"How should I tune recommendations?\"",
				"Or pipe stdin:",
				"  echo \"How should I tune recommendations?\" | gorse-cli chat",
			)
		}
		if err := runChatReAct(cmd, newChatClient(cmd), newGorseClient(cmd), message, cmd.OutOrStdout()); err != nil {
			fatal(cmd, err.Error())
		}
	},
}

type adminTransport struct {
	apiKey string
	base   http.RoundTripper
}

func (t adminTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.apiKey != "" {
		request = request.Clone(request.Context())
		request.Header.Set("X-Api-Key", t.apiKey)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(request)
}

func newChatClient(cmd *cobra.Command) *openai.Client {
	endpoint, apiKey := requireEndpointAndKey(cmd)
	clientConfig := openai.DefaultConfig("")
	clientConfig.BaseURL = strings.TrimRight(endpoint, "/") + "/api"
	clientConfig.HTTPClient = &http.Client{
		Transport: adminTransport{apiKey: apiKey},
	}
	return openai.NewClientWithConfig(clientConfig)
}

func runChatReAct(cmd *cobra.Command, client *openai.Client, gorseClient *gorse.GorseClient, message string, output io.Writer) error {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: chatReActSystemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: message},
	}
	tools := chatTools()

	for range chatMaxToolIterations {
		stream, err := client.CreateChatCompletionStream(cmd.Context(), openai.ChatCompletionRequest{
			Messages:          messages,
			Tools:             tools,
			ToolChoice:        "auto",
			ParallelToolCalls: false,
		})
		if err != nil {
			return fmt.Errorf("failed to create chat completion stream: %w", err)
		}
		streamedCompletion, err := readChatCompletionStream(stream, output)
		closeErr := stream.Close()
		if err != nil {
			return fmt.Errorf("failed to read chat completion stream: %w", err)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close chat completion stream: %w", closeErr)
		}

		if len(streamedCompletion.ToolCalls) == 0 {
			if strings.TrimSpace(streamedCompletion.Content) == "" {
				return errors.New("chat completion returned an empty message")
			}
			_, err = fmt.Fprintln(output)
			return err
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   streamedCompletion.Content,
			ToolCalls: streamedCompletion.ToolCalls,
		})
		for _, toolCall := range streamedCompletion.ToolCalls {
			messages = append(messages, executeChatTool(cmd.Context(), gorseClient, toolCall))
		}
	}

	return fmt.Errorf("tool call limit exceeded after %d iterations", chatMaxToolIterations)
}

type streamedChatCompletion struct {
	Content   string
	ToolCalls []openai.ToolCall
}

func readChatCompletionStream(stream *openai.ChatCompletionStream, output io.Writer) (streamedChatCompletion, error) {
	var result streamedChatCompletion
	var content strings.Builder
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			result.Content = content.String()
			return result, nil
		}
		if err != nil {
			return result, err
		}
		for _, choice := range response.Choices {
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				if output != nil {
					if _, err := io.WriteString(output, choice.Delta.Content); err != nil {
						return result, err
					}
					flushChatOutput(output)
				}
			}
			for _, toolCall := range choice.Delta.ToolCalls {
				result.appendToolCall(toolCall)
			}
		}
	}
}

func flushChatOutput(output io.Writer) {
	switch flusher := output.(type) {
	case interface{ Flush() error }:
		_ = flusher.Flush()
	case interface{ Flush() }:
		flusher.Flush()
	}
}

func (r *streamedChatCompletion) appendToolCall(delta openai.ToolCall) {
	index := len(r.ToolCalls)
	if delta.Index != nil {
		index = *delta.Index
	}
	for len(r.ToolCalls) <= index {
		r.ToolCalls = append(r.ToolCalls, openai.ToolCall{})
	}

	toolCall := &r.ToolCalls[index]
	if delta.ID != "" {
		toolCall.ID = delta.ID
	}
	if delta.Type != "" {
		toolCall.Type = delta.Type
	}
	toolCall.Function.Name += delta.Function.Name
	toolCall.Function.Arguments += delta.Function.Arguments
}

func chatTools() []openai.Tool {
	return []openai.Tool{{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        chatSearchItemsToolName,
			Description: "Search items in the Gorse item catalog by full-text query.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Full-text search query for item IDs, categories, labels, or comments configured in [recommend.search].columns.",
					},
					"n": map[string]any{
						"type":        "integer",
						"description": fmt.Sprintf("Maximum number of items to return. Defaults to %d and is capped at %d.", chatSearchItemsDefaultN, chatSearchItemsMaxN),
						"minimum":     1,
						"maximum":     chatSearchItemsMaxN,
					},
				},
				"required":             []string{"query"},
				"additionalProperties": false,
			},
		},
	}}
}

type chatSearchItemsArguments struct {
	Query string `json:"query"`
	N     int    `json:"n,omitempty"`
}

func executeChatTool(ctx context.Context, gorseClient *gorse.GorseClient, toolCall openai.ToolCall) openai.ChatCompletionMessage {
	switch toolCall.Function.Name {
	case chatSearchItemsToolName:
		return executeChatSearchItems(ctx, gorseClient, toolCall)
	default:
		return chatToolObservation(toolCall, map[string]string{
			"error": "unknown tool: " + toolCall.Function.Name,
		})
	}
}

func executeChatSearchItems(ctx context.Context, gorseClient *gorse.GorseClient, toolCall openai.ToolCall) openai.ChatCompletionMessage {
	var args chatSearchItemsArguments
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return chatToolObservation(toolCall, map[string]string{
			"error": "invalid SearchItems arguments: " + err.Error(),
		})
	}
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return chatToolObservation(toolCall, map[string]string{
			"error": "SearchItems query is required",
		})
	}
	if args.N <= 0 {
		args.N = chatSearchItemsDefaultN
	}
	if args.N > chatSearchItemsMaxN {
		args.N = chatSearchItemsMaxN
	}

	items, err := gorseClient.SearchItems(ctx, args.Query, args.N)
	if err != nil {
		return chatToolObservation(toolCall, map[string]string{
			"error": err.Error(),
		})
	}
	return chatToolObservation(toolCall, map[string]any{
		"query": args.Query,
		"items": items.Items,
	})
}

func chatToolObservation(toolCall openai.ToolCall, value any) openai.ChatCompletionMessage {
	content, err := json.Marshal(value)
	if err != nil {
		content = []byte(fmt.Sprintf(`{"error":"failed to encode tool result: %s"}`, err.Error()))
	}
	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    string(content),
		Name:       toolCall.Function.Name,
		ToolCallID: toolCall.ID,
	}
}
