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
package model

// Index manages the map between sparse Names and dense indices. A sparse ID is
// a user ID or item ID. The dense index is the internal user index or item index
// optimized for faster parameter access and less memory usage.
type Index struct {
	Numbers map[string]int // sparse ID -> dense index
	Names   []string       // dense index -> sparse ID
}

// NotId represents an ID doesn't exist.
const NotId = -1

// NewIndex creates a Index.
func NewIndex() *Index {
	set := new(Index)
	set.Numbers = make(map[string]int)
	set.Names = make([]string, 0)
	return set
}

// Len returns the number of indexed Names.
func (idx *Index) Len() int {
	return len(idx.Names)
}

// Add adds a new ID to the indexer.
func (idx *Index) Add(name string) {
	if _, exist := idx.Numbers[name]; !exist {
		idx.Numbers[name] = len(idx.Names)
		idx.Names = append(idx.Names, name)
	}
}

// ToNumber converts a sparse ID to a dense index.
func (idx *Index) ToNumber(name string) int {
	if denseId, exist := idx.Numbers[name]; exist {
		return denseId
	}
	return NotId
}

// ToName converts a dense index to a sparse ID.
func (idx *Index) ToName(index int) string {
	return idx.Names[index]
}
