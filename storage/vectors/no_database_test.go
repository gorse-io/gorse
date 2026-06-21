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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var _ Database = NoDatabase{}

func TestNoDatabase(t *testing.T) {
	ctx := t.Context()
	var database NoDatabase

	t.Run("lifecycle", func(t *testing.T) {
		assert.ErrorIs(t, database.Init(), ErrNoDatabase)
		assert.ErrorIs(t, database.Optimize(), ErrNoDatabase)
		assert.ErrorIs(t, database.Close(), ErrNoDatabase)
	})

	t.Run("collections", func(t *testing.T) {
		collections, err := database.ListCollections(ctx)
		assert.ErrorIs(t, err, ErrNoDatabase)
		assert.Nil(t, collections)

		assert.ErrorIs(t, database.AddCollection(ctx, "test", 4, Cosine, DefaultVectorConfig()), ErrNoDatabase)
		assert.ErrorIs(t, database.AddCollection(ctx, "test", 4, Dot, DefaultVectorConfig()), ErrNoDatabase)
		assert.ErrorIs(t, database.DeleteCollection(ctx, "test"), ErrNoDatabase)
		assert.ErrorIs(t, database.DeleteCollection(ctx, "missing"), ErrNoDatabase)
	})

	t.Run("vectors", func(t *testing.T) {
		assert.ErrorIs(t, database.AddVectors(ctx, "test", []Vector{
			{Id: "a", Vector: []float32{1, 0, 0, 0}, Categories: []string{"cat-a"}},
		}), ErrNoDatabase)
		assert.ErrorIs(t, database.AddVectors(ctx, "test", nil), ErrNoDatabase)
		assert.ErrorIs(t, database.DeleteVectors(ctx, "test", time.Now()), ErrNoDatabase)
		assert.ErrorIs(t, database.DeleteVectors(ctx, "missing", time.Time{}), ErrNoDatabase)

		results, err := database.QueryVectors(ctx, "test", []float32{1, 0, 0, 0}, []string{"cat-a"}, 10)
		assert.ErrorIs(t, err, ErrNoDatabase)
		assert.Nil(t, results)

		results, err = database.QueryVectors(ctx, "missing", nil, nil, 0)
		assert.ErrorIs(t, err, ErrNoDatabase)
		assert.Nil(t, results)
	})
}
