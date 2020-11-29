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
package base

type StringSet map[string]interface{}

func NewStringSet() StringSet {
	return make(map[string]interface{})
}

func (s StringSet) Add(a ...string) {
	for _, v := range a {
		s[v] = nil
	}
}

func (s StringSet) Contain(v string) bool {
	_, exist := s[v]
	return exist
}

type Set map[int]interface{}

func NewSet(a []int) Set {
	set := make(map[int]interface{})
	for _, s := range a {
		set[s] = nil
	}
	return set
}

func (set Set) Contain(s int) bool {
	_, exist := set[s]
	return exist
}

func (set Set) Len() int {
	return len(set)
}

func (set Set) Add(v int) {
	set[v] = nil
}

func (set Set) Merge(sets ...Set) {
	for _, s := range sets {
		for v := range s {
			set[v] = nil
		}
	}
}
