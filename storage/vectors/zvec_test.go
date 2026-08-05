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
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/zvec"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ZvecTestSuite struct {
	vectorsTestSuite
	root string
}

func (suite *ZvecTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	suite.root = suite.T().TempDir()
	var err error
	suite.Database, err = Open(storage.ZvecPrefix+suite.root, "gorse_")
	suite.Require().NoError(err)
	suite.Require().NoError(suite.Database.Init())
}

func (suite *ZvecTestSuite) TearDownSuite() {
	suite.NoError(suite.Database.Close())
}

func (suite *ZvecTestSuite) TestDiskANNIsDefaultIndex() {
	ctx := suite.T().Context()
	suite.Require().NoError(suite.Database.AddCollection(ctx, "test_diskann", defaultVectorSize, Dot, VectorConfig{}))

	database := suite.Database.(*Zvec)
	collection, err := database.collection("test_diskann")
	suite.Require().NoError(err)
	field, found := collection.Schema().Field("vector")
	suite.Require().True(found)
	suite.Equal(zvec.IndexTypeDiskANN, field.IndexType())
	_, err = os.Stat(filepath.Join(suite.root, "gorse_test_diskann"))
	suite.NoError(err)
}

func (suite *ZvecTestSuite) TestCategoryWithQuote() {
	ctx := suite.T().Context()
	suite.Require().NoError(suite.Database.AddCollection(ctx, "test_quote", defaultVectorSize, Cosine, VectorConfig{}))
	suite.Require().NoError(suite.Database.AddVectors(ctx, "test_quote", []Vector{{
		Id:         "quoted",
		Vector:     []float32{1, 0, 0, 0},
		Categories: []string{"kid's"},
	}}))

	results, err := suite.Database.QueryVectors(ctx, "test_quote", []float32{1, 0, 0, 0}, []string{"kid's"}, 10)
	suite.Require().NoError(err)
	suite.Require().Len(results, 1)
	suite.Equal("quoted", results[0].Id)
}

func TestZvecReopen(t *testing.T) {
	log.SetTestLogger(t)
	ctx := t.Context()
	root := filepath.Join(t.TempDir(), "vectors")

	database, err := Open(storage.ZvecPrefix+root, "gorse_")
	require.NoError(t, err)
	require.NoError(t, database.Init())
	require.NoError(t, database.AddCollection(ctx, "test_reopen", defaultVectorSize, Dot, VectorConfig{}))
	require.NoError(t, database.AddVectors(ctx, "test_reopen", []Vector{{Id: "a", Vector: []float32{2, 0, 0, 0}}}))
	require.NoError(t, database.Close())

	database, err = Open(storage.ZvecPrefix+root, "gorse_")
	require.NoError(t, err)
	require.NoError(t, database.Init())
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	count, err := database.CountVectors(ctx, "test_reopen")
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	results, err := database.QueryVectors(ctx, "test_reopen", []float32{1, 0, 0, 0}, nil, 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "a", results[0].Id)
}

func TestZvecConcurrentCollectionWrites(t *testing.T) {
	log.SetTestLogger(t)
	ctx := t.Context()
	database, err := Open(storage.ZvecPrefix+filepath.Join(t.TempDir(), "vectors"), "gorse_")
	require.NoError(t, err)
	require.NoError(t, database.Init())
	t.Cleanup(func() { require.NoError(t, database.Close()) })

	const collectionCount = 8
	readerReady := make(chan struct{})
	readerDone := make(chan struct{})
	readerErr := make(chan error, 1)
	go func() {
		close(readerReady)
		for {
			select {
			case <-readerDone:
				readerErr <- nil
				return
			default:
				if _, err := database.ListCollections(ctx); err != nil {
					readerErr <- err
					return
				}
			}
		}
	}()
	<-readerReady

	errCh := make(chan error, collectionCount)
	var waitGroup sync.WaitGroup
	for i := range collectionCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			errCh <- database.AddCollection(ctx, fmt.Sprintf("collection_%d", i), defaultVectorSize, Dot, VectorConfig{})
		}()
	}
	waitGroup.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	collections, err := database.ListCollections(ctx)
	require.NoError(t, err)
	require.Len(t, collections, collectionCount)

	errCh = make(chan error, collectionCount)
	for _, collection := range collections {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			errCh <- database.DeleteCollection(ctx, collection)
		}()
	}
	waitGroup.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	collections, err = database.ListCollections(ctx)
	require.NoError(t, err)
	require.Empty(t, collections)
	close(readerDone)
	require.NoError(t, <-readerErr)
}

func TestZvecConcurrentClose(t *testing.T) {
	log.SetTestLogger(t)
	ctx := t.Context()
	database, err := Open(storage.ZvecPrefix+filepath.Join(t.TempDir(), "vectors"), "gorse_")
	require.NoError(t, err)
	require.NoError(t, database.Init())
	require.NoError(t, database.AddCollection(ctx, "collection", defaultVectorSize, Dot, VectorConfig{}))

	const closerCount = 8
	errCh := make(chan error, closerCount)
	var waitGroup sync.WaitGroup
	for range closerCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			errCh <- database.Close()
		}()
	}
	waitGroup.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	_, err = database.ListCollections(ctx)
	require.ErrorContains(t, err, "zvec database is closed")
}

func TestZvec(t *testing.T) {
	suite.Run(t, new(ZvecTestSuite))
}
