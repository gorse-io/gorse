//go:build !noasm && riscv64
// Code generated by GoAT. DO NOT EDIT.
// versions:
// 	clang   18.1.8 (11bb4)
// 	objdump 2.42
// flags: -march=rv64imafdv -O3
// source: src/floats_rvv.c

package floats

import "unsafe"

//go:noescape
func vmul_const_add_to(a, b, c, dst unsafe.Pointer, n int64)

//go:noescape
func vmul_const_add(a, b, c unsafe.Pointer, n int64)

//go:noescape
func vmul_const_to(a, b, c unsafe.Pointer, n int64)

//go:noescape
func vmul_const(a, b unsafe.Pointer, n int64)

//go:noescape
func vadd_const(a, b unsafe.Pointer, n int64)

//go:noescape
func vsub_to(a, b, c unsafe.Pointer, n int64)

//go:noescape
func vsub(a, b unsafe.Pointer, n int64)

//go:noescape
func vmul_to(a, b, c unsafe.Pointer, n int64)

//go:noescape
func vdiv_to(a, b, c unsafe.Pointer, n int64)

//go:noescape
func vsqrt_to(a, b unsafe.Pointer, n int64)

//go:noescape
func vdot(a, b unsafe.Pointer, n int64) (result float32)

//go:noescape
func veuclidean(a, b unsafe.Pointer, n int64) (result float32)

//go:noescape
func vmm(transA, transB bool, m, n, k int64, a unsafe.Pointer, lda int64, b unsafe.Pointer, ldb int64, c unsafe.Pointer, ldc int64)
