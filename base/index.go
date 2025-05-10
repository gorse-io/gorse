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

import (
	"encoding/binary"
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/base/encoding"
	"io"
)

// MarshalIndex marshal index into byte stream.
func MarshalIndex(w io.Writer, index *Index) error {
	return index.Marshal(w)
}

// UnmarshalIndex unmarshal index from byte stream.
func UnmarshalIndex(r io.Reader) (*Index, error) {
	index := &Index{}
	err := index.Unmarshal(r)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return index, nil
}

// Index manages the map between sparse Names and dense indices. A sparse ID is
// a user ID or item ID. The dense index is the internal user index or item index
// optimized for faster parameter access and less memory usage.
type Index struct {
	Numbers map[string]int32 // sparse ID -> dense index
	Names   []string         // dense index -> sparse ID
}

// NotId represents an ID doesn't exist.
const NotId = int32(-1)

// NewMapIndex creates a Index.
func NewMapIndex() *Index {
	set := new(Index)
	set.Numbers = make(map[string]int32)
	set.Names = make([]string, 0)
	return set
}

// Len returns the number of indexed Names.
func (idx *Index) Len() int32 {
	if idx == nil {
		return 0
	}
	return int32(len(idx.Names))
}

// Add adds a new ID to the indexer.
func (idx *Index) Add(name string) {
	if _, exist := idx.Numbers[name]; !exist {
		idx.Numbers[name] = int32(len(idx.Names))
		idx.Names = append(idx.Names, name)
	}
}

// ToNumber converts a sparse ID to a dense index.
func (idx *Index) ToNumber(name string) int32 {
	if denseId, exist := idx.Numbers[name]; exist {
		return denseId
	}
	return NotId
}

// ToName converts a dense index to a sparse ID.
func (idx *Index) ToName(index int32) string {
	return idx.Names[index]
}

// GetNames returns all names in current index.
func (idx *Index) GetNames() []string {
	return idx.Names
}

// Marshal map index into byte stream.
func (idx *Index) Marshal(w io.Writer) error {
	// write length
	err := binary.Write(w, binary.LittleEndian, int32(len(idx.Names)))
	if err != nil {
		return errors.Trace(err)
	}
	// write names
	for _, s := range idx.Names {
		err = encoding.WriteString(w, s)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Unmarshal map index from byte stream.
func (idx *Index) Unmarshal(r io.Reader) error {
	// read length
	var n int32
	err := binary.Read(r, binary.LittleEndian, &n)
	if err != nil {
		return errors.Trace(err)
	}
	// write names
	idx.Names = make([]string, 0, n)
	idx.Numbers = make(map[string]int32, n)
	for i := 0; i < int(n); i++ {
		name, err := encoding.ReadString(r)
		if err != nil {
			return errors.Trace(err)
		}
		idx.Add(name)
	}
	return nil
}
