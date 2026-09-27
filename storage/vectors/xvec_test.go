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
	"context"
	"testing"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/xvec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type XvecTestSuite struct {
	vectorsTestSuite
	root string
}

func (suite *XvecTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	suite.root = suite.T().TempDir()
	var err error
	suite.Database, err = Open(storage.XvecPrefix+suite.root, "gorse_")
	suite.Require().NoError(err)
	suite.Require().NoError(suite.Database.Init())
}

func (suite *XvecTestSuite) TearDownSuite() {
	suite.NoError(suite.Database.Close())
}

func TestXvecDenseCollectionSchemaUsesHNSWInt8(t *testing.T) {
	database := new(Xvec)
	schema, err := database.collectionSchema(context.Background(), "test", 3, Cosine, VectorConfig{})
	require.NoError(t, err)

	field, found := schema.Field(xvecVectorField)
	require.True(t, found)
	params, ok := field.EffectiveIndex().(xvec.HNSWIndexParams)
	require.True(t, ok)
	assert.Equal(t, xvec.DataTypeVectorFP32, field.DataType)
	assert.Equal(t, xvec.MetricTypeCosine, params.Metric)
	assert.Equal(t, xvec.QuantizeTypeInt8, params.Quantize)
	assert.True(t, params.Quantizer.EnableRotate)
}

func TestXvec(t *testing.T) {
	suite.Run(t, new(XvecTestSuite))
}
