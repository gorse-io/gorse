// Copyright 2026 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logics

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/gorse-io/xvec"
	"github.com/stretchr/testify/require"
)

func TestSparseVectorPrunerKeepsHighestValues(t *testing.T) {
	pruner := newSparseVectorPruner(maxSparseVectorNNZ)
	for i := range maxSparseVectorNNZ + 1 {
		pruner.Add(uint32(i), float32(i+1))
	}

	indices, values := pruner.Result()
	if len(indices) != maxSparseVectorNNZ {
		t.Fatalf("expected %d indices, got %d", maxSparseVectorNNZ, len(indices))
	}
	if len(values) != maxSparseVectorNNZ {
		t.Fatalf("expected %d values, got %d", maxSparseVectorNNZ, len(values))
	}
	for i := range indices {
		if indices[i] != uint32(i+1) {
			t.Errorf("index %d: expected %d, got %d", i, i+1, indices[i])
		}
		if values[i] != float32(i+2) {
			t.Errorf("value %d: expected %d, got %f", i, i+2, values[i])
		}
	}
}

func TestSparseVectorPrunerBreaksTiesByIndex(t *testing.T) {
	pruner := newSparseVectorPruner(2)
	pruner.Add(3, 1)
	pruner.Add(2, 1)
	pruner.Add(1, 1)

	indices, values := pruner.Result()
	if len(indices) != 2 || indices[0] != 1 || indices[1] != 2 {
		t.Fatalf("expected indices [1 2], got %v", indices)
	}
	if len(values) != 2 || values[0] != 1 || values[1] != 1 {
		t.Fatalf("expected values [1 1], got %v", values)
	}
}

func TestNewSparseVectorPrunesToMaxNNZ(t *testing.T) {
	ids := make([]int32, maxSparseVectorNNZ+1)
	idf := make([]float32, len(ids))
	for i := range ids {
		ids[i] = int32(i)
		idf[i] = float32(i + 1)
	}

	vector := newSparseVector(ids, idf, 0)

	if len(vector.Indices) != maxSparseVectorNNZ {
		t.Fatalf("expected %d indices, got %d", maxSparseVectorNNZ, len(vector.Indices))
	}
	if len(vector.Values) != maxSparseVectorNNZ {
		t.Fatalf("expected %d values, got %d", maxSparseVectorNNZ, len(vector.Values))
	}
	for i := range vector.Indices {
		if vector.Indices[i] != uint32(i+1) {
			t.Errorf("index %d: expected %d, got %d", i, i+1, vector.Indices[i])
		}
		if math.Abs(float64(vector.Values[i])-math.Sqrt(float64(i+2))) > 1e-6 {
			t.Errorf("value %d: expected sqrt(%d), got %f", i, i+2, vector.Values[i])
		}
	}
}

func TestAppendSparseVectorPrunesCombinedVectorToMaxNNZ(t *testing.T) {
	firstIDs := make([]int32, maxSparseVectorNNZ)
	firstIDF := make([]float32, len(firstIDs))
	secondIDs := make([]int32, maxSparseVectorNNZ)
	secondIDF := make([]float32, len(secondIDs))
	for i := range maxSparseVectorNNZ {
		firstIDs[i] = int32(i)
		firstIDF[i] = 1
		secondIDs[i] = int32(i)
		secondIDF[i] = 2
	}

	vector := newSparseVector(firstIDs, firstIDF, 0)
	vector = appendSparseVector(vector, secondIDs, secondIDF, uint32(len(firstIDF)))

	if len(vector.Indices) != maxSparseVectorNNZ {
		t.Fatalf("expected %d indices, got %d", maxSparseVectorNNZ, len(vector.Indices))
	}
	for i := range vector.Indices {
		expected := uint32(len(firstIDF) + i)
		if vector.Indices[i] != expected {
			t.Errorf("index %d: expected %d, got %d", i, expected, vector.Indices[i])
		}
		if math.Abs(float64(vector.Values[i])-math.Sqrt(2)) > 1e-6 {
			t.Errorf("value %d: expected sqrt(2), got %f", i, vector.Values[i])
		}
	}
}

func TestSparseVectorRepeatedFeedbackDoesNotConsumeCapacity(t *testing.T) {
	ids := make([]int32, 0, 2*maxSparseVectorNNZ)
	idf := make([]float32, maxSparseVectorNNZ)
	for i := range maxSparseVectorNNZ {
		ids = append(ids, int32(i), int32(i))
		idf[i] = 1
	}
	vector := newSparseVector(ids, idf, 0)
	require.Len(t, vector.Indices, maxSparseVectorNNZ)
	for i, index := range vector.Indices {
		require.Equal(t, uint32(i), index)
		require.Equal(t, float32(1), vector.Values[i])
	}
}

func TestVectorWriterCleanCompactsBeforeQuery(t *testing.T) {
	for _, sparse := range []bool{false, true} {
		name := "dense"
		dimension, distance := 2, vectors.Euclidean
		collection := vectors.ItemToItemCollection(name)
		if sparse {
			name = "sparse"
			dimension, distance = 0, vectors.Dot
			collection = vectors.UserToUserCollection(name)
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			root := t.TempDir()
			client, err := vectors.Open("xvec://"+root, "")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			require.NoError(t, client.Init())
			require.NoError(t, client.AddCollection(ctx, collection, dimension, distance, vectors.VectorConfig{}))
			timestamp := time.Now().UTC().Truncate(time.Millisecond)
			makeVector := func(id string, value float32, timestamp time.Time) vectors.Vector {
				v := vectors.Vector{Id: id, Values: []float32{value, 1}, Timestamp: timestamp}
				if sparse {
					v.Indices = []uint32{0, 1}
				}
				return v
			}
			require.NoError(t, client.AddVectors(ctx, collection, []vectors.Vector{
				makeVector("stale", 1, timestamp.Add(-time.Hour)),
				makeVector("current", 1, timestamp.Add(-time.Hour)),
			}))
			writer := newSimilarityVectorWriter(ctx, client, collection, distance, vectors.VectorConfig{}, timestamp, 1024, sparse)
			require.NoError(t, writer.Add(makeVector("current", 2, timestamp)))
			require.NoError(t, writer.Add(makeVector("new", 3, timestamp)))
			require.NoError(t, writer.Clean())
			require.NoError(t, client.Close())

			// Inspect persisted state before any query can build an index lazily.
			persisted, err := xvec.Open(ctx, filepath.Join(root, collection), xvec.CollectionOptions{})
			require.NoError(t, err)
			stats := persisted.Stats()
			require.Equal(t, uint64(2), stats.DocumentCount)
			require.Zero(t, stats.MutableDocuments)
			require.Zero(t, stats.DeletedDocuments)
			require.NotZero(t, stats.ImmutableSegments)
			require.Equal(t, float32(1), stats.IndexCompleteness["vector"])
			require.NoError(t, persisted.Close())

			client, err = vectors.Open("xvec://"+root, "")
			require.NoError(t, err)
			require.NoError(t, client.Init())
			stored, err := client.GetVectors(ctx, collection, []string{"stale", "current", "new"})
			require.NoError(t, err)
			require.Len(t, stored, 2)
			neighbors, err := client.QueryVectors(ctx, collection, makeVector("", 2, timestamp), nil, 2)
			require.NoError(t, err)
			require.Len(t, neighbors, 2)
		})
	}
}
