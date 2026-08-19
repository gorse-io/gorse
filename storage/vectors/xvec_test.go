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

func (suite *XvecTestSuite) TestAlreadyExists() {
	ctx := suite.T().Context()
	suite.Require().NoError(suite.Database.AddCollection(ctx, "test_already_exists", defaultVectorSize, Cosine, VectorConfig{}))
	suite.ErrorIs(suite.Database.AddCollection(ctx, "test_already_exists", defaultVectorSize, Cosine, VectorConfig{}), ErrAlreadyExists)
}

func TestXvec(t *testing.T) {
	suite.Run(t, new(XvecTestSuite))
}
