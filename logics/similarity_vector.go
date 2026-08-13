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
	"context"
	"math"
	"sync"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/juju/errors"
	"go.uber.org/zap"
)

const defaultSimilarityVectorBatchSize = 1024

// similarityVectorWriter owns collection validation and batched vector writes
// for item-to-item and user-to-user recommenders.
type similarityVectorWriter struct {
	ctx          context.Context
	client       vectors.Database
	collection   string
	distance     vectors.Distance
	vectorConfig vectors.VectorConfig
	timestamp    time.Time
	batchSize    int
	sparse       bool

	mu               sync.Mutex
	dimension        int
	dimensionKnown   bool
	collectionExists bool
	buffer           []vectors.Vector
	err              error
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
) *similarityVectorWriter {
	if ctx == nil {
		ctx = context.Background()
	}
	if batchSize <= 0 {
		batchSize = defaultSimilarityVectorBatchSize
	}
	writer := &similarityVectorWriter{
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
		writer.dimensionKnown = true
		writer.vectorConfig = vectors.VectorConfig{}
	}
	return writer
}

func (w *similarityVectorWriter) Push(vector vectors.Vector) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return false
	}
	if w.sparse {
		if len(vector.Indices) == 0 || len(vector.Indices) != len(vector.Values) {
			return false
		}
	} else {
		if len(vector.Indices) != 0 || len(vector.Values) == 0 {
			return false
		}
		if !w.dimensionKnown {
			w.dimension = len(vector.Values)
			w.dimensionKnown = true
		} else if w.dimension != len(vector.Values) {
			log.Logger().Error("invalid similarity vector dimension",
				zap.String("collection", w.collection),
				zap.String("id", vector.Id),
				zap.Int("dimension", len(vector.Values)),
				zap.Int("expected_dimension", w.dimension))
			return false
		}
	}
	if err := w.ensureCollectionLocked(); err != nil {
		w.err = err
		return false
	}
	vector.Timestamp = w.timestamp
	w.buffer = append(w.buffer, vector)
	if len(w.buffer) >= w.batchSize {
		w.err = w.flushLocked()
	}
	return w.err == nil
}

func (w *similarityVectorWriter) Finish() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	if w.dimensionKnown {
		if err := w.ensureCollectionLocked(); err != nil {
			w.err = err
			return err
		}
	}
	if err := w.flushLocked(); err != nil {
		w.err = err
		return err
	}
	if !w.collectionExists {
		return nil
	}
	if err := w.client.DeleteVectors(w.ctx, w.collection, w.timestamp); err != nil {
		w.err = errors.Trace(err)
		return w.err
	}
	return nil
}

func (w *similarityVectorWriter) Dimension() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dimension
}

func (w *similarityVectorWriter) ensureCollectionLocked() error {
	if w.collectionExists {
		return nil
	}
	if w.client == nil {
		return errors.NotAssignedf("vector database")
	}
	info, err := w.client.DescribeCollection(w.ctx, w.collection)
	if errors.Is(err, errors.NotFound) {
		info = nil
	} else if err != nil {
		return errors.Trace(err)
	}
	if info != nil && (info.Dimension != w.dimension || info.Distance != w.distance || info.Type != w.vectorConfig.Type || info.Bits != w.vectorConfig.Bits) {
		log.Logger().Warn("recreating similarity vector collection",
			zap.String("collection", w.collection),
			zap.Int("dimension", w.dimension),
			zap.Int("distance", int(w.distance)))
		if err = w.client.DeleteCollection(w.ctx, w.collection); err != nil && !errors.Is(err, errors.NotFound) {
			return errors.Trace(err)
		}
		info = nil
	}
	if info == nil {
		if err = w.client.AddCollection(w.ctx, w.collection, w.dimension, w.distance, w.vectorConfig); err != nil {
			return errors.Trace(err)
		}
	}
	w.collectionExists = true
	return nil
}

func (w *similarityVectorWriter) flushLocked() error {
	if len(w.buffer) == 0 {
		return nil
	}
	if err := w.client.AddVectors(w.ctx, w.collection, w.buffer); err != nil {
		return errors.Trace(err)
	}
	w.buffer = w.buffer[:0]
	return nil
}

func newSparseVector[T ~int32](ids []T, idf []float32, offset uint32) vectors.Vector {
	vector := vectors.Vector{
		Indices: make([]uint32, 0, len(ids)),
		Values:  make([]float32, 0, len(ids)),
	}
	for _, id := range ids {
		if id < 0 || int(id) >= len(idf) || idf[id] <= 0 {
			continue
		}
		vector.Indices = append(vector.Indices, offset+uint32(id))
		vector.Values = append(vector.Values, float32(math.Sqrt(float64(idf[id]))))
	}
	return vector
}

func appendSparseVector(dst, src vectors.Vector) vectors.Vector {
	dst.Indices = append(dst.Indices, src.Indices...)
	dst.Values = append(dst.Values, src.Values...)
	return dst
}
