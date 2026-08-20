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
	"net/url"
	"os"
	"slices"
	"testing"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/stretchr/testify/suite"
)

const milvusTestDatabase = "gorse_vectors_test"

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
	ctx := suite.T().Context()
	u, err := url.Parse(milvusUri)
	suite.Require().NoError(err)
	u.Path = ""

	adminDatabase, err := Open(u.String(), "gorse_")
	suite.Require().NoError(err)
	admin := adminDatabase.(*Milvus)
	databases, err := admin.client.ListDatabase(ctx, milvusclient.NewListDatabaseOption())
	suite.Require().NoError(err)
	if slices.Contains(databases, milvusTestDatabase) {
		err = admin.client.DropDatabase(ctx, milvusclient.NewDropDatabaseOption(milvusTestDatabase))
		suite.Require().NoError(err)
	}
	err = admin.client.CreateDatabase(ctx, milvusclient.NewCreateDatabaseOption(milvusTestDatabase))
	suite.Require().NoError(err)
	suite.Require().NoError(adminDatabase.Close())

	u.Path = "/" + milvusTestDatabase
	suite.Database, err = Open(u.String(), "gorse_")
	suite.Require().NoError(err)
	suite.Equal(milvusTestDatabase, suite.Database.(*Milvus).database)
}

func (suite *MilvusTestSuite) TearDownSuite() {
	suite.NoError(suite.Database.Close())
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

func TestMilvus(t *testing.T) {
	if milvusUri == "" {
		t.Skip("MILVUS_URI is not set, skipping Milvus test")
	}
	suite.Run(t, new(MilvusTestSuite))
}
