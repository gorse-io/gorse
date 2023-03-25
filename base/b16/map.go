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

const loadFactor = 0.75

type Map interface {
	Put(key int32, value float32)
	Get(key int32) (float32, bool)
}

type stdMap[K comparable, V any] struct {
	m map[K]V
}

func (m *stdMap[K, V]) Put(key K, value V) {
	m.m[key] = value
}

func (m *stdMap[K, V]) Get(key K) (V, bool) {
	v, ok := m.m[key]
	return v, ok
}

func newStdMap(n int) Map {
	return &stdMap[int32, float32]{m: make(map[int32]float32, n)}
}

type b16bucket struct {
	h2       [16]uint8
	keys     [16]int32
	values   [16]float32
	size     int64
	overflow *b16bucket
}

type b16Map struct {
	buckets []*b16bucket
}

func newB16Map(n int) *b16Map {
	return &b16Map{
		buckets: make([]*b16bucket, int(float64(n)/loadFactor)/16),
	}
}

func (m *b16Map) hash(key int32) (uint8, uint8) {
	h1 := uint8(int(key) % len(m.buckets))
	h2 := uint8(key % math.MaxUint8)
	return h1, h2
}

func (m *b16Map) Get(key int32) (float32, bool) {
	var (
		result [16]int64
		count  int64
	)
	h1, h2 := m.hash(key)
	for b := m.buckets[h1]; b != nil; b = b.overflow {
		_scan(unsafe.Pointer(&b.h2[0]), unsafe.Pointer(uintptr(h2)), unsafe.Pointer(uintptr(b.size)),
			unsafe.Pointer(&result[0]), unsafe.Pointer(&count))
		for i := 0; i < int(count); i++ {
			if b.keys[result[i]] == key {
				return b.values[result[i]], true
			}
		}
	}
	return 0, false
}

func (m *b16Map) Put(key int32, value float32) {
	h1, h2 := m.hash(key)
	if m.buckets[h1] == nil {
		// create new bucket
		m.buckets[h1] = &b16bucket{}
	}
	for b := m.buckets[h1]; b != nil; b = b.overflow {
		var (
			result [16]int64
			count  int64
		)
		_scan(unsafe.Pointer(&b.h2[0]), unsafe.Pointer(uintptr(h2)), unsafe.Pointer(uintptr(b.size)),
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
			b.overflow = &b16bucket{}
		}
	}
}
