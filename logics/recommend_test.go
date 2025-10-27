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
	"testing"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/suite"
)

type RecommenderTestSuite struct {
	suite.Suite
	recommender *Recommender
}

func (suite *RecommenderTestSuite) SetupSuite() {
	// open database
	dataClient, err := data.Open(fmt.Sprintf("sqlite://%s/data.db", suite.T().TempDir()), "")
	suite.NoError(err)
	cacheClient, err := cache.Open(fmt.Sprintf("sqlite://%s/cache.db", suite.T().TempDir()), "")
	suite.NoError(err)
	// init database
	err = dataClient.Init()
	suite.NoError(err)
	err = cacheClient.Init()
	suite.NoError(err)
	suite.recommender = NewRecommender(config.Config{}, cacheClient, dataClient, "user_1", []string{}, []string{})
}

func (suite *RecommenderTestSuite) TearDownSuite() {
	err := suite.recommender.dataClient.Close()
	suite.NoError(err)
	err = suite.recommender.cacheClient.Close()
	suite.NoError(err)
}

func (suite *RecommenderTestSuite) TestLatest() {

}

func TestRecommenderTestSuite(t *testing.T) {
	suite.Run(t, new(RecommenderTestSuite))
}
