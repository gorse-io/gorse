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

package logics

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRenderPrompt(t *testing.T) {
	agent, err := NewAgent(config.AgentConfig{
		Name: "assistant",
		PromptTemplate: `Recommend items for user {{ user_id }}:
{% for item in feedback -%}
- {{ item.FeedbackType }} {{ item.ItemId }}
{% endfor -%}`,
	}, config.OpenAIConfig{}, nil, "bob", []data.Feedback{}, nil, mapset.NewSet[string](), 10)
	require.NoError(t, err)

	prompt, err := agent.renderPrompt([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "like", ItemId: "item-1"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "read", ItemId: "item-2"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Recommend items for user bob:\n- like item-1\n- read item-2", prompt)
}

func TestNewAgentInvalidPromptTemplate(t *testing.T) {
	_, err := NewAgent(config.AgentConfig{
		Name:           "assistant",
		PromptTemplate: "{% for item in feedback %}",
	}, config.OpenAIConfig{}, nil, "bob", nil, nil, mapset.NewSet[string](), 10)
	assert.Error(t, err)
}
