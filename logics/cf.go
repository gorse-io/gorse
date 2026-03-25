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
	"io"
	"sync"
	"time"

	"github.com/gorse-io/gorse/common/ann"
	"github.com/gorse-io/gorse/common/encoding"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

func distance(a, b []float32) float32 {
	return -floats.Dot(a, b)
}

type MatrixFactorizationItems struct {
	timestamp time.Time
	items     []string
	itemsLock sync.Mutex
	index     *ann.HNSW[[]float32]
	dimension int
}

func NewMatrixFactorizationItems(timestamp time.Time) *MatrixFactorizationItems {
	return &MatrixFactorizationItems{
		timestamp: timestamp,
		items:     make([]string, 0),
		index:     ann.NewHNSW(distance),
	}
}

func (items *MatrixFactorizationItems) Add(itemId string, v []float32) {
	// Check dimension
	items.itemsLock.Lock()
	if items.dimension == 0 {
		items.dimension = len(v)
	} else if items.dimension != len(v) {
		log.Logger().Error("dimension mismatch", zap.Int("dimension", len(v)))
		return
	}
	// Push item
	items.items = append(items.items, "")
	items.itemsLock.Unlock()
	j := items.index.Add(v)
	items.itemsLock.Lock()
	items.items[j] = itemId
	items.itemsLock.Unlock()
}

func (items *MatrixFactorizationItems) Search(v []float32, n int) []cache.Score {
	scores := items.index.SearchVector(v, n, false)
	return lo.Map(scores, func(v lo.Tuple2[int, float32], _ int) cache.Score {
		return cache.Score{
			Id:        items.items[v.A],
			Score:     -float64(v.B),
			Timestamp: items.timestamp,
		}
	})
}

func (items *MatrixFactorizationItems) Marshal(w io.Writer) error {
	if err := encoding.WriteGob(w, items.timestamp); err != nil {
		return errors.WithStack(err)
	}
	if err := encoding.WriteGob(w, items.dimension); err != nil {
		return errors.WithStack(err)
	}
	if err := items.index.Marshal(w); err != nil {
		return errors.WithStack(err)
	}
	numItems := int64(len(items.items))
	if err := encoding.WriteGob(w, numItems); err != nil {
		return errors.WithStack(err)
	}
	for _, item := range items.items {
		if err := encoding.WriteGob(w, item); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (items *MatrixFactorizationItems) Unmarshal(r io.Reader) error {
	if err := encoding.ReadGob(r, &items.timestamp); err != nil {
		return errors.WithStack(err)
	}
	if err := encoding.ReadGob(r, &items.dimension); err != nil {
		return errors.WithStack(err)
	}
	if err := items.index.Unmarshal(r); err != nil {
		return errors.WithStack(err)
	}
	var numItems int64
	if err := encoding.ReadGob(r, &numItems); err != nil {
		return errors.WithStack(err)
	}
	items.items = make([]string, numItems)
	for i := int64(0); i < numItems; i++ {
		if err := encoding.ReadGob(r, &items.items[i]); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

type MatrixFactorizationUsers struct {
	mu         sync.RWMutex
	embeddings map[string][]float32
}

func NewMatrixFactorizationUsers() *MatrixFactorizationUsers {
	return &MatrixFactorizationUsers{
		embeddings: make(map[string][]float32),
	}
}

func (users *MatrixFactorizationUsers) Add(userId string, v []float32) {
	users.mu.Lock()
	defer users.mu.Unlock()
	users.embeddings[userId] = v
}

func (users *MatrixFactorizationUsers) Get(userId string) ([]float32, bool) {
	users.mu.RLock()
	defer users.mu.RUnlock()
	v, ok := users.embeddings[userId]
	return v, ok
}

func (users *MatrixFactorizationUsers) Marshal(w io.Writer) error {
	users.mu.RLock()
	defer users.mu.RUnlock()
	numUsers := int64(len(users.embeddings))
	if err := encoding.WriteGob(w, numUsers); err != nil {
		return errors.WithStack(err)
	}
	for userId, embedding := range users.embeddings {
		if err := encoding.WriteString(w, userId); err != nil {
			return errors.WithStack(err)
		}
		if err := encoding.WriteSlice(w, embedding); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (users *MatrixFactorizationUsers) Unmarshal(r io.Reader) error {
	users.mu.Lock()
	defer users.mu.Unlock()
	var numUsers int64
	if err := encoding.ReadGob(r, &numUsers); err != nil {
		return errors.WithStack(err)
	}
	users.embeddings = make(map[string][]float32, numUsers)
	for i := int64(0); i < numUsers; i++ {
		userId, err := encoding.ReadString(r)
		if err != nil {
			return errors.WithStack(err)
		}
		var embedding []float32
		if err = encoding.ReadSlice(r, &embedding); err != nil {
			return errors.WithStack(err)
		}
		users.embeddings[userId] = embedding
	}
	return nil
}
