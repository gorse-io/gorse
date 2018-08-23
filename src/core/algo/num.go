package algo

import "math/rand"

func Copy(dst []float64, src []float64) []float64 {
	copy(dst, src)
	return dst
}

func MulConst(c float64, dst []float64) []float64 {
	for i := 0; i < len(dst); i++ {
		dst[i] *= c
	}
	return dst
}

func DivConst(c float64, dst []float64) []float64 {
	for i := 0; i < len(dst); i++ {
		dst[i] /= c
	}
	return dst
}

func NewNormalVector(size int, mean float64, stdDev float64) []float64 {
	ret := make([]float64, size)
	for i := 0; i < len(ret); i++ {
		ret[i] = rand.NormFloat64()*stdDev + mean
	}
	return ret
}

func NewUniformVector(size int, low float64, high float64) []float64 {
	ret := make([]float64, size)
	scale := high - low
	for i := 0; i < len(ret); i++ {
		ret[i] = rand.Float64()*scale + low
	}
	return ret
}
