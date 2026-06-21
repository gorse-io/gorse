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

package vectors

import (
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQdrantRQQuantizationConfig(t *testing.T) {
	config, err := qdrantQuantizationConfig(VectorConfig{
		Quantization:     QuantizationRQ,
		QuantizationBits: 4,
	})
	require.NoError(t, err)
	require.NotNil(t, config)
	turbo := config.GetTurboquant()
	require.NotNil(t, turbo)
	assert.Equal(t, qdrant.TurboQuantBitSize_Bits4, turbo.GetBits())
}

func TestQdrantRQDefaultBits(t *testing.T) {
	config, err := qdrantQuantizationConfig(VectorConfig{Quantization: QuantizationRQ})
	require.NoError(t, err)
	require.NotNil(t, config)
	turbo := config.GetTurboquant()
	require.NotNil(t, turbo)
	assert.Nil(t, turbo.Bits)
}

func TestQdrantTurboQuantBits(t *testing.T) {
	tests := map[int]qdrant.TurboQuantBitSize{
		1: qdrant.TurboQuantBitSize_Bits1,
		2: qdrant.TurboQuantBitSize_Bits2,
		4: qdrant.TurboQuantBitSize_Bits4,
	}
	for bits, expected := range tests {
		actual, err := qdrantTurboQuantBits(bits)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
	_, err := qdrantTurboQuantBits(8)
	assert.Error(t, err)
}
