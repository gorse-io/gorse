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
	"strconv"
	"testing"
	"time"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/suite"
)

type UserToUserTestSuite struct {
	suite.Suite
}

func (suite *UserToUserTestSuite) TestEmbedding() {
	timestamp := time.Now()
	user2user, err := newEmbeddingUserToUser(config.UserToUserConfig{
		Column: "user.Labels.description",
	}, 10, timestamp)
	suite.NoError(err)

	for i := 0; i < 100; i++ {
		user2user.Push(&data.User{
			UserId: strconv.Itoa(i),
			Labels: map[string]any{
				"description": []float32{0.1 * float32(i), 0.2 * float32(i), 0.3 * float32(i)},
			},
		}, nil)
	}

	scores := user2user.PopAll(0)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func (suite *UserToUserTestSuite) TestTags() {
	timestamp := time.Now()
	idf := make([]float32, 101)
	for i := range idf {
		idf[i] = 1
	}
	user2user, err := newTagsUserToUser(config.UserToUserConfig{
		Column: "user.Labels",
	}, 10, timestamp, idf)
	suite.NoError(err)

	for i := 0; i < 100; i++ {
		labels := make(map[string]any)
		for j := 1; j <= 100-i; j++ {
			labels[strconv.Itoa(j)] = []dataset.ID{dataset.ID(j)}
		}
		user2user.Push(&data.User{
			UserId: strconv.Itoa(i),
			Labels: labels,
		}, nil)
	}

	scores := user2user.PopAll(0)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func (suite *UserToUserTestSuite) TestItems() {
	timestamp := time.Now()
	idf := make([]float32, 101)
	for i := range idf {
		idf[i] = 1
	}
	user2user, err := newItemsUserToUser(config.UserToUserConfig{}, 10, timestamp, idf)
	suite.NoError(err)

	for i := 0; i < 100; i++ {
		feedback := make([]int32, 0, 100-i)
		for j := 1; j <= 100-i; j++ {
			feedback = append(feedback, int32(j))
		}
		user2user.Push(&data.User{UserId: strconv.Itoa(i)}, feedback)
	}

	scores := user2user.PopAll(0)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func (suite *UserToUserTestSuite) TestAuto() {
	timestamp := time.Now()
	idf := make([]float32, 101)
	for i := range idf {
		idf[i] = 1
	}
	user2user, err := newAutoUserToUser(config.UserToUserConfig{}, 10, timestamp, idf, idf)
	suite.NoError(err)

	for i := 0; i < 100; i++ {
		user := &data.User{UserId: strconv.Itoa(i)}
		feedback := make([]int32, 0, 100-i)
		if i%2 == 0 {
			labels := make(map[string]any)
			for j := 1; j <= 100-i; j++ {
				labels[strconv.Itoa(j)] = []dataset.ID{dataset.ID(j)}
			}
			user.Labels = labels
		} else {
			for j := 1; j <= 100-i; j++ {
				feedback = append(feedback, int32(j))
			}
		}
		user2user.Push(user, feedback)
	}

	scores0 := user2user.PopAll(0)
	suite.Len(scores0, 10)
	scores1 := user2user.PopAll(1)
	suite.Len(scores1, 10)
}

func TestUserToUser(t *testing.T) {
	suite.Run(t, new(UserToUserTestSuite))
}
