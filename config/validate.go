// Copyright 2021 gorse Project Authors
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

package config

import (
	"fmt"
	"github.com/scylladb/go-set/strset"
	"strings"
)

func validateNotEmpty(name string, val []string) {
	if len(val) == 0 {
		panic(fmt.Sprintf("value of `%s` in config must not be empty, but the current value is [%s]",
			name, strings.Join(val, ",")))
	}
}

func validateNotNegative(name string, val int) {
	if val < 0 {
		panic(fmt.Sprintf("value of `%s` in config must not be negative, but the current value is %d", name, val))
	}
}

func validatePositive(name string, val int) {
	if val == 0 {
		panic(fmt.Sprintf("value of `%s` in config must be positive, but the current value is %d", name, val))
	}
}

func validateIn(name, val string, expectedValues []string) {
	expectedValueSet := strset.New(expectedValues...)
	if !expectedValueSet.Has(val) {
		panic(fmt.Sprintf("value of `%s` in config must be one of [%s], but the current value is %s",
			name, strings.Join(expectedValues, ","), val))
	}
}

func validateSubset(name string, val, expectedValues []string) {
	valueSet := strset.New(val...)
	expectedValueSet := strset.New(expectedValues...)
	if !expectedValueSet.IsSubset(valueSet) {
		panic(fmt.Sprintf("value of `%s` in config must be a subset of [%s], but the current value is [%s]",
			name, strings.Join(expectedValues, ","), strings.Join(val, ",")))
	}
}
