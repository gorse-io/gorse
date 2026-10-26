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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
)

var (
	weaviateUri string
)

func init() {
	// os.Setenv("WEAVIATE_URI", "weaviate://127.0.0.1:8080")
	weaviateUri = os.Getenv("WEAVIATE_URI")
}

type WeaviateTestSuite struct {
	vectorsTestSuite
}

func (suite *WeaviateTestSuite) TestSparse() {
	suite.T().Skip("sparse vectors are not supported")
}

func (suite *WeaviateTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	var err error
	suite.Database, err = Open(weaviateUri, "gorse_")
	suite.NoError(err)
}

func (suite *WeaviateTestSuite) TestQuantization() {
	suite.testQuantization(QuantizationNone, 0)
	suite.testQuantization(QuantizationSQ, 0)
	suite.testQuantization(QuantizationRQ, 0)
	suite.testQuantization(QuantizationRQ, 1)
	suite.testQuantization(QuantizationRQ, 8)
	suite.testQuantization(QuantizationPQ, 0)
}

func TestWeaviateURL(t *testing.T) {
	config, err := weaviateConfig("weaviate://localhost:8080")
	require.NoError(t, err)
	require.Equal(t, &weaviate.Config{Host: "localhost:8080", Scheme: "http"}, config)

	config, err = weaviateConfig("weaviates://example.com")
	require.NoError(t, err)
	require.Equal(t, &weaviate.Config{Host: "example.com", Scheme: "https"}, config)

	config, err = weaviateConfig("weaviates://[::1]:8443")
	require.NoError(t, err)
	require.Equal(t, &weaviate.Config{Host: "[::1]:8443", Scheme: "https"}, config)

	config, err = weaviateConfig("weaviates://example.com:8443?apiKey=a%2Bb%26c&timeout=1500ms&startupTimeout=5s")
	require.NoError(t, err)
	require.Equal(t, &weaviate.Config{
		Host: "example.com:8443", Scheme: "https", AuthConfig: auth.ApiKey{Value: "a+b&c"},
		Timeout: 1500 * time.Millisecond, StartupTimeout: 5 * time.Second,
	}, config)

	config, err = weaviateConfig("weaviate://localhost:8080?apiKey=&timeout=0s&startupTimeout=0")
	require.NoError(t, err)
	require.Equal(t, &weaviate.Config{Host: "localhost:8080", Scheme: "http"}, config)
}

func TestWeaviate(t *testing.T) {
	if weaviateUri == "" {
		t.Skip("WEAVIATE_URI is not set, skipping Weaviate test")
	}
	suite.Run(t, new(WeaviateTestSuite))
}
