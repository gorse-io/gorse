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

#include <immintrin.h>
#include <stdint.h>

void _mm256_from_float32(float *a, uint16_t *dst, int64_t n)
{
    for (int64_t i = 0; i < n; i += 8)
    {
        __m256 value = _mm256_loadu_ps(a + i);
        __m128i converted = _mm256_cvtps_ph(value, _MM_FROUND_TO_NEAREST_INT | _MM_FROUND_NO_EXC);
        _mm_storeu_si128((__m128i *)(dst + i), converted);
    }
}

void _mm256_to_float32(uint16_t *a, float *dst, int64_t n)
{
    for (int64_t i = 0; i < n; i += 8)
    {
        __m128i value = _mm_loadu_si128((__m128i *)(a + i));
        __m256 converted = _mm256_cvtph_ps(value);
        _mm256_storeu_ps(dst + i, converted);
    }
}
