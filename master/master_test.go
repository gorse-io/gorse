// Copyright 2020 gorse Project Authors
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
package master

import (
	"context"
	"fmt"
	"testing"

	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/stretchr/testify/suite"
)

type MasterTestSuite struct {
	suite.Suite
	Master
}

func (s *MasterTestSuite) SetupTest() {
	// open database
	var err error
	s.tracer = monitor.NewTracer("test")
	s.Config = config.GetDefaultConfig()
	s.DataClient, err = data.Open(fmt.Sprintf("sqlite://%s/data.db", s.T().TempDir()), "")
	s.NoError(err)
	s.CacheClient, err = cache.Open(fmt.Sprintf("sqlite://%s/cache.db", s.T().TempDir()), "")
	s.NoError(err)
	s.VectorClient, err = vectors.Open(fmt.Sprintf("sqlite://%s/vector.db", s.T().TempDir()), "")
	s.NoError(err)
	// init database
	err = s.DataClient.Init()
	s.NoError(err)
	err = s.CacheClient.Init()
	s.NoError(err)
	err = s.VectorClient.Init()
	s.NoError(err)
}

func (s *MasterTestSuite) TearDownTest() {
	s.NoError(s.DataClient.Close())
	s.NoError(s.CacheClient.Close())
	s.NoError(s.VectorClient.Close())
}

func (s *MasterTestSuite) TestInitCollaborativeFilteringVectorCollection() {
	err := s.initCollaborativeFilteringVectorCollection(context.Background())
	s.Require().NoError(err)

	collections, err := s.VectorClient.ListCollections(context.Background())
	s.NoError(err)
	s.Contains(collections, vectors.CollaborativeFiltering)

	info, err := s.VectorClient.DescribeCollection(context.Background(), vectors.CollaborativeFiltering)
	s.Require().NoError(err)
	s.Equal(vectors.CollaborativeFiltering, info.Name)
	s.Equal(16, info.Dimension)
	s.Equal(vectors.Cosine, info.Distance)
	s.Equal(vectors.QuantizationNone, info.Type)
	s.Zero(info.Bits)
}

func (s *MasterTestSuite) TestInitCollaborativeFilteringVectorCollectionRecreateOnMismatch() {
	ctx := context.Background()

	err := s.VectorClient.AddCollection(ctx, vectors.CollaborativeFiltering, 8, vectors.Cosine, vectors.VectorConfig{})
	s.Require().NoError(err)

	err = s.initCollaborativeFilteringVectorCollection(ctx)
	s.Require().NoError(err)

	info, err := s.VectorClient.DescribeCollection(ctx, vectors.CollaborativeFiltering)
	s.Require().NoError(err)
	s.Equal(vectors.CollaborativeFiltering, info.Name)
	s.Equal(16, info.Dimension)
	s.Equal(vectors.Cosine, info.Distance)
	s.Equal(vectors.QuantizationNone, info.Type)
	s.Zero(info.Bits)
}

func TestMaster(t *testing.T) {
	suite.Run(t, new(MasterTestSuite))
}
