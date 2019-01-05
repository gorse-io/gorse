package floats

import "gonum.org/v1/gonum/floats"

func SubTo(a, b, dst []float64) {
	floats.SubTo(dst, a, b)
}

func Add(dst, s []float64) {
	floats.Add(dst, s)
}

func MulConst(dst []float64, c float64) {
	MulConstTo(dst, c, dst)
}

func Div(dst, s []float64) {
	floats.Div(dst, s)
}

func Mul(dst, s []float64) {
	floats.Mul(dst, s)
}

func Sub(dst, s []float64) {
	floats.Sub(dst, s)
}
