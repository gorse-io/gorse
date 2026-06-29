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

package util

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/gorse-io/gorse/common/log"
	"go.uber.org/zap"
)

// RangeInt generate a slice [0, ..., n-1].
func RangeInt(n int) []int {
	a := make([]int, n)
	for i := range a {
		a[i] = i
	}
	return a
}

// RepeatFloat32s repeats value n times.
func RepeatFloat32s(n int, value float32) []float32 {
	a := make([]float32, n)
	for i := range a {
		a[i] = value
	}
	return a
}

// RemoveEmbeddings recursively removes embeddings from a JSON-like value.
func RemoveEmbeddings(value any, maxValues int) any {
	if isLongFloatArray(value, maxValues) {
		return nil
	}
	v := indirectValue(reflect.ValueOf(value))
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return value
		}
		result := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			result[iter.Key().String()] = RemoveEmbeddings(valueFromReflect(iter.Value()), maxValues)
		}
		return result
	case reflect.Slice, reflect.Array:
		result := make([]any, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			result = append(result, RemoveEmbeddings(valueFromReflect(v.Index(i)), maxValues))
		}
		return result
	default:
		return value
	}
}

func isLongFloatArray(value any, maxValues int) bool {
	v := indirectValue(reflect.ValueOf(value))
	if !v.IsValid() || (v.Kind() != reflect.Slice && v.Kind() != reflect.Array) {
		return false
	}
	if maxValues < 0 {
		maxValues = 0
	}
	if v.Len() <= maxValues {
		return false
	}
	for i := 0; i < v.Len(); i++ {
		if !isFloatValue(valueFromReflect(v.Index(i))) {
			return false
		}
	}
	return true
}

func isFloatValue(value any) bool {
	switch typed := indirectInterface(value).(type) {
	case float64, float32:
		return true
	case json.Number:
		_, err := typed.Float64()
		return err == nil
	default:
		return false
	}
}

func valueFromReflect(v reflect.Value) any {
	v = indirectValue(v)
	if !v.IsValid() {
		return nil
	}
	if v.CanInterface() {
		return v.Interface()
	}
	return fmt.Sprint(v)
}

func indirectInterface(value any) any {
	return valueFromReflect(reflect.ValueOf(value))
}

func indirectValue(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

// NewMatrix32 creates a 2D matrix of 32-bit floats.
func NewMatrix32(row, col int) [][]float32 {
	ret := make([][]float32, row)
	for i := range ret {
		ret[i] = make([]float32, col)
	}
	return ret
}

// NewTensor32 creates a 3D tensor of 32-bit floats.
func NewTensor32(a, b, c int) [][][]float32 {
	ret := make([][][]float32, a)
	for i := range ret {
		ret[i] = NewMatrix32(b, c)
	}
	return ret
}

// NewMatrixInt creates a 2D matrix of integers.
func NewMatrixInt(row, col int) [][]int {
	ret := make([][]int, row)
	for i := range ret {
		ret[i] = make([]int, col)
	}
	return ret
}

// CheckPanic catches panic.
func CheckPanic() {
	if r := recover(); r != nil {
		log.Logger().Error("panic recovered", zap.Any("panic", r))
	}
}

// ValidateId validates user/item id. Id cannot be empty and contain [/,].
func ValidateId(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("id cannot be empty")
	} else if strings.Contains(text, "/") {
		return fmt.Errorf("id cannot contain `/`")
	}
	return nil
}

// MD5 computes the MD5 hash of unordered strings.
func MD5(s ...string) string {
	hash := md5.New()
	sort.Strings(s)
	for _, str := range s {
		hash.Write([]byte(str))
	}
	return hex.EncodeToString(hash.Sum(nil))
}
