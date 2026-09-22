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
	"container/heap"
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	defaultSimilarityVectorBatchSize = 1024
	maxSparseVectorNNZ               = 1024
)

// VectorWriter owns collection validation and batched vector writes
// for item-to-item and user-to-user recommenders.
type VectorWriter struct {
	ctx          context.Context
	client       vectors.Database
	collection   string
	distance     vectors.Distance
	vectorConfig vectors.VectorConfig
	timestamp    time.Time
	batchSize    int
	sparse       bool

	mu               sync.Mutex
	dimension        *int
	collectionExists bool
	buffer           []vectors.Vector
}

func newSimilarityVectorWriter(
	ctx context.Context,
	client vectors.Database,
	collection string,
	distance vectors.Distance,
	vectorConfig vectors.VectorConfig,
	timestamp time.Time,
	batchSize int,
	sparse bool,
) *VectorWriter {
	if ctx == nil {
		ctx = context.Background()
	}
	if batchSize <= 0 {
		batchSize = defaultSimilarityVectorBatchSize
	}
	writer := &VectorWriter{
		ctx:          ctx,
		client:       client,
		collection:   collection,
		distance:     distance,
		vectorConfig: vectorConfig,
		timestamp:    timestamp,
		batchSize:    batchSize,
		sparse:       sparse,
	}
	if sparse {
		writer.dimension = new(int)
		writer.vectorConfig = vectors.VectorConfig{}
	}
	return writer
}

func (w *VectorWriter) Add(vector vectors.Vector) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(vector.HValues) > 0 {
		if w.sparse {
			return nil
		}
	} else if w.sparse {
		if len(vector.Indices) == 0 || len(vector.Indices) != len(vector.Values) {
			return nil
		}
	} else {
		if len(vector.Indices) != 0 || vector.Len() == 0 {
			return nil
		}
	}
	w.buffer = append(w.buffer, vector)
	if len(w.buffer) >= w.batchSize {
		return w.flushLocked()
	}
	return nil
}

func (w *VectorWriter) Clean() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.flushLocked(); err != nil {
		return err
	}
	if err := w.client.DeleteVectors(w.ctx, w.collection, w.timestamp); err != nil && !errors.Is(err, storage.ErrNotFound) {
		return errors.WithStack(err)
	}
	return nil
}

func (w *VectorWriter) ensureCollectionLocked() error {
	if w.collectionExists {
		return nil
	}
	if w.client == nil {
		return fmt.Errorf("vector database %w", storage.ErrNoDatabase)
	}
	info, err := w.client.DescribeCollection(w.ctx, w.collection)
	if errors.Is(err, storage.ErrNotFound) {
		info = nil
	} else if err != nil {
		return errors.WithStack(err)
	}
	dimension := *w.dimension
	if info != nil && (info.Dimension != dimension || info.Distance != w.distance || info.Type != w.vectorConfig.Type || info.Bits != w.vectorConfig.Bits) {
		log.Logger().Warn("recreating similarity vector collection",
			zap.String("collection", w.collection),
			zap.Int("dimension", dimension),
			zap.Int("distance", int(w.distance)))
		if err = w.client.DeleteCollection(w.ctx, w.collection); err != nil && !errors.Is(err, storage.ErrNotFound) {
			return errors.WithStack(err)
		}
		info = nil
	}
	if info == nil {
		if err = w.client.AddCollection(w.ctx, w.collection, dimension, w.distance, w.vectorConfig); err != nil {
			return errors.WithStack(err)
		}
	}
	w.collectionExists = true
	return nil
}

func (w *VectorWriter) flushLocked() error {
	if len(w.buffer) == 0 {
		return nil
	}
	if !w.sparse {
		if w.dimension == nil {
			counts := make(map[int]int)
			for _, vector := range w.buffer {
				counts[vector.Len()]++
			}
			dimension := w.buffer[0].Len()
			for _, vector := range w.buffer {
				candidate := vector.Len()
				if counts[candidate] > counts[dimension] {
					dimension = candidate
				}
			}
			w.dimension = &dimension
		}
		dimension := *w.dimension
		vectorsWithExpectedDimension := w.buffer[:0]
		for _, vector := range w.buffer {
			if vector.Len() != dimension {
				log.Logger().Error("invalid similarity vector dimension",
					zap.String("collection", w.collection),
					zap.String("id", vector.Id),
					zap.Int("dimension", vector.Len()),
					zap.Int("expected_dimension", dimension))
				continue
			}
			vectorsWithExpectedDimension = append(vectorsWithExpectedDimension, vector)
		}
		w.buffer = vectorsWithExpectedDimension
	}
	if err := w.ensureCollectionLocked(); err != nil {
		return err
	}
	if err := w.client.AddVectors(w.ctx, w.collection, w.buffer); err != nil {
		return errors.WithStack(err)
	}
	w.buffer = w.buffer[:0]
	return nil
}

type sparseVectorEntry struct {
	index uint32
	value float32
}

type sparseVectorHeap []sparseVectorEntry

func (h sparseVectorHeap) Len() int {
	return len(h)
}

func (h sparseVectorHeap) Less(i, j int) bool {
	if h[i].value == h[j].value {
		return h[i].index > h[j].index
	}
	return h[i].value < h[j].value
}

func (h sparseVectorHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *sparseVectorHeap) Push(value any) {
	*h = append(*h, value.(sparseVectorEntry))
}

func (h *sparseVectorHeap) Pop() any {
	old := *h
	n := len(old)
	value := old[n-1]
	*h = old[:n-1]
	return value
}

type sparseVectorPruner struct {
	entries sparseVectorHeap
	maxNNZ  int
}

func newSparseVectorPruner(maxNNZ int) *sparseVectorPruner {
	return &sparseVectorPruner{
		entries: make(sparseVectorHeap, 0, maxNNZ),
		maxNNZ:  maxNNZ,
	}
}

func (p *sparseVectorPruner) Add(index uint32, value float32) {
	if p.maxNNZ <= 0 {
		return
	}
	entry := sparseVectorEntry{index: index, value: value}
	if len(p.entries) < p.maxNNZ {
		heap.Push(&p.entries, entry)
		return
	}
	if entry.value > p.entries[0].value || entry.value == p.entries[0].value && entry.index < p.entries[0].index {
		p.entries[0] = entry
		heap.Fix(&p.entries, 0)
	}
}

func (p *sparseVectorPruner) Result() ([]uint32, []float32) {
	entries := append([]sparseVectorEntry(nil), p.entries...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].index < entries[j].index
	})
	indices := make([]uint32, len(entries))
	values := make([]float32, len(entries))
	for i, entry := range entries {
		indices[i] = entry.index
		values[i] = entry.value
	}
	return indices, values
}

func newSparseVector[T ~int32](ids []T, idf []float32, offset uint32) vectors.Vector {
	return appendSparseVector(vectors.Vector{}, ids, idf, offset)
}

func appendSparseVector[T ~int32](vector vectors.Vector, ids []T, idf []float32, offset uint32) vectors.Vector {
	pruner := newSparseVectorPruner(maxSparseVectorNNZ)
	for i, index := range vector.Indices {
		pruner.Add(index, vector.Values[i])
	}
	for _, id := range ids {
		if id < 0 || int(id) >= len(idf) || idf[id] <= 0 {
			continue
		}
		pruner.Add(offset+uint32(id), float32(math.Sqrt(float64(idf[id]))))
	}
	vector.Indices, vector.Values = pruner.Result()
	return vector
}
