//+build !avx2

package floats

import "gonum.org/v1/gonum/floats"

func MulConstTo(a []float64, c float64, dst []float64) {
	for i := 0; i < len(a); i++ {
		dst[i] = c * a[i]
	}
}

func MulConstAddTo(a []float64, c float64, dst []float64) {
	for i := 0; i < len(a); i++ {
		dst[i] += a[i] * c
	}
}

func AddTo(a, b, dst []float64) {
	floats.AddTo(dst, a, b)
}

func Dot(a, b []float64) float64 {
	return floats.Dot(a, b)
}
