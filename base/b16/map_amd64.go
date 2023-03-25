// Copyright 2023 gorse Project Authors
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

package b16

import (
	"math"
	"unsafe"
)

//go:generate go run ../../cmd/goat src/scan_sse2.c -O3

type sse2bucket struct {
	h2       [16]uint8
	keys     [16]int32
	values   [16]float32
	size     int
	overflow *sse2bucket
}

type SSE2Map struct {
	buckets []*sse2bucket
}

func NewSSE2Map(n int) *SSE2Map {
	return &SSE2Map{
		buckets: make([]*sse2bucket, int(float64(n)/loadFactor)/16),
	}
}

func (m *SSE2Map) hash(key int32) (uint8, uint8) {
	h1 := uint8(int(key) % len(m.buckets))
	h2 := uint8(key % math.MaxUint8)
	return h1, h2
}

func (m *SSE2Map) Get(key int32) (float32, bool) {
	var (
		result [16]int64
		count  int64
	)
	h1, h2 := m.hash(key)
	for b := m.buckets[h1]; b != nil; b = b.overflow {
		_mm_scan(unsafe.Pointer(&b.h2[0]), unsafe.Pointer(uintptr(h2)), unsafe.Pointer(uintptr(b.size)),
			unsafe.Pointer(&result[0]), unsafe.Pointer(&count))
		for i := 0; i < int(count); i++ {
			if b.keys[result[i]] == key {
				return b.values[result[i]], true
			}
		}
	}
	return 0, false
}

func (m *SSE2Map) Put(key int32, value float32) {
	h1, h2 := m.hash(key)
	if m.buckets[h1] == nil {
		// create new bucket
		m.buckets[h1] = &sse2bucket{}
	}
	for b := m.buckets[h1]; b != nil; b = b.overflow {
		var (
			result [16]int64
			count  int64
		)
		_mm_scan(unsafe.Pointer(&b.h2[0]), unsafe.Pointer(uintptr(h2)), unsafe.Pointer(uintptr(b.size)),
			unsafe.Pointer(&result[0]), unsafe.Pointer(&count))
		for i := 0; i < int(count); i++ {
			if b.keys[result[i]] == key {
				b.values[result[i]] = value
				return
			}
		}
		if b.size < 16 {
			b.h2[b.size] = h2
			b.keys[b.size] = key
			b.values[b.size] = value
			b.size++
			return
		} else if b.overflow == nil {
			b.overflow = &sse2bucket{}
		}
	}
}
