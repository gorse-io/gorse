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

void _scan(uint8_t *a, int64_t b, int64_t n, int64_t *ret, int64_t *n_ret)
{
    const __m128i v1 = _mm_loadu_si128(a);
    const __m128i v2 = _mm_set1_epi8(b);
    const __m128i result = _mm_cmpeq_epi8(v1, v2);
    uint64_t mask = _mm_movemask_epi8(result);
    *n_ret = 0;
    for (; mask > 0; mask &= mask - 1)
    {
        int64_t i = __builtin_ctzll(mask);
        if (i >= n)
        {
            return;
        }
        ret[*n_ret] = i;
        (*n_ret)++;
    }
}
