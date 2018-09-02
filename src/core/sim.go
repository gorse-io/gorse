package core

import "math"

type Sim func(map[int]float64, map[int]float64) float64

// Compute the cosine similarity between a pair of users (or items).
func Cosine(a map[int]float64, b map[int]float64) float64 {
	m, n, l := .0, .0, .0
	for id, ratingA := range a {
		if ratingB, exist := b[id]; exist {
			m += ratingA * ratingA
			n += ratingB * ratingB
			l += ratingA * ratingB
		}
	}
	return l / (math.Sqrt(m) * math.Sqrt(n))
}

// Compute the Mean Squared Difference similarity between a pair of users (or items).
func MSD(a map[int]float64, b map[int]float64) float64 {
	count := 0.0
	sum := 0.0
	for id, ratingA := range a {
		if ratingB, exist := b[id]; exist {
			sum += (ratingA - ratingB) * (ratingA - ratingB)
			count += 1
		}
	}
	return 1.0 / (sum/count + 1)
}

// Compute the Pearson correlation coefficient between a pair of users (or items).
func Pearson(a map[int]float64, b map[int]float64) float64 {
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
	for id, ratingA := range a {
		if ratingB, exist := b[id]; exist {
			ratingA -= meanA
			ratingB -= meanB
			m += ratingA * ratingA
			n += ratingB * ratingB
			l += ratingA * ratingB
		}
	}
	return l / (math.Sqrt(m) * math.Sqrt(n))
}
