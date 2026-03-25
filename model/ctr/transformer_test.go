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

	"github.com/chewxy/math32"
	"github.com/stretchr/testify/assert"
)

func TestMinMaxScaler(t *testing.T) {
	t.Run("fit and transform without log", func(t *testing.T) {
		scaler := NewMinMaxScaler()
		values := []float32{-5, -3, 0, 3, 5}
		scaler.Fit(values)

		assert.Equal(t, float32(-5), scaler.Min)
		assert.Equal(t, float32(5), scaler.Max)
		assert.False(t, scaler.UseLog)

		// Test transform
		assert.Equal(t, float32(0), scaler.Transform(-5))
		assert.Equal(t, float32(0.3), scaler.Transform(-2))
		assert.Equal(t, float32(0.5), scaler.Transform(0))
		assert.Equal(t, float32(0.75), scaler.Transform(2.5))
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

	t.Run("non-negative values use log", func(t *testing.T) {
		scaler := NewMinMaxScaler()
		values := []float32{0, 10, 100, 1000}
		scaler.Fit(values)

		assert.Equal(t, float32(0), scaler.Min)
		assert.Equal(t, float32(1000), scaler.Max)
		assert.True(t, scaler.UseLog)

		// Verify log transformation
		minLog := math32.Log1p(0)
		maxLog := math32.Log1p(1000)
		rangeLog := maxLog - minLog

		assert.InDelta(t, 0.0, scaler.Transform(0), 0.001)
		assert.InDelta(t, math32.Log1p(10)/rangeLog, scaler.Transform(10), 0.001)
		assert.InDelta(t, math32.Log1p(100)/rangeLog, scaler.Transform(100), 0.001)
		assert.InDelta(t, 1.0, scaler.Transform(1000), 0.001)
	})

	t.Run("marshal and unmarshal without log", func(t *testing.T) {
		scaler := NewMinMaxScaler()
		values := []float32{-10, -5, 0, 5, 10}
		scaler.Fit(values)

		assert.False(t, scaler.UseLog)

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
		assert.Equal(t, scaler.UseLog, scaler2.UseLog)
	})

	t.Run("marshal and unmarshal with log", func(t *testing.T) {
		scaler := NewMinMaxScaler()
		values := []float32{0, 100, 1000}
		scaler.Fit(values)

		assert.True(t, scaler.UseLog)

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
		assert.Equal(t, scaler.UseLog, scaler2.UseLog)

		// Verify transform gives same result
		assert.Equal(t, scaler.Transform(500), scaler2.Transform(500))
	})
}
