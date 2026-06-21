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

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMilvusRaBitQIndex(t *testing.T) {
	idx, err := milvusIndex(entity.COSINE, QuantizationRaBitQ)
	require.NoError(t, err)
	assert.Equal(t, milvusIVFRaBitQIndexType, idx.IndexType())
	assert.Equal(t, map[string]string{
		"index_type":  string(milvusIVFRaBitQIndexType),
		"metric_type": string(entity.COSINE),
		"nlist":       "128",
	}, idx.Params())
}

func TestMilvusRaBitQSearchParam(t *testing.T) {
	params := milvusSearchParam{
		"nprobe":         defaultMilvusRaBitQNProbe,
		"rbq_query_bits": defaultMilvusRaBitQQueryBits,
		"refine_k":       defaultMilvusRaBitQRefineK,
	}
	params.AddRadius(0.8)
	params.AddRangeFilter(0.1)

	assert.Equal(t, map[string]interface{}{
		"nprobe":         defaultMilvusRaBitQNProbe,
		"rbq_query_bits": defaultMilvusRaBitQQueryBits,
		"refine_k":       defaultMilvusRaBitQRefineK,
		"radius":         0.8,
		"range_filter":   0.1,
	}, params.Params())
}
