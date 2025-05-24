//go:build !noasm

// Copyright 2022 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package floats

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestASIMD(t *testing.T) {
	suite.Run(t, &SIMDTestSuite{Feature: ASIMD})
}

func initializeFloat32Array(n int) []float32 {
	x := make([]float32, n)
	for i := 0; i < n; i++ {
		x[i] = rand.Float32()
	}
	return x
}

func BenchmarkDot(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.dot(v1, v2)
					}
				})
			}
		})
	}
}

func BenchmarkEuclidean(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.euclidean(v1, v2)
					}
				})
			}
		})
	}
}

func BenchmarkMulConstAddTo(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					v3 := make([]float32, i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.mulConstAddTo(v1, 2, v2, v3)
					}
				})
			}
		})
	}
}

func BenchmarkMulConstAdd(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.mulConstAdd(v1, 2, v2)
					}
				})
			}
		})
	}
}

func BenchmarkMulConstTo(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.mulConstTo(v1, 2, v2)
					}
				})
			}
		})
	}
}

func BenchmarkAddConst(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.addConst(v1, 2)
					}
				})
			}
		})
	}
}

func BenchmarkMulConst(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.mulConst(v1, 2)
					}
				})
			}
		})
	}
}

func BenchmarkSubTo(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					v3 := make([]float32, i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.subTo(v1, v2, v3)
					}
				})
			}
		})
	}
}

func BenchmarkSub(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.sub(v1, v2)
					}
				})
			}
		})
	}
}

func BenchmarkMulTo(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					v3 := make([]float32, i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.mulTo(v1, v2, v3)
					}
				})
			}
		})
	}
}

func BenchmarkDivTo(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					v3 := make([]float32, i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.divTo(v1, v2, v3)
					}
				})
			}
		})
	}
}

func BenchmarkSqrtTo(b *testing.B) {
	for _, feat := range []Feature{0, ASIMD} {
		b.Run(feat.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := make([]float32, i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						feat.sqrtTo(v1, v2)
					}
				})
			}
		})
	}
}

func BenchmarkMM(b *testing.B) {
	for _, transA := range []bool{false, true} {
		for _, transB := range []bool{false, true} {
			for _, feat := range []Feature{0, ASIMD} {
				b.Run(fmt.Sprintf("(%v,%v,%v)", transA, transB, feat.String()), func(b *testing.B) {
					for n := 16; n <= 128; n *= 2 {
						b.Run(strconv.Itoa(n), func(b *testing.B) {
							matA := initializeFloat32Array(n * n)
							matB := initializeFloat32Array(n * n)
							matC := make([]float32, n*n)
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								feat.mm(matA, matB, matC, n, n, n, transA, transB)
							}
						})
					}
				})
			}
		}
	}
}
