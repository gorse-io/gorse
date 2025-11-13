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

package worker

import (
	"fmt"
	"testing"

	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/suite"
)

type PipelineTestSuite struct {
	suite.Suite
	dataClient data.Database
}

func (suite *PipelineTestSuite) SetupSuite() {
	var err error
	suite.dataClient, err = data.Open(fmt.Sprintf("sqlite://%s/data.db", suite.T().TempDir()), "")
	suite.NoError(err)
	err = suite.dataClient.Init()
	suite.NoError(err)

	// insert items
	err = suite.dataClient.BatchInsertItems(nil, []data.Item{
		{ItemId: "1"},
		{ItemId: "2"},
		{ItemId: "3"},
		{ItemId: "4"},
		{ItemId: "5"},
	})
	suite.NoError(err)
}

func (suite *PipelineTestSuite) TearDownSuite() {
	err := suite.dataClient.Close()
	suite.NoError(err)
}

func (suite *PipelineTestSuite) TestGetSlice() {
	c := NewItemCache(suite.dataClient)
	items, err := c.GetSlice([]string{"1", "2", "3", "4", "5", "6"})
	suite.NoError(err)
	suite.Equal(5, len(items))
}

func (suite *PipelineTestSuite) TestGetMap() {
	c := NewItemCache(suite.dataClient)
	items, err := c.GetMap([]string{"1", "2", "3", "4", "5", "6"})
	suite.NoError(err)
	suite.Equal(5, len(items))
}

func TestPipeline(t *testing.T) {
	suite.Run(t, new(PipelineTestSuite))
}
