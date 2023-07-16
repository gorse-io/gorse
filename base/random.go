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
	"math/rand"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
)

// RandomGenerator is the random generator for gorse.
type RandomGenerator struct {
	*rand.Rand
}

// NewRandomGenerator creates a RandomGenerator.
func NewRandomGenerator(seed int64) RandomGenerator {
	return RandomGenerator{rand.New(rand.NewSource(int64(seed)))}
}

// UniformVector makes a vec filled with uniform random floats,
func (rng RandomGenerator) UniformVector(size int, low, high float32) []float32 {
	ret := make([]float32, size)
	scale := high - low
	for i := 0; i < len(ret); i++ {
		ret[i] = rng.Float32()*scale + low
	}
	return ret
}

// NewNormalVector makes a vec filled with normal random floats.
func (rng RandomGenerator) NewNormalVector(size int, mean, stdDev float32) []float32 {
	ret := make([]float32, size)
	for i := 0; i < len(ret); i++ {
		ret[i] = float32(rng.NormFloat64())*stdDev + mean
	}
	return ret
}

// NormalMatrix makes a matrix filled with normal random floats.
func (rng RandomGenerator) NormalMatrix(row, col int, mean, stdDev float32) [][]float32 {
	ret := make([][]float32, row)
	for i := range ret {
		ret[i] = rng.NewNormalVector(col, mean, stdDev)
	}
	return ret
}

func (rng RandomGenerator) NormalVector(size int, mean, stdDev float32) []float32 {
	ret := make([]float32, size)
	for i := 0; i < len(ret); i++ {
		ret[i] = float32(rng.NormFloat64())*stdDev + mean
	}
	return ret
}

// UniformMatrix makes a matrix filled with uniform random floats.
func (rng RandomGenerator) UniformMatrix(row, col int, low, high float32) [][]float32 {
	ret := make([][]float32, row)
	for i := range ret {
		ret[i] = rng.UniformVector(col, low, high)
	}
	return ret
}

// NormalVector64 makes a vec filled with normal random floats.
func (rng RandomGenerator) NormalVector64(size int, mean, stdDev float64) []float64 {
	ret := make([]float64, size)
	for i := 0; i < len(ret); i++ {
		ret[i] = rng.NormFloat64()*stdDev + mean
	}
	return ret
}

// Sample n values between low and high, but not in exclude.
func (rng RandomGenerator) Sample(low, high, n int, exclude ...mapset.Set[int]) []int {
	intervalLength := high - low
	excludeSet := mapset.NewSet[int]()
	for _, set := range exclude {
		excludeSet = excludeSet.Union(set)
	}
	sampled := make([]int, 0, n)
	if n >= intervalLength-excludeSet.Cardinality() {
		for i := low; i < high; i++ {
			if !excludeSet.Contains(i) {
				sampled = append(sampled, i)
				excludeSet.Add(i)
			}
		}
	} else {
		for len(sampled) < n {
			v := rng.Intn(intervalLength) + low
			if !excludeSet.Contains(v) {
				sampled = append(sampled, v)
				excludeSet.Add(v)
			}
		}
	}
	return sampled
}

// SampleInt32 n 32bit values between low and high, but not in exclude.
func (rng RandomGenerator) SampleInt32(low, high int32, n int, exclude ...mapset.Set[int32]) []int32 {
	intervalLength := high - low
	excludeSet := mapset.NewSet[int32]()
	for _, set := range exclude {
		excludeSet = excludeSet.Union(set)
	}
	sampled := make([]int32, 0, n)
	if n >= int(intervalLength)-excludeSet.Cardinality() {
		for i := low; i < high; i++ {
			if !excludeSet.Contains(i) {
				sampled = append(sampled, i)
				excludeSet.Add(i)
			}
		}
	} else {
		for len(sampled) < n {
			v := rng.Int31n(intervalLength) + low
			if !excludeSet.Contains(v) {
				sampled = append(sampled, v)
				excludeSet.Add(v)
			}
		}
	}
	return sampled
}

// lockedSource allows a random number generator to be used by multiple goroutines concurrently.
// The code is very similar to math/rand.lockedSource, which is unfortunately not exposed.
type lockedSource struct {
	mut sync.Mutex
	src rand.Source
}

// NewRand returns a rand.Rand that is threadsafe.
func NewRand(seed int64) *rand.Rand {
	return rand.New(&lockedSource{src: rand.NewSource(seed)})
}

func (r *lockedSource) Int63() (n int64) {
	r.mut.Lock()
	n = r.src.Int63()
	r.mut.Unlock()
	return
}

func (r *lockedSource) Seed(seed int64) {
	r.mut.Lock()
	r.src.Seed(seed)
	r.mut.Unlock()
}
