//go:build !noasm && riscv64

// Copyright 2026 gorse Project Authors
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

package bfloats

import (
	"unsafe"

	"golang.org/x/sys/cpu"
)

type Feature uint64

const (
	V Feature = 1 << iota
)

var feature Feature

func init() {
	if cpu.RISCV64.HasV {
		feature |= V
	}
}

func (feature Feature) euclidean(a, b []uint16) float32 {
	if feature&V == V {
		return veuclidean_bf16(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	}
	return euclidean(a, b)
}
