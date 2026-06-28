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
	"time"

	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/juju/errors"
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
	// init database
	err = s.DataClient.Init()
	s.NoError(err)
	err = s.CacheClient.Init()
	s.NoError(err)
}

func (s *MasterTestSuite) TearDownTest() {
	s.NoError(s.DataClient.Close())
	s.NoError(s.CacheClient.Close())
}

func (s *MasterTestSuite) TestInitCollaborativeFilteringVectorCollection() {
	vectorStore := &fakeVectorDatabase{collections: make(map[string]*vectors.CollectionInfo)}
	s.VectorClient = vectorStore
	s.Config.Database.Vector.QuantizationType = string(vectors.QuantizationRQ)
	s.Config.Database.Vector.QuantizationBits = 4

	err := s.initCollaborativeFilteringVectorCollection(context.Background())
	s.NoError(err)
	s.Len(vectorStore.collections, 1)
	info := vectorStore.collections[cache.CollaborativeFiltering]
	s.Require().NotNil(info)
	s.Equal(cache.CollaborativeFiltering, info.Name)
	s.Equal(16, info.Dimension)
	s.Equal(vectors.Dot, info.Distance)
	s.Equal(vectors.QuantizationRQ, info.Type)
	s.Equal(4, info.Bits)
}

func TestMaster(t *testing.T) {
	suite.Run(t, new(MasterTestSuite))
}

type fakeVectorDatabase struct {
	collections map[string]*vectors.CollectionInfo
}

func (db *fakeVectorDatabase) Init() error     { return nil }
func (db *fakeVectorDatabase) Optimize() error { return nil }
func (db *fakeVectorDatabase) Close() error    { return nil }

func (db *fakeVectorDatabase) ListCollections(context.Context) ([]string, error) {
	collections := make([]string, 0, len(db.collections))
	for collection := range db.collections {
		collections = append(collections, collection)
	}
	return collections, nil
}

func (db *fakeVectorDatabase) DescribeCollection(_ context.Context, name string) (*vectors.CollectionInfo, error) {
	info, ok := db.collections[name]
	if !ok {
		return nil, errors.NotFoundf("collection %s", name)
	}
	return info, nil
}

func (db *fakeVectorDatabase) AddCollection(_ context.Context, name string, dimensions int, distance vectors.Distance, config vectors.VectorConfig) error {
	db.collections[name] = &vectors.CollectionInfo{
		Name:         name,
		Dimension:    dimensions,
		Distance:     distance,
		VectorConfig: config,
	}
	return nil
}

func (db *fakeVectorDatabase) DeleteCollection(_ context.Context, name string) error {
	delete(db.collections, name)
	return nil
}

func (db *fakeVectorDatabase) AddVectors(context.Context, string, []vectors.Vector) error {
	return nil
}

func (db *fakeVectorDatabase) DeleteVectors(context.Context, string, time.Time) error {
	return nil
}

func (db *fakeVectorDatabase) QueryVectors(context.Context, string, []float32, []string, int) ([]vectors.Vector, error) {
	return nil, nil
}
