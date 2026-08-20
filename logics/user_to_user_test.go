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
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/stretchr/testify/suite"
)

type UserToUserTestSuite struct {
	suite.Suite
	vectorClient vectors.Database
}

func (suite *UserToUserTestSuite) SetupTest() {
	log.SetTestLogger(suite.T())
	var err error
	suite.vectorClient, err = vectors.Open(fmt.Sprintf("xvec://%s/vectors", suite.T().TempDir()), "")
	suite.NoError(err)
	suite.NoError(suite.vectorClient.Init())
}

func (suite *UserToUserTestSuite) TearDownTest() {
	suite.NoError(suite.vectorClient.Close())
}

func (suite *UserToUserTestSuite) TestEmbedding() {
	timestamp := time.Now()
	opts := &UserToUserOptions{Context: suite.T().Context(), VectorClient: suite.vectorClient}
	cfg := config.UserToUserConfig{Name: "embedding", Type: "embedding", Column: "user.Labels.description"}
	user2user, err := newEmbeddingUserToUser(cfg, timestamp, opts)
	suite.NoError(err)

	for i := range 100 {
		user2user.Push(&data.User{
			UserId: strconv.Itoa(i),
			Labels: map[string]any{
				"description": []float32{0.1 * float32(i), 0.2 * float32(i), 0.3 * float32(i)},
			},
		}, nil)
	}

	suite.NoError(user2user.Finish())
	scores, err := QueryUserToUser(suite.T().Context(), opts.VectorClient, cfg, "0", 10)
	suite.NoError(err)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func (suite *UserToUserTestSuite) TestIDFInnerProductScores() {
	opts := &UserToUserOptions{Context: suite.T().Context(), VectorClient: suite.vectorClient}
	cfg := config.UserToUserConfig{Name: "items-idf", Type: "items"}
	user2user, err := newItemsUserToUser(cfg, time.Now(), opts, []float32{0, 1, 2})
	suite.NoError(err)

	user2user.Push(&data.User{UserId: "query"}, []int32{1, 2})
	user2user.Push(&data.User{UserId: "idf-2"}, []int32{2})
	user2user.Push(&data.User{UserId: "idf-1"}, []int32{1})

	suite.NoError(user2user.Finish())
	scores, err := QueryUserToUser(suite.T().Context(), opts.VectorClient, cfg, "query", 2)
	suite.NoError(err)
	suite.Require().Len(scores, 2)
	suite.Equal("idf-2", scores[0].Id)
	suite.InDelta(2, scores[0].Score, 1e-6)
	suite.Equal("idf-1", scores[1].Id)
	suite.InDelta(1, scores[1].Score, 1e-6)
}

func (suite *UserToUserTestSuite) TestTags() {
	timestamp := time.Now()
	opts := &UserToUserOptions{Context: suite.T().Context(), VectorClient: suite.vectorClient}
	idf := make([]float32, 101)
	for i := range idf {
		idf[i] = 1
	}
	cfg := config.UserToUserConfig{Name: "tags", Type: "tags", Column: "user.Labels"}
	user2user, err := newTagsUserToUser(cfg, timestamp, opts, idf)
	suite.NoError(err)

	for i := range 100 {
		labels := make(map[string]any)
		for j := 1; j <= 100-i; j++ {
			labels[strconv.Itoa(j)] = []dataset.ID{dataset.ID(j)}
		}
		user2user.Push(&data.User{
			UserId: strconv.Itoa(i),
			Labels: labels,
		}, nil)
	}

	suite.NoError(user2user.Finish())
	scores, err := QueryUserToUser(suite.T().Context(), opts.VectorClient, cfg, "0", 10)
	suite.NoError(err)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func (suite *UserToUserTestSuite) TestItems() {
	timestamp := time.Now()
	opts := &UserToUserOptions{Context: suite.T().Context(), VectorClient: suite.vectorClient}
	idf := make([]float32, 101)
	for i := range idf {
		idf[i] = 1
	}
	cfg := config.UserToUserConfig{Name: "items", Type: "items"}
	user2user, err := newItemsUserToUser(cfg, timestamp, opts, idf)
	suite.NoError(err)

	for i := range 100 {
		feedback := make([]int32, 0, 100-i)
		for j := 1; j <= 100-i; j++ {
			feedback = append(feedback, int32(j))
		}
		user2user.Push(&data.User{UserId: strconv.Itoa(i)}, feedback)
	}

	suite.NoError(user2user.Finish())
	scores, err := QueryUserToUser(suite.T().Context(), opts.VectorClient, cfg, "0", 10)
	suite.NoError(err)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func (suite *UserToUserTestSuite) TestAuto() {
	timestamp := time.Now()
	opts := &UserToUserOptions{Context: suite.T().Context(), VectorClient: suite.vectorClient}
	idf := make([]float32, 101)
	for i := range idf {
		idf[i] = 1
	}
	cfg := config.UserToUserConfig{Name: "auto", Type: "auto"}
	user2user, err := newAutoUserToUser(cfg, timestamp, opts, idf, idf)
	suite.NoError(err)

	for i := range 100 {
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

	suite.NoError(user2user.Finish())
	scores0, err := QueryUserToUser(suite.T().Context(), opts.VectorClient, cfg, "0", 10)
	suite.NoError(err)
	suite.Len(scores0, 10)
	scores1, err := QueryUserToUser(suite.T().Context(), opts.VectorClient, cfg, "1", 10)
	suite.NoError(err)
	suite.Len(scores1, 10)
}

func TestUserToUser(t *testing.T) {
	suite.Run(t, new(UserToUserTestSuite))
}
