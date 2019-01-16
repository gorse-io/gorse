//+build !avx2

package floats

import "gonum.org/v1/gonum/floats"

// MulConstTo multiplies a vector and a const, then saves the result in dst: dst = a * c
func MulConstTo(a []float64, c float64, dst []float64) {
	floats.ScaleTo(dst, c, a)
}

// MulConstAddTo multiplies a vector and a const, then adds to dst: dst = dst + a * c
func MulConstAddTo(a []float64, c float64, dst []float64) {
	floats.AddScaled(dst, c, a)
}

// AddTo adds two vectors and saves the result in dst: dst = a + b
func AddTo(a, b, dst []float64) {
	floats.AddTo(dst, a, b)
}

// Dot two vectors.
func Dot(a, b []float64) float64 {
	return floats.Dot(a, b)
}
