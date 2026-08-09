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

	"github.com/gorse-io/gorse/common/log"
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

func (s *MasterTestSuite) SetupSuite() {
	log.SetTestLogger(s.T())
}

func (s *MasterTestSuite) SetupTest() {
	log.SetTestLogger(s.T())
	// open database
	var err error
	s.tracer = monitor.NewTracer("test")
	s.Config = config.GetDefaultConfig()
	s.DataClient, err = data.Open(fmt.Sprintf("sqlite://%s/data.db", s.T().TempDir()), "")
	s.NoError(err)
	s.CacheClient, err = cache.Open(fmt.Sprintf("sqlite://%s/cache.db", s.T().TempDir()), "")
	s.NoError(err)
	s.VectorClient, err = vectors.Open(fmt.Sprintf("zvec://%s/vectors", s.T().TempDir()), "")
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
	collection := vectors.CollaborativeFilteringCollection(1)
	err := s.initCollaborativeFilteringVectorCollection(context.Background(), collection, 16)
	s.Require().NoError(err)

	collections, err := s.VectorClient.ListCollections(context.Background())
	s.NoError(err)
	s.Contains(collections, collection)

	info, err := s.VectorClient.DescribeCollection(context.Background(), collection)
	s.Require().NoError(err)
	s.Equal(collection, info.Name)
	s.Equal(16, info.Dimension)
	s.Equal(vectors.Dot, info.Distance)
	s.Equal(vectors.QuantizationNone, info.Type)
	s.Zero(info.Bits)
}

func (s *MasterTestSuite) TestInitCollaborativeFilteringVectorCollectionRecreateOnMismatch() {
	ctx := context.Background()
	collection := vectors.CollaborativeFilteringCollection(1)
	otherCollection := vectors.CollaborativeFilteringCollection(2)

	err := s.VectorClient.AddCollection(ctx, collection, 8, vectors.Dot, vectors.VectorConfig{})
	s.Require().NoError(err)
	err = s.VectorClient.AddCollection(ctx, otherCollection, 8, vectors.Dot, vectors.VectorConfig{})
	s.Require().NoError(err)

	err = s.initCollaborativeFilteringVectorCollection(ctx, collection, 16)
	s.Require().NoError(err)

	info, err := s.VectorClient.DescribeCollection(ctx, collection)
	s.Require().NoError(err)
	s.Equal(collection, info.Name)
	s.Equal(16, info.Dimension)
	s.Equal(vectors.Dot, info.Distance)
	s.Equal(vectors.QuantizationNone, info.Type)
	s.Zero(info.Bits)
	otherInfo, err := s.VectorClient.DescribeCollection(ctx, otherCollection)
	s.Require().NoError(err)
	s.Equal(8, otherInfo.Dimension)
}

func (s *MasterTestSuite) TestWriteCollaborativeFilteringItemsUsesModelVersion() {
	ctx := s.T().Context()
	oldCollection := vectors.CollaborativeFilteringCollection(1)
	s.Require().NoError(s.initCollaborativeFilteringVectorCollection(ctx, oldCollection, 2))
	s.Require().NoError(s.VectorClient.AddVectors(ctx, oldCollection, []vectors.Vector{{Id: "old", Vector: []float32{1, 0}}}))
	s.Require().NoError(s.DataClient.BatchInsertItems(ctx, []data.Item{{ItemId: "new", Categories: []string{"category"}}}))
	newCollection := vectors.CollaborativeFilteringCollection(2)
	s.Require().NoError(s.initCollaborativeFilteringVectorCollection(ctx, newCollection, 2))
	s.Require().NoError(s.VectorClient.AddVectors(ctx, newCollection, []vectors.Vector{{Id: "stale", Vector: []float32{1, 0}}}))

	err := s.writeCollaborativeFilteringItems(ctx, 2, []vectors.Vector{{Id: "new", Vector: []float32{1, 0}}})
	s.Require().NoError(err)
	oldCount, err := s.VectorClient.CountVectors(ctx, oldCollection)
	s.Require().NoError(err)
	s.Equal(int64(1), oldCount)
	newCount, err := s.VectorClient.CountVectors(ctx, newCollection)
	s.Require().NoError(err)
	s.Equal(int64(1), newCount)
	results, err := s.VectorClient.QueryVectors(ctx, newCollection, []float32{1, 0}, nil, 1)
	s.Require().NoError(err)
	s.Require().Len(results, 1)
	s.Equal("new", results[0].Id)
	s.Equal([]string{"category"}, results[0].Categories)
}

func (s *MasterTestSuite) TestNextCollaborativeFilteringModelID() {
	s.collaborativeFilteringModelMutex.Lock()
	oldMeta := s.collaborativeFilteringMeta
	oldLastModelID := s.collaborativeFilteringLastModelID
	s.collaborativeFilteringMeta.ID = 9_000_000_000_000
	s.collaborativeFilteringLastModelID = 0
	current := s.collaborativeFilteringMeta.ID
	s.collaborativeFilteringModelMutex.Unlock()
	defer func() {
		s.collaborativeFilteringModelMutex.Lock()
		s.collaborativeFilteringMeta = oldMeta
		s.collaborativeFilteringLastModelID = oldLastModelID
		s.collaborativeFilteringModelMutex.Unlock()
	}()

	first := s.nextCollaborativeFilteringModelID()
	second := s.nextCollaborativeFilteringModelID()
	s.Equal(current+1, first)
	s.Equal(first+1, second)
}

func (s *MasterTestSuite) TestInitCollaborativeFilteringVectorCollectionRecreateOnDistanceMismatch() {
	ctx := context.Background()
	collection := vectors.CollaborativeFilteringCollection(1)
	s.Require().NoError(s.VectorClient.AddCollection(ctx, collection, 16, vectors.Cosine, vectors.VectorConfig{}))

	s.Require().NoError(s.initCollaborativeFilteringVectorCollection(ctx, collection, 16))
	info, err := s.VectorClient.DescribeCollection(ctx, collection)
	s.Require().NoError(err)
	s.Equal(vectors.Dot, info.Distance)
}

func TestMaster(t *testing.T) {
	suite.Run(t, new(MasterTestSuite))
}
