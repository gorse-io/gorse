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
	"testing"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage"
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

func (suite *XvecTestSuite) TestDenseFP16Readback() {
	ctx := suite.T().Context()
	suite.Require().NoError(suite.Database.AddCollection(ctx, "dense", 4, Cosine, VectorConfig{}))
	values := []float32{0.1, -0.2, 3.14159, 65504}
	suite.Require().NoError(suite.Database.AddVectors(ctx, "dense", []Vector{{Id: "vector", Values: values}}))

	stored, err := suite.Database.GetVectors(ctx, "dense", []string{"vector"})
	suite.Require().NoError(err)
	suite.Require().Len(stored, 1)
	suite.Equal(float32Vector(xvecVectorFP16(values)), stored[0].Values)
	suite.NotEqual(values, stored[0].Values)
}

func TestXvec(t *testing.T) {
	suite.Run(t, new(XvecTestSuite))
}
