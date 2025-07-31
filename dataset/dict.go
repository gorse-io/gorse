// Copyright 2025 gorse Project Authors
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

package dataset

import "github.com/gorse-io/gorse/base"

type FreqDict struct {
	si  map[string]int32
	is  []string
	cnt []int32
}

func NewFreqDict() (d *FreqDict) {
	d = &FreqDict{map[string]int32{}, []string{}, []int32{}}
	return
}

func (d *FreqDict) Count() int32 {
	return int32(len(d.is))
}

func (d *FreqDict) Add(s string) (y int32) {
	if y, ok := d.si[s]; ok {
		d.cnt[y]++
		return y
	}

	y = int32(len(d.is))
	d.si[s] = y
	d.is = append(d.is, s)
	d.cnt = append(d.cnt, 1)
	return
}

func (d *FreqDict) AddNoCount(s string) (y int32) {
	if y, ok := d.si[s]; ok {
		return y
	}

	y = int32(len(d.is))
	d.si[s] = y
	d.is = append(d.is, s)
	d.cnt = append(d.cnt, 0)
	return
}

func (d *FreqDict) Id(s string) int32 {
	if y, ok := d.si[s]; ok {
		return y
	}
	return -1
}

func (d *FreqDict) String(id int32) (s string, ok bool) {
	if id >= int32(len(d.is)) {
		return "", false
	}
	return d.is[id], true
}

func (d *FreqDict) Freq(id int32) int32 {
	if id >= int32(len(d.cnt)) {
		return 0
	}
	return d.cnt[id]
}

func (d *FreqDict) ToIndex() *base.Index {
	return &base.Index{
		Numbers: d.si,
		Names:   d.is,
	}
}
