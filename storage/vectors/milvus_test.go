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
	"github.com/gorse-io/gorse/storage"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var (
	milvusUri string
)

func init() {
	// os.Setenv("MILVUS_URI", "milvus://127.0.0.1:19530")
	milvusUri = os.Getenv("MILVUS_URI")
}

type MilvusTestSuite struct {
	vectorsTestSuite
}

func (suite *MilvusTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	var err error
	suite.Database, err = Open(milvusUri, "gorse_")
	suite.NoError(err)
}

func (suite *MilvusTestSuite) TestInvalidSparseCollection() {
	ctx := suite.T().Context()
	err := suite.Database.AddCollection(ctx, "test_invalid_sparse", 0, Cosine, VectorConfig{})
	suite.ErrorIs(err, storage.ErrNotSupported)
	_, err = suite.Database.DescribeCollection(ctx, "test_invalid_sparse")
	suite.ErrorIs(err, storage.ErrNotFound)
}

func (suite *MilvusTestSuite) TestQuantization() {
	suite.testQuantization(QuantizationNone, 0)
	suite.testQuantization(QuantizationRQ, 0)
	suite.testQuantization(QuantizationSQ, 0)
	suite.testQuantization(QuantizationSQ, 8)
	suite.testQuantization(QuantizationPQ, 0)
	suite.testQuantization(QuantizationPQ, 8)
}

func TestMilvusConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		config, err := milvusConfig("milvus://localhost:19530")
		require.NoError(t, err)
		require.Equal(t, &milvusclient.ClientConfig{Address: "localhost:19530"}, config)
	})
	t.Run("URI credentials and database", func(t *testing.T) {
		config, err := milvusConfig("milvus://root:p%40ss@localhost:19530/my%5Fdb")
		require.NoError(t, err)
		require.Equal(t, &milvusclient.ClientConfig{
			Address: "localhost:19530", Username: "root", Password: "p@ss", DBName: "my_db",
		}, config)
	})
	t.Run("query options override URI", func(t *testing.T) {
		config, err := milvusConfig("milvus://old:old@localhost:19530/old?username=root&password=p%40ss&dbName=new&apiKey=token&enableTLSAuth=true&dialTimeout=250ms")
		require.NoError(t, err)
		require.Equal(t, "localhost:19530", config.Address)
		require.Equal(t, "root", config.Username)
		require.Equal(t, "p@ss", config.Password)
		require.Equal(t, "new", config.DBName)
		require.Equal(t, "token", config.APIKey)
		require.True(t, config.EnableTLSAuth)
		require.Len(t, config.DialOptions, 1)
	})
	t.Run("explicit defaults", func(t *testing.T) {
		config, err := milvusConfig("milvus://old:old@localhost:19530/old?username=&password=&dbName=&enableTLSAuth=false&dialTimeout=0s")
		require.NoError(t, err)
		require.Equal(t, &milvusclient.ClientConfig{Address: "localhost:19530"}, config)
	})
	for _, uri := range []string{
		"milvus://localhost:bad",
		"milvus://localhost?apiKey=%zz",
		"milvus://localhost?apiKey=token;bad=value",
		"milvus://localhost?enableTLSAuth=invalid",
		"milvus://localhost?enableTLSAuth=",
		"milvus://localhost?dialTimeout=invalid",
		"milvus://localhost?dialTimeout=",
		"milvus://localhost?dialTimeout=-1s",
		"milvus://localhost?dialTimeout=9223372036854775808ns",
	} {
		t.Run(uri, func(t *testing.T) {
			_, err := milvusConfig(uri)
			require.Error(t, err)
		})
	}
}

func TestMilvus(t *testing.T) {
	if milvusUri == "" {
		t.Skip("MILVUS_URI is not set, skipping Milvus test")
	}
	suite.Run(t, new(MilvusTestSuite))
}
