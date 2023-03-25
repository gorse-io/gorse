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

const loadFactor = 0.75

type Map interface {
	Put(key int32, value float32)
	Get(key int32) (float32, bool)
}

type StdMap[K comparable, V any] struct {
	m map[K]V
}

func (m *StdMap[K, V]) Put(key K, value V) {
	m.m[key] = value
}

func (m *StdMap[K, V]) Get(key K) (V, bool) {
	v, ok := m.m[key]
	return v, ok
}

func NewStdMap(n int) Map {
	return &StdMap[int32, float32]{m: make(map[int32]float32, n)}
}
