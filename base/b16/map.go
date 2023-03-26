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
	"unsafe"

	"modernc.org/mathutil"
)

const loadFactor = 0.75

type b16bucket struct {
	h2       [16]uint8
	keys     [16]int32
	values   [16]float32
	size     int64
	overflow *b16bucket
}

type Map struct {
	buckets []b16bucket
	h1bits  int
	h1mask  uint8
}

func NewMap(n int) *Map {
	pow2, bits := nextPowOf2(int(float64(n) / loadFactor))
	return &Map{
		buckets: make([]b16bucket, mathutil.MaxVal(pow2/16, 1)),
		h1bits:  mathutil.MaxVal(bits-4, 0),
		h1mask:  1<<mathutil.MaxVal(bits-4, 0) - 1,
	}
}

func (m *Map) hash(key int32) (uint8, uint8) {
	h1 := uint8(key) & m.h1mask
	h2 := uint8(key >> m.h1bits)
	return h1, h2
}

func (m *Map) Get(key int32) (float32, bool) {
	if len(m.buckets) == 0 {
		return 0, false
	}
	var index int64
	h1, h2 := m.hash(key)
	for b := &m.buckets[h1]; b != nil; b = b.overflow {
		_scan_int32(unsafe.Pointer(&b.h2[0]), unsafe.Pointer(uintptr(h2)),
			unsafe.Pointer(&b.keys[0]), unsafe.Pointer(uintptr(key)),
			unsafe.Pointer(uintptr(b.size)), unsafe.Pointer(&index))
		if index >= 0 {
			return b.values[index], true
		}
	}
	return 0, false
}

func (m *Map) Put(key int32, value float32) {
	h1, h2 := m.hash(key)
	for b := &m.buckets[h1]; b != nil; b = b.overflow {
		var index int64
		_scan_int32(unsafe.Pointer(&b.h2[0]), unsafe.Pointer(uintptr(h2)),
			unsafe.Pointer(&b.keys[0]), unsafe.Pointer(uintptr(key)),
			unsafe.Pointer(uintptr(b.size)), unsafe.Pointer(&index))
		if index >= 0 {
			b.values[index] = value
			return
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

func NewMapFromStdMap(m map[int32]float32) *Map {
	b16map := NewMap(len(m))
	for k, v := range m {
		b16map.Put(k, v)
	}
	return b16map
}

func nextPowOf2(n int) (pow2 int, bits int) {
	pow2 = 1
	for pow2 < n {
		pow2 = pow2 << 1
		bits++
	}
	return
}
