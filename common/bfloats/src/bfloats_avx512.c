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

static inline __m512 bf16_to_fp32_16(const uint16_t *src)
{
    __m256i values = _mm256_loadu_si256((const __m256i *)src);
    __m512i expanded = _mm512_cvtepu16_epi32(values);
    expanded = _mm512_slli_epi32(expanded, 16);
    return _mm512_castsi512_ps(expanded);
}

float _mm512_euclidean_bf16(uint16_t *a, uint16_t *b, int64_t n)
{
    int64_t epoch = n / 16;
    int64_t remain = n % 16;
    __m512 sum = _mm512_setzero_ps();
    for (int64_t i = 0; i < epoch; i++)
    {
        __m512 v1 = bf16_to_fp32_16(a);
        __m512 v2 = bf16_to_fp32_16(b);
        __m512 v = _mm512_sub_ps(v1, v2);
        sum = _mm512_add_ps(sum, _mm512_mul_ps(v, v));
        a += 16;
        b += 16;
    }

    float partial[16];
    _mm512_storeu_ps(partial, sum);
    float ret = 0;
    for (int i = 0; i < 16; i++)
    {
        ret += partial[i];
    }

    for (int64_t i = 0; i < remain; i++)
    {
        float ai = *((float *)(uint32_t[]){(uint32_t)a[i] << 16});
        float bi = *((float *)(uint32_t[]){(uint32_t)b[i] << 16});
        ret += (ai - bi) * (ai - bi);
    }

    __m128 v = _mm_set1_ps(ret);
    __m128 r = _mm_sqrt_ss(v);
    return _mm_cvtss_f32(r);
}
