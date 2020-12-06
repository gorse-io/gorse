// Copyright 2020 gorse Project Authors
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
package leader

type Splitter struct {
	count map[string]int
	total int
}

func NewSplitter() *Splitter {
	s := new(Splitter)
	s.count = make(map[string]int)
	for c := '0'; c <= '9'; c++ {
		s.count[string(c)] = 1
		s.total++
	}
	for c := 'a'; c <= 'z'; c++ {
		s.count[string(c)] = 1
		s.total++
	}
	for c := 'A'; c <= 'Z'; c++ {
		s.count[string(c)] = 1
		s.total++
	}
	return s
}

func (s *Splitter) Add(name string) {
	if len(name) == 0 {
		panic("empty name")
	}
	s.count[string(name[0])]++
	s.total++
}

func (s *Splitter) Split(n int) [][]string {
	ps := NewPrefixes()
	for p, count := range s.count {
		ps.Add(p, count)
	}
	partitionSize := s.total / n
	low, high := 0, len(ps.count)-1
	parts := make([][]string, 0)
	for low <= high && len(parts) < n {
		part := make([]string, 0)
		sum := ps.count[low]
		part = append(part, ps.prefixes[low])
		low++
		for sum < partitionSize && low <= high {
			sum += ps.count[high]
			part = append(part, ps.prefixes[high])
			high--
		}
		parts = append(parts, part)
	}
	for i := low; i <= high; i++ {
		parts[i%len(parts)] = append(parts[i%len(parts)], ps.prefixes[i])
	}
	return parts
}

type Prefixes struct {
	prefixes []string
	count    []int
}

func NewPrefixes() *Prefixes {
	return &Prefixes{
		prefixes: make([]string, 0),
		count:    make([]int, 0),
	}
}

func (p *Prefixes) Add(prefix string, count int) {
	p.prefixes = append(p.prefixes, prefix)
	p.count = append(p.count, count)
}

func (p *Prefixes) Len() int {
	return len(p.prefixes)
}

func (p *Prefixes) Less(i, j int) bool {
	return p.count[i] < p.count[j]
}

func (p *Prefixes) Swap(i, j int) {
	p.count[i], p.count[j] = p.count[j], p.count[i]
	p.prefixes[i], p.prefixes[j] = p.prefixes[j], p.prefixes[i]
}
