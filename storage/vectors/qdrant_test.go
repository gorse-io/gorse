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
	"time"

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

func (suite *QdrantTestSuite) TestURL() {
	config, err := qdrantConfig("qdrant://localhost:6333")
	suite.NoError(err)
	suite.Equal("localhost", config.Host)
	suite.Equal(6333, config.Port)
	suite.Empty(config.APIKey)
	suite.False(config.UseTLS)

	// Query options use the Qdrant client's camelCase field names.
	config, err = qdrantConfig("qdrant://xyz.cloud.qdrant.io:6334?apiKey=a%2Bb%26c&useTLS=true&skipCompatibilityCheck=true&poolSize=5&keepAliveTime=30&keepAliveTimeout=4&versionCheckTimeout=5s")
	suite.Require().NoError(err)
	suite.Equal(&qdrant.Config{
		Host: "xyz.cloud.qdrant.io", Port: 6334, APIKey: "a+b&c", UseTLS: true,
		SkipCompatibilityCheck: true, PoolSize: 5, KeepAliveTime: 30,
		KeepAliveTimeout: 4, VersionCheckTimeout: 5 * time.Second,
	}, config)
}

func TestQdrant(t *testing.T) {
	if qdrantUri == "" {
		t.Skip("QDRANT_URI is not set, skipping Qdrant test")
	}
	suite.Run(t, new(QdrantTestSuite))
}
