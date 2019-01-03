//+build avx2

package floats

func _MulConstTo(a []float64, c float64, dst []float64, len int)
func _MulConstAddTo(a []float64, b float64, dst []float64, len int)
func _Dot(a, b []float64, len int) float64

func MulConstTo(a []float64, c float64, dst []float64) {
	_MulConstTo(a, c, dst, len(a))
}

func MulConstAddTo(a []float64, b float64, dst []float64) {
	_MulConstAddTo(a, b, dst, len(a))
}

func Dot(a, b []float64) float64 {
	return _Dot(a, b, len(a))
}
