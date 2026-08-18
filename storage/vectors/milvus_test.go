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
	"github.com/juju/errors"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/stretchr/testify/assert"
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
	suite.Error(err)
	_, err = suite.Database.DescribeCollection(ctx, "test_invalid_sparse")
	suite.True(errors.Is(err, errors.NotFound), err)
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

func TestMilvusVectors(t *testing.T) {
	timestampA := time.Now().UTC().Truncate(time.Millisecond)
	timestampB := timestampA.Add(time.Second)
	result := milvusclient.ResultSet{
		ResultCount: 2,
		Fields: milvusclient.DataSet{
			column.NewColumnVarChar(milvusIdField, []string{"a", "b"}),
			column.NewColumnVarCharArray(milvusCategoriesField, [][]string{{"cat-a"}, {"cat-b"}}),
			column.NewColumnBool(milvusHiddenField, []bool{false, true}),
			column.NewColumnInt64(milvusTimestampField, []int64{timestampA.UnixMilli(), timestampB.UnixMilli()}),
			column.NewColumnFloatVector(milvusVectorField, 2, [][]float32{{1, 0}, {0, 1}}),
		},
	}
	vectors, err := milvusVectors("test", []string{"b", "missing", "a"}, result)
	require.NoError(t, err)
	assert.Equal(t, []Vector{
		{Id: "b", Values: []float32{0, 1}, IsHidden: true, Categories: []string{"cat-b"}, Timestamp: timestampB},
		{Id: "a", Values: []float32{1, 0}, Categories: []string{"cat-a"}, Timestamp: timestampA},
	}, vectors)

	sparse, err := entity.NewSliceSparseEmbedding([]uint32{1, 100}, []float32{1, 2})
	require.NoError(t, err)
	result.ResultCount = 1
	result.Fields = milvusclient.DataSet{
		column.NewColumnVarChar(milvusIdField, []string{"sparse"}),
		column.NewColumnVarCharArray(milvusCategoriesField, [][]string{{}}),
		column.NewColumnBool(milvusHiddenField, []bool{false}),
		column.NewColumnInt64(milvusTimestampField, []int64{timestampA.UnixMilli()}),
		column.NewColumnSparseVectors(milvusVectorField, []entity.SparseEmbedding{sparse}),
	}
	vectors, err = milvusVectors("test", []string{"sparse"}, result)
	require.NoError(t, err)
	require.Len(t, vectors, 1)
	assert.Equal(t, []uint32{1, 100}, vectors[0].Indices)
	assert.Equal(t, []float32{1, 2}, vectors[0].Values)

	valid := milvusclient.DataSet{
		column.NewColumnVarChar(milvusIdField, []string{"a"}),
		column.NewColumnVarCharArray(milvusCategoriesField, [][]string{{"cat-a"}}),
		column.NewColumnBool(milvusHiddenField, []bool{false}),
		column.NewColumnInt64(milvusTimestampField, []int64{timestampA.UnixMilli()}),
		column.NewColumnFloatVector(milvusVectorField, 2, [][]float32{{1, 0}}),
	}
	tests := map[string]milvusclient.DataSet{
		"missing id":         valid[1:],
		"missing categories": {valid[0], valid[2], valid[3], valid[4]},
		"missing hidden":     {valid[0], valid[1], valid[3], valid[4]},
		"missing timestamp":  {valid[0], valid[1], valid[2], valid[4]},
		"unsupported vector": {valid[0], valid[1], valid[2], valid[3], column.NewColumnInt64(milvusVectorField, []int64{1})},
		"invalid id row":     {column.NewColumnVarChar(milvusIdField, nil), valid[1], valid[2], valid[3], valid[4]},
	}
	for name, fields := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := milvusVectors("test", []string{"a"}, milvusclient.ResultSet{ResultCount: 1, Fields: fields})
			assert.Error(t, err)
		})
	}
}
