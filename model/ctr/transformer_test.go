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
	"github.com/chewxy/math32"
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

		// When range is 0, should return 1
		assert.Equal(t, float32(1), scaler.Transform(5))
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

func TestRobustScaler(t *testing.T) {
	t.Run("fit and transform", func(t *testing.T) {
		scaler := NewRobustScaler()
		// Values: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
		values := []float32{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
		scaler.Fit(values)

		// Sorted: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
		// Median = (5 + 6) / 2 = 5.5
		// Q1 (index 2) = 3
		// Q3 (index 7) = 8
		// IQR = 8 - 3 = 5
		assert.Equal(t, float32(5.5), scaler.Median)
		assert.Equal(t, float32(3), scaler.Q1)
		assert.Equal(t, float32(8), scaler.Q3)
		assert.Equal(t, float32(5), scaler.IQR)

		// Transform: (X - median) / IQR
		assert.InDelta(t, -0.9, scaler.Transform(1), 0.01)
		assert.InDelta(t, 0.0, scaler.Transform(5.5), 0.01)
		assert.InDelta(t, 0.9, scaler.Transform(10), 0.01)
	})

	t.Run("robust to outliers", func(t *testing.T) {
		scaler := NewRobustScaler()
		// Values with outliers: 1, 2, 3, 4, 5, 100, 1000
		values := []float32{1, 2, 3, 4, 5, 100, 1000}
		scaler.Fit(values)

		// Sorted: 1, 2, 3, 4, 5, 100, 1000
		// n = 7, median index = 3, median = 4
		// Q1 index = 1, Q1 = 2
		// Q3 index = 5, Q3 = 100
		// IQR = 100 - 2 = 98
		assert.Equal(t, float32(4), scaler.Median)
		assert.Equal(t, float32(2), scaler.Q1)
		assert.Equal(t, float32(100), scaler.Q3)
		assert.Equal(t, float32(98), scaler.IQR)
	})

	t.Run("constant values", func(t *testing.T) {
		scaler := NewRobustScaler()
		values := []float32{5, 5, 5, 5, 5}
		scaler.Fit(values)

		assert.Equal(t, float32(5), scaler.Median)
		assert.Equal(t, float32(5), scaler.Q1)
		assert.Equal(t, float32(5), scaler.Q3)
		assert.Equal(t, float32(1.0), scaler.IQR) // Avoid division by zero
	})

	t.Run("marshal and unmarshal", func(t *testing.T) {
		scaler := NewRobustScaler()
		values := []float32{1, 5, 10, 15, 20}
		scaler.Fit(values)

		// Marshal
		var buf bytes.Buffer
		err := scaler.Marshal(&buf)
		assert.NoError(t, err)

		// Unmarshal
		scaler2 := NewRobustScaler()
		err = scaler2.Unmarshal(&buf)
		assert.NoError(t, err)

		assert.Equal(t, scaler.Median, scaler2.Median)
		assert.Equal(t, scaler.Q1, scaler2.Q1)
		assert.Equal(t, scaler.Q3, scaler2.Q3)
		assert.Equal(t, scaler.IQR, scaler2.IQR)

		// Verify transform gives same result
		assert.Equal(t, scaler.Transform(7), scaler2.Transform(7))
	})
}

func TestAutoScaler(t *testing.T) {
	t.Run("non-negative values use log1p + MinMax", func(t *testing.T) {
		scaler := NewAutoScaler()
		values := []float32{0, 10, 100, 1000}
		scaler.Fit(values)

		assert.True(t, scaler.UseLog)
		assert.False(t, scaler.HasRobust)

		// Verify log transformation
		// log1p(0) = 0, log1p(1000) ≈ 6.909
		minLog := math32.Log1p(0)
		maxLog := math32.Log1p(1000)

		assert.InDelta(t, 0.0, scaler.Transform(0), 0.001)
		assert.InDelta(t, 1.0, scaler.Transform(1000), 0.001)

		// Verify intermediate values
		expectedVal := (math32.Log1p(100) - minLog) / (maxLog - minLog)
		assert.InDelta(t, expectedVal, scaler.Transform(100), 0.001)
	})

	t.Run("negative values use RobustScaler", func(t *testing.T) {
		scaler := NewAutoScaler()
		values := []float32{-10, -5, 0, 5, 10}
		scaler.Fit(values)

		assert.False(t, scaler.UseLog)
		assert.True(t, scaler.HasRobust)

		// Should use RobustScaler
		// median = 0, Q1 = -5, Q3 = 5, IQR = 10
		assert.Equal(t, float32(0), scaler.Robust.Median)
		assert.Equal(t, float32(10), scaler.Robust.IQR)

		assert.InDelta(t, 0.0, scaler.Transform(0), 0.001)
		assert.InDelta(t, 1.0, scaler.Transform(10), 0.001)
		assert.InDelta(t, -1.0, scaler.Transform(-10), 0.001)
	})

	t.Run("mixed values with single negative", func(t *testing.T) {
		scaler := NewAutoScaler()
		values := []float32{-1, 10, 100, 1000}
		scaler.Fit(values)

		assert.False(t, scaler.UseLog)
		assert.True(t, scaler.HasRobust)
	})

	t.Run("marshal and unmarshal with log", func(t *testing.T) {
		scaler := NewAutoScaler()
		values := []float32{0, 100, 1000}
		scaler.Fit(values)

		assert.True(t, scaler.UseLog)

		// Marshal
		var buf bytes.Buffer
		err := scaler.Marshal(&buf)
		assert.NoError(t, err)

		// Unmarshal
		scaler2 := NewAutoScaler()
		err = scaler2.Unmarshal(&buf)
		assert.NoError(t, err)

		assert.Equal(t, scaler.UseLog, scaler2.UseLog)
		assert.Equal(t, scaler.HasRobust, scaler2.HasRobust)
		assert.Equal(t, scaler.MinMax.Min, scaler2.MinMax.Min)
		assert.Equal(t, scaler.MinMax.Max, scaler2.MinMax.Max)

		// Verify transform gives same result
		assert.Equal(t, scaler.Transform(500), scaler2.Transform(500))
	})

	t.Run("marshal and unmarshal with robust", func(t *testing.T) {
		scaler := NewAutoScaler()
		values := []float32{-100, -50, 0, 50, 100}
		scaler.Fit(values)

		assert.True(t, scaler.HasRobust)

		// Marshal
		var buf bytes.Buffer
		err := scaler.Marshal(&buf)
		assert.NoError(t, err)

		// Unmarshal
		scaler2 := NewAutoScaler()
		err = scaler2.Unmarshal(&buf)
		assert.NoError(t, err)

		assert.Equal(t, scaler.UseLog, scaler2.UseLog)
		assert.Equal(t, scaler.HasRobust, scaler2.HasRobust)
		assert.Equal(t, scaler.Robust.Median, scaler2.Robust.Median)
		assert.Equal(t, scaler.Robust.IQR, scaler2.Robust.IQR)

		// Verify transform gives same result
		assert.Equal(t, scaler.Transform(25), scaler2.Transform(25))
	})
}
