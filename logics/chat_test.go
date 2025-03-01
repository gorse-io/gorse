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

package logics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/common/mock"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/data"
)

func TestChatRankerhat(t *testing.T) {
	mockAI := mock.NewOpenAIServer()
	go func() {
		_ = mockAI.Start()
	}()
	mockAI.Ready()
	defer mockAI.Close()

	ranker, err := NewChatRanker(config.OpenAIConfig{
		BaseURL:             mockAI.BaseURL(),
		AuthToken:           mockAI.AuthToken(),
		ChatCompletionModel: "deepseek-r1",
		EmbeddingModel:      "text-similarity-ada-001",
	}, "{{ user.UserId }} is a {{ user.Comment }}.")
	assert.NoError(t, err)
	_, err = ranker.Rank(&data.User{
		UserId:  "Tom",
		Comment: "horror movie enthusiast",
	}, []*FeedbackItem{
		{},
	}, []*data.Item{
		{ItemId: "tt0890870", Comment: "Saw IV"},
		{ItemId: "tt0435761", Comment: "Saw V"},
		{ItemId: "tt1132626", Comment: "Saw VI"},
	})
	assert.NoError(t, err)
	t.Fail()
}
