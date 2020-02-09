package base

import (
	"math"
)

// FuncSimilarity computes the similarity between a pair of vectors.
type FuncSimilarity func(a, b *MarginalSubSet) float64

// CosineSimilarity computes the cosine similarity between a pair of vectors.
func CosineSimilarity(a, b *MarginalSubSet) float64 {
	m, n, l := .0, .0, .0
	a.ForIntersection(b, func(_ string, a, b float64) {
		m += a * a
		n += b * b
		l += a * b
	})
	return l / (math.Sqrt(m) * math.Sqrt(n))
}

// MSDSimilarity computes the Mean Squared Difference similarity between a pair of vectors.
func MSDSimilarity(a, b *MarginalSubSet) float64 {
	count, sum := 0.0, 0.0
	a.ForIntersection(b, func(_ string, a, b float64) {
		sum += (a - b) * (a - b)
		count += 1
	})
	return 1.0 / (sum/count + 1)
}

// PearsonSimilarity computes the absolute Pearson correlation coefficient between a pair of vectors.
func PearsonSimilarity(a, b *MarginalSubSet) float64 {
	// Mean of a
	meanA := a.Mean()
	// Mean of b
	meanB := b.Mean()
	// Mean-centered cosine
	m, n, l := .0, .0, .0
	a.ForIntersection(b, func(_ string, a, b float64) {
		ratingA := a - meanA
		ratingB := b - meanB
		m += ratingA * ratingA
		n += ratingB * ratingB
		l += ratingA * ratingB
	})
	return math.Abs(l) / (math.Sqrt(m) * math.Sqrt(n))
}

// ImplicitSimilarity computes similarity between two vectors with implicit feedback.
func ImplicitSimilarity(a, b *MarginalSubSet) float64 {
	intersect := 0.0
	a.ForIntersection(b, func(_ string, a, b float64) {
		intersect++
	})
	return intersect / (math.Sqrt(float64(a.Len())) * math.Sqrt(float64(b.Len())))
}
