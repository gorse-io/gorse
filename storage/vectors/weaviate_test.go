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

	"github.com/stretchr/testify/suite"
)

var (
	weaviateUri string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	weaviateUri = env("WEAVIATE_URI", "weaviate://127.0.0.1:8080")
}

type WeaviateTestSuite struct {
	vectorsTestSuite
}

func (suite *WeaviateTestSuite) SetupSuite() {
	var err error
	suite.Database, err = Open(weaviateUri, "gorse_")
	suite.NoError(err)
}

func (suite *WeaviateTestSuite) TestRQQuantization() {
	ctx := suite.T().Context()
	err := suite.Database.AddCollection(ctx, "rq", defaultVectorSize, Cosine, VectorConfig{
		Quantization:     QuantizationRQ,
		QuantizationBits: 8,
	})
	suite.NoError(err)

	vectorA := make([]float32, defaultVectorSize)
	vectorA[0] = 1
	vectorB := make([]float32, defaultVectorSize)
	vectorB[0] = 0.9
	vectorB[1] = 0.1

	err = suite.Database.AddVectors(ctx, "rq", []Vector{
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

	results, err := suite.Database.QueryVectors(ctx, "rq", vectorA, []string{"common"}, 10)
	suite.NoError(err)
	suite.Len(results, 2)
}

func TestWeaviate(t *testing.T) {
	suite.Run(t, new(WeaviateTestSuite))
}
