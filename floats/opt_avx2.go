//+build avx2

package floats

func _MulConstTo(a []float64, c float64, dst []float64)
func _MulConstAddTo(a []float64, b float64, dst []float64)
func _AddTo(a, b, dst []float64)
func _Dot(a, b []float64) float64

func MulConstTo(a []float64, c float64, dst []float64) {
	if len(a) != len(dst) {
		panic("floats: lengths of the slices do not match")
	}
	_MulConstTo(a, c, dst)
}

func MulConstAddTo(a []float64, b float64, dst []float64) {
	if len(a) != len(dst) {
		panic("floats: lengths of the slices do not match")
	}
	_MulConstAddTo(a, b, dst)
}

func AddTo(a, b, dst []float64) {
	if len(a) != len(dst) {
		panic("floats: lengths of the slices do not match")
	}
	_AddTo(a, b, dst)
}

func Dot(a, b []float64) float64 {
	if len(a) != len(b) {
		panic("floats: lengths of the slices do not match")
	}
	return _Dot(a, b)
}
