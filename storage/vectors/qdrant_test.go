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
	"net"
	"os"
	"testing"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage"
	"github.com/qdrant/go-client/qdrant"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	qdrantUri string
)

func init() {
	// os.Setenv("QDRANT_URI", "qdrant://127.0.0.1:6334")
	qdrantUri = os.Getenv("QDRANT_URI")
}

type QdrantTestSuite struct {
	vectorsTestSuite
}

func (suite *QdrantTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	var err error
	suite.Database, err = Open(qdrantUri, "gorse_")
	suite.NoError(err)
}

func (suite *QdrantTestSuite) TestQuantization() {
	suite.testQuantization(QuantizationNone, 0)
	suite.testQuantization(QuantizationRQ, 0)
	suite.testQuantization(QuantizationRQ, 1)
	suite.testQuantization(QuantizationRQ, 2)
	suite.testQuantization(QuantizationRQ, 4)
	suite.testQuantization(QuantizationSQ, 0)
	suite.testQuantization(QuantizationSQ, 8)
	suite.testQuantization(QuantizationPQ, 0)
	suite.testQuantization(QuantizationPQ, 1)
	suite.testQuantization(QuantizationPQ, 2)
	suite.testQuantization(QuantizationPQ, 4)
	suite.testQuantization(QuantizationPQ, 8)
}

func TestQdrant(t *testing.T) {
	if qdrantUri == "" {
		t.Skip("QDRANT_URI is not set, skipping Qdrant test")
	}
	suite.Run(t, new(QdrantTestSuite))
}

type qdrantScoreCollectionsServer struct {
	qdrant.UnimplementedCollectionsServer
	distance qdrant.Distance
	err      error
}

func (s *qdrantScoreCollectionsServer) Get(context.Context, *qdrant.GetCollectionInfoRequest) (*qdrant.GetCollectionInfoResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &qdrant.GetCollectionInfoResponse{Result: &qdrant.CollectionInfo{
		Config: &qdrant.CollectionConfig{Params: &qdrant.CollectionParams{
			VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
				qdrantVectorName: {Size: 4, Distance: s.distance},
			}),
		}},
	}}, nil
}

type qdrantScorePointsServer struct {
	qdrant.UnimplementedPointsServer
}

func (*qdrantScorePointsServer) Query(context.Context, *qdrant.QueryPoints) (*qdrant.QueryResponse, error) {
	return &qdrant.QueryResponse{Result: []*qdrant.ScoredPoint{
		{Score: 0.5}, {Score: 1}, {Score: 2},
	}}, nil
}

func TestQdrantQueryScores(t *testing.T) {
	for _, test := range []struct {
		name     string
		distance qdrant.Distance
		sparse   bool
		missing  bool
		want     []float32
	}{
		{name: "euclidean", distance: qdrant.Distance_Euclid, want: []float32{-0.5, -1, -2}},
		{name: "cosine", distance: qdrant.Distance_Cosine, want: []float32{0.5, 1, 2}},
		{name: "dot", distance: qdrant.Distance_Dot, want: []float32{0.5, 1, 2}},
		{name: "sparse", sparse: true, want: []float32{0.5, 1, 2}},
		{name: "missing collection", missing: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			server := grpc.NewServer()
			collections := &qdrantScoreCollectionsServer{distance: test.distance}
			if test.missing || test.sparse {
				collections.err = status.Error(codes.NotFound, "collection not found")
			}
			qdrant.RegisterCollectionsServer(server, collections)
			qdrant.RegisterPointsServer(server, &qdrantScorePointsServer{})
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(server.Stop)
			client, err := qdrant.NewClient(&qdrant.Config{
				Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port,
				SkipCompatibilityCheck: true,
			})
			require.NoError(t, err)
			db := &Qdrant{client: client}
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			query := Vector{Values: []float32{1, 0, 0, 0}}
			if test.sparse {
				query = Vector{Indices: []uint32{1}, Values: []float32{1}}
			}
			results, err := db.QueryVectors(t.Context(), "test", query, nil, 3)
			if test.missing {
				require.ErrorIs(t, err, storage.ErrNotFound)
				return
			}
			require.NoError(t, err)
			require.Len(t, results, len(test.want))
			for i, score := range test.want {
				require.Equal(t, score, results[i].Score)
			}
		})
	}
}
