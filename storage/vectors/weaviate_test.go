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

func TestWeaviateConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		uri  string
		want *weaviate.Config
	}{
		{"defaults", "weaviate://localhost:8080", &weaviate.Config{Host: "localhost:8080", Scheme: "http"}},
		{"secure", "weaviates://example.com", &weaviate.Config{Host: "example.com", Scheme: "https"}},
		{"ipv6", "weaviates://[::1]:8443", &weaviate.Config{Host: "[::1]:8443", Scheme: "https"}},
		{"options", "weaviates://example.com:8443?apiKey=a%2Bb%26c&timeout=1500ms&startupTimeout=5s", &weaviate.Config{
			Host: "example.com:8443", Scheme: "https", AuthConfig: auth.ApiKey{Value: "a+b&c"},
			Timeout: 1500 * time.Millisecond, StartupTimeout: 5 * time.Second,
		}},
		{"explicit zero", "weaviate://localhost:8080?apiKey=&timeout=0s&startupTimeout=0", &weaviate.Config{Host: "localhost:8080", Scheme: "http"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := weaviateConfig(tc.uri)
			require.NoError(t, err)
			require.Equal(t, tc.want, config)
		})
	}
}

func TestWeaviateConfigInvalid(t *testing.T) {
	for _, query := range []string{
		"timeout=", "timeout=bad", "timeout=5", "timeout=-1s",
		"timeout=9223372036854775808ns", "startupTimeout=", "startupTimeout=no",
		"startupTimeout=-1s", "startupTimeout=9223372036854775808ns",
		"timeout=%zz", "timeout=1s;startupTimeout=2s",
	} {
		t.Run(query, func(t *testing.T) {
			config, err := weaviateConfig("weaviate://localhost:8080?" + query)
			require.Error(t, err)
			require.Nil(t, config)
		})
	}
	_, err := weaviateConfig("weaviate://[::1")
	require.Error(t, err)
}

func TestWeaviate(t *testing.T) {
	if weaviateUri == "" {
		t.Skip("WEAVIATE_URI is not set, skipping Weaviate test")
	}
	suite.Run(t, new(WeaviateTestSuite))
}
