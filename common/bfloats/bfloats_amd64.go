//go:build !noasm && amd64

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
	"strings"
	"unsafe"

	"golang.org/x/sys/cpu"
)

//go:generate sh -c "set -e; tmp=$$(mktemp -d); out=$$tmp/bfloats; mkdir -p $$out; goat -o $$out -O 3 -m avx2 src/bfloats_avx.c; cp $$out/bfloats_avx.go $$out/bfloats_avx.s ."
//go:generate sh -c "set -e; tmp=$$(mktemp -d); out=$$tmp/bfloats; mkdir -p $$out; goat -o $$out -O 3 -m avx -m avx512f -m avx512bw src/bfloats_avx512.c; cp $$out/bfloats_avx512.go $$out/bfloats_avx512.s ."

type Feature uint64

const (
	AVX2 Feature = 1 << iota
	AVX512F
	AVX512BW
)

const AVX512 = AVX2 | AVX512F | AVX512BW

var feature Feature

func init() {
	if cpu.X86.HasAVX2 {
		feature |= AVX2
	}
	if cpu.X86.HasAVX512F {
		feature |= AVX512F
	}
	if cpu.X86.HasAVX512BW {
		feature |= AVX512BW
	}
}

func (feature Feature) String() string {
	var features []string
	if feature&AVX512 == AVX512 {
		features = append(features, "AVX512")
	} else if feature&AVX2 == AVX2 {
		features = append(features, "AVX2")
	}
	if len(features) == 0 {
		return "AMD64"
	}
	return strings.Join(features, "+")
}

func (feature Feature) euclidean(a, b []uint16) float32 {
	if feature&AVX512 == AVX512 {
		return _mm512_euclidean_bf16(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	}
	if feature&AVX2 == AVX2 {
		return _mm256_euclidean_bf16(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	}
	return euclidean(a, b)
}
