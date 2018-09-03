package core

import "math"

type Sim func([]float64, []float64) float64

// Compute the cosine similarity between a pair of users (or items).
func Cosine(a []float64, b []float64) float64 {
	m, n, l := .0, .0, .0
	for i := range a {
		if !math.IsNaN(a[i]) && !math.IsNaN(b[i]) {
			m += a[i] * a[i]
			n += b[i] * b[i]
			l += a[i] * b[i]
		}
	}
	return l / (math.Sqrt(m) * math.Sqrt(n))
}

// Compute the Mean Squared Difference similarity between a pair of users (or items).
func MSD(a []float64, b []float64) float64 {
	count := 0.0
	sum := 0.0
	for i := range a {
		if !math.IsNaN(a[i]) && !math.IsNaN(b[i]) {
			sum += (a[i] - b[i]) * (a[i] - b[i])
			count += 1
		}
	}
	return 1.0 / (sum/count + 1)
}

// Compute the Pearson correlation coefficient between a pair of users (or items).
func Pearson(a []float64, b []float64) float64 {
	// Mean of a
	count, sum := .0, .0
	for _, rating := range a {
		sum += rating
		count += 1
	}
	meanA := sum / count
	// Mean of b
	count, sum = .0, .0
	for _, rating := range b {
		sum += rating
		count += 1
	}
	meanB := sum / count
	// Mean-centered cosine
	m, n, l := .0, .0, .0
	for i := range a {
		if !math.IsNaN(a[i]) && !math.IsNaN(b[i]) {
			ratingA := a[i] - meanA
			ratingB := b[i] - meanB
			m += ratingA * ratingA
			n += ratingB * ratingB
			l += ratingA * ratingB
		}
	}
	return l / (math.Sqrt(m) * math.Sqrt(n))
}
