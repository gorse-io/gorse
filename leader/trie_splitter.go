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

import "unicode"

func stringToPath(s string) []int32 {
	path := make([]int32, len(s))
	for i, c := range s {
		if !unicode.IsLetter(c) {
			panic("unsupported name " + s)
		} else if '0' <= c && c <= '9' {
			path[i] = c - '0'
		} else if 'a' <= c && c <= 'z' {

		} else {

		}
	}
}

func pathToString() {

}

type TrieNode struct {
	count    int
	children []*TrieNode
}

type TrieSplitter struct {
	root *TrieNode
}

func (s *TrieSplitter) Add(name string) {

}

func (s *TrieSplitter) Split(n int) [][]string {
	return nil
}
