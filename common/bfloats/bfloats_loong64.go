//go:build !noasm && loong64

// Copyright 2026 gorse Project Authors
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

package bfloats

import (
	"unsafe"

	"golang.org/x/sys/cpu"
)

//go:generate sh -c "set -e; tmp=$$(mktemp -d); out=$$tmp/bfloats; mkdir -p $$out; OBJDUMP=loongarch64-linux-gnu-objdump go tool goat -o $$out -t loong64 -O 3 -m lasx src/bfloats_lasx.c; cp $$out/bfloats_lasx.go $$out/bfloats_lasx.s ."

type Feature uint64

const (
	LASX Feature = 1 << iota
)

var feature Feature

func init() {
	if cpu.Loong64.HasLASX {
		feature |= LASX
	}
}

func (feature Feature) euclidean(a, b []uint16) float32 {
	if feature&LASX == LASX {
		return lasx_euclidean_bf16(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	}
	return euclidean(a, b)
}
