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

package vectors

import (

	"github.com/stretchr/testify/suite"
)

const defaultVectorSize = 4

type vectorsTestSuite struct {
	suite.Suite
	Database
}

func (suite *vectorsTestSuite) SetupTest() {
	// purge
	ctx := suite.T().Context()
	collections, err := suite.Database.ListCollections(ctx)
	suite.NoError(err)
	for _, collection := range collections {
		err = suite.Database.DeleteCollection(ctx, collection)
		suite.NoError(err)
	}
}

func (suite *vectorsTestSuite) TestCollections() {
	ctx := suite.T().Context()
	// list collections
	collections, err := suite.Database.ListCollections(ctx)
	suite.NoError(err)
	suite.Empty(collections)
	// create collection
	err = suite.Database.AddCollection(ctx, "test", defaultVectorSize, Cosine)
	suite.NoError(err)
	// list collections
	collections, err = suite.Database.ListCollections(ctx)
	suite.NoError(err)
	suite.Equal([]string{"test"}, collections)
	// delete collection
	err = suite.Database.DeleteCollection(ctx, "test")
	suite.NoError(err)
	// list collections
	collections, err = suite.Database.ListCollections(ctx)
	suite.NoError(err)
	suite.Empty(collections)
	// delete non-existent collection
	err = suite.Database.DeleteCollection(ctx, "non-existent")
	suite.Error(err)
}

func (suite *vectorsTestSuite) TestVectors() {
	ctx := suite.T().Context()
	err := suite.Database.AddCollection(ctx, "test", defaultVectorSize, Cosine)
	suite.NoError(err)

	vectorA := make([]float32, defaultVectorSize)
	vectorA[0] = 1
	vectorB := make([]float32, defaultVectorSize)
	vectorB[0] = 0.9
	vectorB[1] = 0.1

	err = suite.Database.AddVectors(ctx, "test", []Vector{
		{
			Id:         "a",
			Vector:     vectorA,
			Categories: []string{"cat-a", "common"},
		},
		{
			Id:         "b",
			Vector:     vectorB,
			Categories: []string{"cat-b", "common"},
		},
	})
	suite.NoError(err)

	results, err := suite.Database.QueryVectors(ctx, "test", vectorA, []string{"cat-a"}, 10)
	suite.NoError(err)
	suite.Len(results, 1)
	suite.Equal("a", results[0].Id)
	suite.NotEmpty(results[0].Categories)

	results, err = suite.Database.QueryVectors(ctx, "test", vectorA, []string{"common"}, 10)
	suite.NoError(err)
	suite.Len(results, 2)
	ids := map[string]bool{}
	for _, result := range results {
		ids[result.Id] = true
		suite.NotEmpty(result.Categories)
	}
	suite.True(ids["a"])
	suite.True(ids["b"])

	results, err = suite.Database.QueryVectors(ctx, "test", vectorA, nil, 1)
	suite.NoError(err)
	suite.NotEmpty(results)
	for _, result := range results {
		suite.NotEmpty(result.Categories)
	}
}
