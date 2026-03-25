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

package ctr

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinMaxScaler(t *testing.T) {
	t.Run("fit and transform", func(t *testing.T) {
		scaler := NewMinMaxScaler()
		values := []float32{1, 2, 3, 4, 5}
		scaler.Fit(values)

		assert.Equal(t, float32(1), scaler.Min)
		assert.Equal(t, float32(5), scaler.Max)

		// Test transform
		assert.Equal(t, float32(0), scaler.Transform(1))
		assert.Equal(t, float32(0.25), scaler.Transform(2))
		assert.Equal(t, float32(0.5), scaler.Transform(3))
		assert.Equal(t, float32(0.75), scaler.Transform(4))
		assert.Equal(t, float32(1), scaler.Transform(5))
	})

	t.Run("constant values", func(t *testing.T) {
		scaler := NewMinMaxScaler()
		values := []float32{5, 5, 5}
		scaler.Fit(values)

		assert.Equal(t, float32(5), scaler.Min)
		assert.Equal(t, float32(5), scaler.Max)

		// When range is 0, should return 0.5
		assert.Equal(t, float32(0.5), scaler.Transform(5))
	})

	t.Run("negative values", func(t *testing.T) {
		scaler := NewMinMaxScaler()
		values := []float32{-5, -3, 0, 3, 5}
		scaler.Fit(values)

		assert.Equal(t, float32(-5), scaler.Min)
		assert.Equal(t, float32(5), scaler.Max)

		assert.Equal(t, float32(0), scaler.Transform(-5))
		assert.Equal(t, float32(0.3), scaler.Transform(-2))
		assert.Equal(t, float32(0.5), scaler.Transform(0))
		assert.Equal(t, float32(1), scaler.Transform(5))
	})

	t.Run("marshal and unmarshal", func(t *testing.T) {
		scaler := NewMinMaxScaler()
		values := []float32{10, 20, 30}
		scaler.Fit(values)

		// Marshal
		var buf bytes.Buffer
		err := scaler.Marshal(&buf)
		assert.NoError(t, err)

		// Unmarshal
		scaler2 := NewMinMaxScaler()
		err = scaler2.Unmarshal(&buf)
		assert.NoError(t, err)

		assert.Equal(t, scaler.Min, scaler2.Min)
		assert.Equal(t, scaler.Max, scaler2.Max)
	})
}
