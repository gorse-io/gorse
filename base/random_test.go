// Copyright 2020 gorse Project Authors
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
package base

import (
	"testing"

	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

const randomEpsilon = 0.1

func TestRandomGenerator_MakeNormalMatrix(t *testing.T) {
	rng := NewRandomGenerator(0)
	vec := rng.NormalMatrix(1, 1000, 1, 2)[0]
	assert.False(t, math32.Abs(mean(vec)-1) > randomEpsilon)
	assert.False(t, math32.Abs(stdDev(vec)-2) > randomEpsilon)
}

func TestRandomGenerator_MakeUniformMatrix(t *testing.T) {
	rng := NewRandomGenerator(0)
	vec := rng.UniformMatrix(1, 1000, 1, 2)[0]
	assert.False(t, lo.Min(vec) < 1)
	assert.False(t, lo.Max(vec) > 2)
}

func TestRandomGenerator_Sample(t *testing.T) {
	excludeSet := mapset.NewSet(0, 1, 2, 3, 4)
	rng := NewRandomGenerator(0)
	for i := 1; i <= 10; i++ {
		sampled := rng.Sample(0, 10, i, excludeSet)
		for j := range sampled {
			assert.False(t, excludeSet.Contains(sampled[j]))
		}
	}
}

func TestRandomGenerator_SampleInt32(t *testing.T) {
	excludeSet := mapset.NewSet[int32](0, 1, 2, 3, 4)
	rng := NewRandomGenerator(0)
	for i := 1; i <= 10; i++ {
		sampled := rng.SampleInt32(0, 10, i, excludeSet)
		for j := range sampled {
			assert.False(t, excludeSet.Contains(sampled[j]))
		}
	}
}

// mean of a slice of 32-bit floats.
func mean(x []float32) float32 {
	return lo.Sum(x) / float32(len(x))
}

// stdDev returns the sample standard deviation.
func stdDev(x []float32) float32 {
	_, variance := meanVariance(x)
	return math32.Sqrt(variance)
}

// meanVariance computes the sample mean and unbiased variance, where the mean and variance are
//
//	\sum_i w_i * x_i / (sum_i w_i)
//	\sum_i w_i (x_i - mean)^2 / (sum_i w_i - 1)
//
// respectively.
// If weights is nil then all of the weights are 1. If weights is not nil, then
// len(x) must equal len(weights).
// When weights sum to 1 or less, a biased variance estimator should be used.
func meanVariance(x []float32) (m, variance float32) {
	// This uses the corrected two-pass algorithm (1.7), from "Algorithms for computing
	// the sample variance: Analysis and recommendations" by Chan, Tony F., Gene H. Golub,
	// and Randall J. LeVeque.

	// note that this will panic if the slice lengths do not match
	m = mean(x)
	var (
		ss           float32
		compensation float32
	)
	for _, v := range x {
		d := v - m
		ss += d * d
		compensation += d
	}
	variance = (ss - compensation*compensation/float32(len(x))) / float32(len(x)-1)
	return
}
