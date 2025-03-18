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

package util

import (
	"reflect"
	"strconv"

	"golang.org/x/exp/constraints"
)

func ParseFloat[T constraints.Float](s string) (T, error) {
	v, err := strconv.ParseFloat(s, reflect.TypeOf(T(0)).Bits())
	return T(v), err
}

func ParseUInt[T constraints.Unsigned](s string) (T, error) {
	v, err := strconv.ParseUint(s, 10, reflect.TypeOf(T(0)).Bits())
	return T(v), err
}

func ParseInt[T constraints.Signed](s string) (T, error) {
	v, err := strconv.ParseInt(s, 10, reflect.TypeOf(T(0)).Bits())
	return T(v), err
}

func FormatInt[T constraints.Signed](i T) string {
	return strconv.FormatInt(int64(i), 10)
}
