// Copyright 2022 gorse Project Authors
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

#include <immintrin.h>
#include <stdint.h>

void _scan_int32(uint8_t *h_keys, int64_t h2, int32_t *keys, int64_t key, int64_t n, int64_t *ret)
{
    const __m128i v1 = _mm_loadu_si128(h_keys);
    const __m128i v2 = _mm_set1_epi8(h2);
    const __m128i result = _mm_cmpeq_epi8(v1, v2);
    uint64_t mask = _mm_movemask_epi8(result);
    *ret = -1;
    for (; mask > 0; mask &= mask - 1)
    {
        int64_t i = __builtin_ctzll(mask);
        if (i >= n)
        {
            return;
        }
        else if (keys[i] == key)
        {
            *ret = i;
            return;
        }
    }
}
