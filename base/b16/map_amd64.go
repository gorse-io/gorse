// Copyright 2023 gorse Project Authors
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

package b16

import (
	"github.com/klauspost/cpuid/v2"
)

//go:generate go run ../../cmd/goat src/scan_sse2.c -O3

var hasSSE2 = cpuid.CPU.Supports(cpuid.SSE2)

func NewMap(n int) Map {
	if hasSSE2 {
		return newB16Map(n)
	}
	return newStdMap(n)
}
