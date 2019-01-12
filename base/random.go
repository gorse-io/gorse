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

// NewUniformVectorInt makes a vec filled with uniform random integers.
func (rng RandomGenerator) NewUniformVectorInt(size, low, high int) []int {
	ret := make([]int, size)
	scale := high - low
	for i := 0; i < len(ret); i++ {
		ret[i] = rng.Intn(scale) + low
	}
	return ret
}

// NewUniformVector makes a vec filled with uniform random floats,
func (rng RandomGenerator) NewUniformVector(size int, low, high float64) []float64 {
	ret := make([]float64, size)
	scale := high - low
	for i := 0; i < len(ret); i++ {
		ret[i] = rng.Float64()*scale + low
	}
	return ret
}

// NewNormalVector makes a vec filled with normal random floats.
func (rng RandomGenerator) NewNormalVector(size int, mean, stdDev float64) []float64 {
	ret := make([]float64, size)
	for i := 0; i < len(ret); i++ {
		ret[i] = rng.NormFloat64()*stdDev + mean
	}
	return ret
}

// NewNormalMatrix makes a matrix filled with normal random floats.
func (rng RandomGenerator) NewNormalMatrix(row, col int, mean, stdDev float64) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = rng.NewNormalVector(col, mean, stdDev)
	}
	return ret
}

// NewUniformMatrix makes a matrix filled with uniform random floats.
func (rng RandomGenerator) NewUniformMatrix(row, col int, low, high float64) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = rng.NewUniformVector(col, low, high)
	}
	return ret
}
