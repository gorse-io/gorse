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
	"github.com/gorse-io/gorse/storage"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var _ Database = NoDatabase{}

func TestNoDatabase(t *testing.T) {
	ctx := t.Context()
	var database NoDatabase

	t.Run("lifecycle", func(t *testing.T) {
		assert.ErrorIs(t, database.Init(), storage.ErrNoDatabase)
		assert.NoError(t, database.Optimize(ctx, "test"))
		assert.ErrorIs(t, database.Close(), storage.ErrNoDatabase)
	})

	t.Run("collections", func(t *testing.T) {
		collections, err := database.ListCollections(ctx)
		assert.ErrorIs(t, err, storage.ErrNoDatabase)
		assert.Nil(t, collections)

		info, err := database.DescribeCollection(ctx, "test")
		assert.ErrorIs(t, err, storage.ErrNoDatabase)
		assert.Nil(t, info)

		assert.ErrorIs(t, database.AddCollection(ctx, "test", 4, Cosine, VectorConfig{}), storage.ErrNoDatabase)
		assert.ErrorIs(t, database.AddCollection(ctx, "test", 4, Dot, VectorConfig{}), storage.ErrNoDatabase)
		assert.ErrorIs(t, database.DeleteCollection(ctx, "test"), storage.ErrNoDatabase)
		assert.ErrorIs(t, database.DeleteCollection(ctx, "missing"), storage.ErrNoDatabase)
	})

	t.Run("vectors", func(t *testing.T) {
		count, err := database.CountVectors(ctx, "test")
		assert.ErrorIs(t, err, storage.ErrNoDatabase)
		assert.Zero(t, count)

		assert.ErrorIs(t, database.AddVectors(ctx, "test", []Vector{
			{Id: "a", Values: []float32{1, 0, 0, 0}, Categories: []string{"cat-a"}},
		}), storage.ErrNoDatabase)
		assert.ErrorIs(t, database.AddVectors(ctx, "test", nil), storage.ErrNoDatabase)
		vectors, err := database.GetVectors(ctx, "test", []string{"a"})
		assert.ErrorIs(t, err, storage.ErrNoDatabase)
		assert.Nil(t, vectors)
		assert.ErrorIs(t, database.DeleteVectors(ctx, "test", time.Now()), storage.ErrNoDatabase)
		assert.ErrorIs(t, database.DeleteVectors(ctx, "missing", time.Time{}), storage.ErrNoDatabase)

		results, err := database.QueryVectors(ctx, "test", Vector{Values: []float32{1, 0, 0, 0}}, []string{"cat-a"}, 10)
		assert.ErrorIs(t, err, storage.ErrNoDatabase)
		assert.Nil(t, results)

		results, err = database.QueryVectors(ctx, "missing", Vector{}, nil, 0)
		assert.ErrorIs(t, err, storage.ErrNoDatabase)
		assert.Nil(t, results)
	})
}
