package base

import (
	"gonum.org/v1/gonum/stat"
	"math"
)

// FuncSimilarity computes the similarity between a pair of vectors.
type FuncSimilarity func(a, b *SparseVector) float64

// Cosine computes the cosine similarity between a pair of vectors.
func CosineSimilarity(a, b *SparseVector) float64 {
	m, n, l := .0, .0, .0
	a.ForIntersection(b, func(index int, a, b float64) {
		m += a * a
		n += b * b
		l += a * b
	})
	return l / (math.Sqrt(m) * math.Sqrt(n))
}

// MSD computes the Mean Squared Difference similarity between a pair of vectors.
func MSDSimilarity(a, b *SparseVector) float64 {
	count, sum := 0.0, 0.0
	a.ForIntersection(b, func(index int, a, b float64) {
		sum += (a - b) * (a - b)
		count += 1
	})
	return 1.0 / (sum/count + 1)
}

// Pearson computes the absolute Pearson correlation coefficient between a pair of vectors.
func PearsonSimilarity(a, b *SparseVector) float64 {
	// Mean of a
	meanA := stat.Mean(a.Values, nil)
	// Mean of b
	meanB := stat.Mean(b.Values, nil)
	// Mean-centered cosine
	m, n, l := .0, .0, .0
	a.ForIntersection(b, func(index int, a, b float64) {
		ratingA := a - meanA
		ratingB := b - meanB
		m += ratingA * ratingA
		n += ratingB * ratingB
		l += ratingA * ratingB
	})
	return math.Abs(l) / (math.Sqrt(m) * math.Sqrt(n))
}
