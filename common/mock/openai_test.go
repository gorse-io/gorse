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
	"context"
	"github.com/juju/errors"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/suite"
	"io"
	"strings"
	"testing"
)

type OpenAITestSuite struct {
	suite.Suite
	server *OpenAIServer
	client *openai.Client
}

func (suite *OpenAITestSuite) SetupSuite() {
	// Start mock server
	suite.server = NewOpenAIServer()
	go func() {
		_ = suite.server.Start()
	}()
	suite.server.Ready()
	// Create client
	clientConfig := openai.DefaultConfig(suite.server.AuthToken())
	clientConfig.BaseURL = suite.server.BaseURL()
	suite.client = openai.NewClientWithConfig(clientConfig)
}

func (suite *OpenAITestSuite) TearDownSuite() {
	suite.NoError(suite.server.Close())
}

func (suite *OpenAITestSuite) TestChatCompletion() {
	resp, err := suite.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: "qwen2.5",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Hello",
				},
			},
		},
	)
	suite.NoError(err)
	suite.Equal("Hello", resp.Choices[0].Message.Content)
}

func (suite *OpenAITestSuite) TestChatCompletionStream() {
	content := "In my younger and more vulnerable years my father gave me some advice that I've been turning over in" +
		" my mind ever since. Whenever you feel like criticizing anyone, he told me, just remember that all the " +
		"people in this world haven't had the advantages that you've had."
	stream, err := suite.client.CreateChatCompletionStream(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: "qwen2.5",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: content,
				},
			},
			Stream: true,
		},
	)
	suite.NoError(err)
	defer stream.Close()
	var buffer strings.Builder
	for {
		var resp openai.ChatCompletionStreamResponse
		resp, err = stream.Recv()
		if errors.Is(err, io.EOF) {
			suite.Equal(content, buffer.String())
			return
		}
		suite.NoError(err)
		buffer.WriteString(resp.Choices[0].Delta.Content)
	}
}

func (suite *OpenAITestSuite) TestEmbeddings() {
	suite.server.Embeddings([]float32{1, 2, 3})
	resp, err := suite.client.CreateEmbeddings(
		context.Background(),
		openai.EmbeddingRequest{
			Input: "Hello",
			Model: "mxbai-embed-large",
		},
	)
	suite.NoError(err)
	suite.Equal([]float32{1, 2, 3}, resp.Data[0].Embedding)
}

func TestOpenAITestSuite(t *testing.T) {
	suite.Run(t, new(OpenAITestSuite))
}
