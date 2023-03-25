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
	"testing"

	"github.com/stretchr/testify/suite"
)

type SSE2MapTestSuite struct {
	baseTestSuite
}

func (s *SSE2MapTestSuite) SetupTest() {
	s.Map = newB16Map(100)
}

func TestSSE2Map(t *testing.T) {
	suite.Run(t, new(SSE2MapTestSuite))
}

func BenchmarkSSE2MapPut(b *testing.B) {
	m := newB16Map(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Put(int32(i%100), float32(i))
	}
}

func BenchmarkSSE2MapGetHit100(b *testing.B) {
	m := newB16Map(100)
	for i := 0; i < 100; i++ {
		m.Put(int32(i), float32(i))
	}
	nums := randomIntegers(b.N, 100)
	b.ResetTimer()
	for _, num := range nums {
		m.Get(num)
	}
}

func BenchmarkSSE2MapGetHit10(b *testing.B) {
	m := newB16Map(100)
	for i := 0; i < 100; i++ {
		m.Put(int32(i), float32(i))
	}
	nums := randomIntegers(b.N, 1000)
	b.ResetTimer()
	for _, num := range nums {
		m.Get(num)
	}
}

func BenchmarkSSE2MapGetHit1(b *testing.B) {
	m := newB16Map(100)
	for i := 0; i < 100; i++ {
		m.Put(int32(i), float32(i))
	}
	nums := randomIntegers(b.N, 10000)
	b.ResetTimer()
	for _, num := range nums {
		m.Get(num)
	}
}
