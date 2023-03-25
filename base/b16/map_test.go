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
	"math/rand"
	"testing"

	"github.com/stretchr/testify/suite"
)

type baseTestSuite struct {
	suite.Suite
	Map
}

func (s *baseTestSuite) TestPutAndGet() {
	for i := 0; i < 100; i++ {
		s.Map.Put(int32(i), float32(i))
	}
	for i := 0; i < 100; i++ {
		value, ok := s.Map.Get(int32(i))
		s.True(ok)
		s.Equal(float32(i), value)
	}
	for i := 100; i < 1000; i++ {
		_, ok := s.Map.Get(int32(i))
		s.False(ok)
	}
}

type StdMapTestSuite struct {
	baseTestSuite
}

func (s *StdMapTestSuite) SetupTest() {
	s.Map = newStdMap(100)
}

func TestStdMap(t *testing.T) {
	suite.Run(t, new(StdMapTestSuite))
}

func BenchmarkStdMapPut(b *testing.B) {
	m := newStdMap(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Put(int32(i%100), float32(i))
	}
}

func BenchmarkStdMapGetHit100(b *testing.B) {
	m := newStdMap(100)
	for i := 0; i < 100; i++ {
		m.Put(int32(i), float32(i))
	}
	nums := randomIntegers(b.N, 100)
	b.ResetTimer()
	for _, num := range nums {
		m.Get(num)
	}
}

func BenchmarkStdMapGetHit10(b *testing.B) {
	m := newStdMap(100)
	for i := 0; i < 100; i++ {
		m.Put(int32(i), float32(i))
	}
	nums := randomIntegers(b.N, 1000)
	b.ResetTimer()
	for _, num := range nums {
		m.Get(num)
	}
}

func BenchmarkStdMapGetHit1(b *testing.B) {
	m := newStdMap(100)
	for i := 0; i < 100; i++ {
		m.Put(int32(i), float32(i))
	}
	nums := randomIntegers(b.N, 10000)
	b.ResetTimer()
	for _, num := range nums {
		m.Get(num)
	}
}

func randomIntegers(size int, n int32) []int32 {
	r := rand.New(rand.NewSource(0))
	integers := make([]int32, size)
	for i := 0; i < size; i++ {
		integers[i] = r.Int31n(int32(n))
	}
	return integers
}
