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

type FreqDict struct {
	si  map[string]int
	is  []string
	cnt []int
}

func NewFreqDict() (d *FreqDict) {
	d = &FreqDict{map[string]int{}, []string{}, []int{}}
	return
}

func (d *FreqDict) Count() int {
	return len(d.is)
}

func (d *FreqDict) Id(s string) (y int) {
	if y, ok := d.si[s]; ok {
		d.cnt[y]++
		return y
	}

	y = len(d.is)
	d.si[s] = y
	d.is = append(d.is, s)
	d.cnt = append(d.cnt, 1)
	return
}

func (d *FreqDict) NotCount(s string) (y int) {
	if y, ok := d.si[s]; ok {
		return y
	}

	y = len(d.is)
	d.si[s] = y
	d.is = append(d.is, s)
	d.cnt = append(d.cnt, 0)
	return
}

func (d *FreqDict) String(id int) (s string, ok bool) {
	if id >= len(d.is) {
		return "", false
	}
	return d.is[id], true
}

func (d *FreqDict) Freq(id int) int {
	if id >= len(d.cnt) {
		return 0
	}
	return d.cnt[id]
}
