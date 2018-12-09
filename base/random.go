package base

import "math/rand"

// RandomGenerator is the random generator for gorse.
type RandomGenerator struct {
	*rand.Rand
}

// NewRandomGenerator creates a RandomGenerator.
func NewRandomGenerator(seed int64) RandomGenerator {
	return RandomGenerator{rand.New(rand.NewSource(int64(seed)))}
}

// MakeUniformVectorInt makes a vector filled with uniform random integers.
func (rng RandomGenerator) MakeUniformVectorInt(size, low, high int) []int {
	ret := make([]int, size)
	scale := high - low
	for i := 0; i < len(ret); i++ {
		ret[i] = rng.Intn(scale) + low
	}
	return ret
}

// MakeUniformVector makes a vector filled with uniform random floats,
func (rng RandomGenerator) MakeUniformVector(size int, low, high float64) []float64 {
	ret := make([]float64, size)
	scale := high - low
	for i := 0; i < len(ret); i++ {
		ret[i] = rng.Float64()*scale + low
	}
	return ret
}

// MakeNormalVector makes a vector filled with normal random floats.
func (rng RandomGenerator) MakeNormalVector(size int, mean, stdDev float64) []float64 {
	ret := make([]float64, size)
	for i := 0; i < len(ret); i++ {
		ret[i] = rng.NormFloat64()*stdDev + mean
	}
	return ret
}

// MakeNormalMatrix makes a matrix filled with normal random floats.
func (rng RandomGenerator) MakeNormalMatrix(row, col int, mean, stdDev float64) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = rng.MakeNormalVector(col, mean, stdDev)
	}
	return ret
}

// MakeUniformMatrix makes a matrix filled with uniform random floats.
func (rng RandomGenerator) MakeUniformMatrix(row, col int, low, high float64) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = rng.MakeUniformVector(col, low, high)
	}
	return ret
}
