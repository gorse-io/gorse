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

static inline __m256 bf16_to_fp32_8(const uint16_t *src)
{
    __m128i values = _mm_loadu_si128((const __m128i *)src);
    __m256i expanded = _mm256_cvtepu16_epi32(values);
    expanded = _mm256_slli_epi32(expanded, 16);
    return _mm256_castsi256_ps(expanded);
}

float _mm256_euclidean_bf16(uint16_t *a, uint16_t *b, int64_t n)
{
    int64_t epoch = n / 8;
    int64_t remain = n % 8;
    __m256 sum = _mm256_setzero_ps();
    for (int64_t i = 0; i < epoch; i++)
    {
        __m256 v1 = bf16_to_fp32_8(a);
        __m256 v2 = bf16_to_fp32_8(b);
        __m256 v = _mm256_sub_ps(v1, v2);
        sum = _mm256_add_ps(sum, _mm256_mul_ps(v, v));
        a += 8;
        b += 8;
    }

    __m128 s7_6_5_4 = _mm256_extractf128_ps(sum, 1);
    __m128 s3_2_1_0 = _mm256_castps256_ps128(sum);
    __m128 s37_26_15_04 = _mm_add_ps(s7_6_5_4, s3_2_1_0);
    __m128 sxx_15_04 = s37_26_15_04;
    __m128 sxx_37_26 = _mm_movehl_ps(s37_26_15_04, s37_26_15_04);
    const __m128 sxx_1357_0246 = _mm_add_ps(sxx_15_04, sxx_37_26);
    const __m128 sxxx_0246 = sxx_1357_0246;
    const __m128 sxxx_1357 = _mm_shuffle_ps(sxx_1357_0246, sxx_1357_0246, 0x1);
    __m128 sxxx_01234567 = _mm_add_ss(sxxx_0246, sxxx_1357);
    float ret = _mm_cvtss_f32(sxxx_01234567);

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
