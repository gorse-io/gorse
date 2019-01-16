package floats

import "gonum.org/v1/gonum/floats"

// SubTo subtracts one vector by another and saves the result in dst: dst = a - b
func SubTo(a, b, dst []float64) {
	floats.SubTo(dst, a, b)
}

// Add two vectors: dst = dst + s
func Add(dst, s []float64) {
	floats.Add(dst, s)
}

// MulConst multiplies a vector with a const: dst = dst * c
func MulConst(dst []float64, c float64) {
	MulConstTo(dst, c, dst)
}

// Div one vectors by another: dst = dst / s
func Div(dst, s []float64) {
	floats.Div(dst, s)
}

// Mul two vectors: dst = dst * s
func Mul(dst, s []float64) {
	floats.Mul(dst, s)
}

// Sub one vector by another: dst = dst - s
func Sub(dst, s []float64) {
	floats.Sub(dst, s)
}
