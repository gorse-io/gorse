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

	"github.com/stretchr/testify/assert"
)

func TestNoDatabase(t *testing.T) {
	ctx := t.Context()
	var database NoDatabase
	err := database.Init()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Close()
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.ListCollections(ctx)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.AddCollection(ctx, "test", 2, Cosine)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.DeleteCollection(ctx, "test")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.AddVectors(ctx, "test", []Vector{{Id: "a", Vector: []float32{1, 0}}})
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.QueryVectors(ctx, "test", []float32{1, 0}, nil, 10)
	assert.ErrorIs(t, err, ErrNoDatabase)
}
