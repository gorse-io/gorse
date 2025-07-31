// Copyright 2025 gorse Project Authors
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

package logics

import (
	"time"

	"github.com/gorse-io/gorse/base/log"
	"github.com/gorse-io/gorse/common/ann"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type MatrixFactorization struct {
	timestamp time.Time
	items     []*data.Item
	index     *ann.HNSW[[]float32]
	dimension int
}

func NewMatrixFactorization(timestamp time.Time) *MatrixFactorization {
	return &MatrixFactorization{
		timestamp: timestamp,
		items:     make([]*data.Item, 0),
		index:     ann.NewHNSW[[]float32](floats.Dot),
	}
}

func (mf *MatrixFactorization) Add(item *data.Item, v []float32) {
	// Check dimension
	if mf.dimension == 0 {
		mf.dimension = len(v)
	} else if mf.dimension != len(v) {
		log.Logger().Error("dimension mismatch", zap.Int("dimension", len(v)))
		return
	}
	// Push item
	mf.items = append(mf.items, item)
	_ = mf.index.Add(v)
}

func (mf *MatrixFactorization) Search(v []float32, n int) []cache.Score {
	scores := mf.index.SearchVector(v, n, false)
	return lo.Map(scores, func(v lo.Tuple2[int, float32], _ int) cache.Score {
		return cache.Score{
			Id:         mf.items[v.A].ItemId,
			Score:      -float64(v.B),
			Categories: mf.items[v.A].Categories,
			Timestamp:  mf.timestamp,
		}
	})
}
