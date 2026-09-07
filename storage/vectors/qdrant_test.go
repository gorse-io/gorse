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
	"os"
	"testing"

	"github.com/gorse-io/gorse/common/log"
	"github.com/qdrant/go-client/qdrant"
	"github.com/stretchr/testify/suite"
)

var (
	qdrantUri string
)

func init() {
	// os.Setenv("QDRANT_URI", "qdrant://127.0.0.1:6334")
	qdrantUri = os.Getenv("QDRANT_URI")
}

type QdrantTestSuite struct {
	vectorsTestSuite
}

func (suite *QdrantTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	var err error
	suite.Database, err = Open(qdrantUri, "gorse_")
	suite.NoError(err)
}

func (suite *QdrantTestSuite) TestQuantization() {
	suite.testQuantization(QuantizationNone, 0)
	suite.testQuantization(QuantizationRQ, 0)
	suite.testQuantization(QuantizationRQ, 1)
	suite.testQuantization(QuantizationRQ, 2)
	suite.testQuantization(QuantizationRQ, 4)
	suite.testQuantization(QuantizationSQ, 0)
	suite.testQuantization(QuantizationSQ, 8)
	suite.testQuantization(QuantizationPQ, 0)
	suite.testQuantization(QuantizationPQ, 1)
	suite.testQuantization(QuantizationPQ, 2)
	suite.testQuantization(QuantizationPQ, 4)
	suite.testQuantization(QuantizationPQ, 8)
}

func (suite *QdrantTestSuite) TestCategoryFilteringStrictMode() {
	ctx := suite.T().Context()
	db := suite.Database.(*Qdrant)
	for _, test := range []struct {
		name       string
		dimensions int
		query      Vector
	}{
		{name: "dense_categories", dimensions: defaultVectorSize, query: Vector{Values: []float32{1, 0, 0, 0}}},
		{name: "sparse_categories", query: Vector{Indices: []uint32{1}, Values: []float32{1}}},
	} {
		suite.Run(test.name, func() {
			suite.Require().NoError(db.AddCollection(ctx, test.name, test.dimensions, Dot, VectorConfig{}))
			suite.Require().NoError(db.client.UpdateCollection(ctx, &qdrant.UpdateCollection{
				CollectionName: test.name,
				StrictModeConfig: &qdrant.StrictModeConfig{
					Enabled:                    new(true),
					UnindexedFilteringRetrieve: new(false),
				},
			}))
			match := test.query
			match.Id = "match"
			match.Categories = []string{"common", "cat-a"}
			other := test.query
			other.Id = "other"
			other.Categories = []string{"common", "cat-b"}
			suite.Require().NoError(db.AddVectors(ctx, test.name, []Vector{match, other}))

			results, err := db.QueryVectors(ctx, test.name, test.query, []string{"common", "cat-a"}, 10)
			suite.Require().NoError(err)
			suite.Require().Len(results, 1)
			suite.Equal("match", results[0].Id)
		})
	}
}

func TestQdrant(t *testing.T) {
	if qdrantUri == "" {
		t.Skip("QDRANT_URI is not set, skipping Qdrant test")
	}
	suite.Run(t, new(QdrantTestSuite))
}
