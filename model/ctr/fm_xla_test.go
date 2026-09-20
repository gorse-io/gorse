//go:build cgo && xla

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

package ctr

import (
	"testing"

	"github.com/gomlx/compute/dtypes"
	"github.com/gomlx/compute/dtypes/float16"
	"github.com/stretchr/testify/require"
)

func TestCtrDatasetBatchUsesFloat16Embeddings(t *testing.T) {
	dataSet := newSynthesisDataset()
	fm := NewAFM(nil)
	fm.Init(dataSet)

	batch := (&ctrDataset{
		trainSet:     dataSet,
		numFeatures:  fm.numFeatures,
		numDimension: fm.numDimension,
		embeddingDim: fm.embeddingDim,
		batchSize:    1,
		scalers:      fm.Scalers,
	}).batch(0)
	t.Cleanup(func() { require.NoError(t, batch.Finalize()) })

	require.Len(t, batch.Inputs, 2+len(fm.embeddingDim))
	for i, input := range batch.Inputs[2:] {
		require.Equal(t, dtypes.Float16, input.DType())
		input.MustConstFlatData(func(flat any) {
			values := flat.([]float16.Float16)
			for j, bits := range dataSet.ItemEmbeddings[0][i] {
				require.Equal(t, bits, values[j].Bits())
			}
		})
	}
}
