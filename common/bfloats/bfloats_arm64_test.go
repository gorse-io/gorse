//go:build !noasm && arm64

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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEuclideanNEONMatchesScalar(t *testing.T) {
	a := FromFloat32([]float32{0, 1, -2.5, 3.25, 4, 5.5, 6, 7.75, 8, 9, 10})
	b := FromFloat32([]float32{1, 2, -1.5, 1.25, 8, 3.5, 12, 0.75, 16, 18, 20})
	require.InDelta(t, euclidean(a, b), feature.euclidean(a, b), 1e-6)
}
