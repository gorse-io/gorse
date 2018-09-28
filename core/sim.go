package core

import (
	"math"
)

// Sim is the prototype of similarity functions.
type Sim func(SortedIdRatings, SortedIdRatings) float64

// Cosine computes the cosine similarity between a pair of users (or items).
func Cosine(a SortedIdRatings, b SortedIdRatings) float64 {
	m, n, l, ptr := .0, .0, .0, 0
	for _, ir := range a.data {
		for ptr < len(b.data) && b.data[ptr].Id < ir.Id {
			ptr++
		}
		if ptr < len(b.data) && b.data[ptr].Id == ir.Id {
			jr := b.data[ptr]
			m += ir.Rating * ir.Rating
			n += jr.Rating * jr.Rating
			l += ir.Rating * jr.Rating
		}
	}
	return l / (math.Sqrt(m) * math.Sqrt(n))
}

// MSD computes the Mean Squared Difference similarity between a pair of users (or items).
func MSD(a SortedIdRatings, b SortedIdRatings) float64 {
	count, sum, ptr := 0.0, 0.0, 0
	for _, ir := range a.data {
		for ptr < len(b.data) && b.data[ptr].Id < ir.Id {
			ptr++
		}
		if ptr < len(b.data) && b.data[ptr].Id == ir.Id {
			jr := b.data[ptr]
			sum += (ir.Rating - jr.Rating) * (ir.Rating - jr.Rating)
			count += 1
		}
	}
	return 1.0 / (sum/count + 1)
}

// Pearson computes the Pearson correlation coefficient between a pair of users (or items).
func Pearson(a SortedIdRatings, b SortedIdRatings) float64 {
	// Mean of a
	count, sum := .0, .0
	for _, ir := range a.data {
		sum += ir.Rating
		count++
	}
	meanA := sum / count
	// Mean of b
	count, sum = .0, .0
	for _, ir := range b.data {
		sum += ir.Rating
		count++
	}
	meanB := sum / count
	// Mean-centered cosine
	m, n, l, ptr := .0, .0, .0, 0
	for _, ir := range a.data {
		for ptr < len(b.data) && b.data[ptr].Id < ir.Id {
			ptr++
		}
		if ptr < len(b.data) && b.data[ptr].Id == ir.Id {
			jr := b.data[ptr]
			ratingA := ir.Rating - meanA
			ratingB := jr.Rating - meanB
			m += ratingA * ratingA
			n += ratingB * ratingB
			l += ratingA * ratingB
		}
	}
	return l / (math.Sqrt(m) * math.Sqrt(n))
}
