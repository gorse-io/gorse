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

// FP16 storage, FP32 arithmetic, explicit round-to-nearest-even writeback.
#define HALF_RNE (_MM_FROUND_TO_NEAREST_INT | _MM_FROUND_NO_EXC)
static inline __m256 hload(const uint16_t *p) {
    return _mm256_cvtph_ps(_mm_loadu_si128((const __m128i *)p));
}
static inline void hstore(uint16_t *p, __m256 x) {
    _mm_storeu_si128((__m128i *)p, _mm256_cvtps_ph(x, HALF_RNE));
}

void _h256_add(uint16_t *dst, const uint16_t *s, int64_t n) {
    int64_t i = 0;
    for (; i + 8 <= n; i += 8) hstore(dst+i, _mm256_add_ps(hload(dst+i), hload(s+i)));
    for (; i < n; ++i) dst[i] = _cvtss_sh(_cvtsh_ss(dst[i]) + _cvtsh_ss(s[i]), HALF_RNE);
}
void _h256_add_const(uint16_t *dst, const float *c, int64_t n) {
    __m256 v = _mm256_broadcast_ss(c);
    int64_t i = 0;
    for (; i + 8 <= n; i += 8) hstore(dst+i, _mm256_add_ps(hload(dst+i), v));
    for (; i < n; ++i) dst[i] = _cvtss_sh(_cvtsh_ss(dst[i]) + *c, HALF_RNE);
}
void _h256_mul_to(const uint16_t *a, const uint16_t *b, uint16_t *dst, int64_t n) {
    int64_t i = 0;
    for (; i + 8 <= n; i += 8) hstore(dst+i, _mm256_mul_ps(hload(a+i), hload(b+i)));
    for (; i < n; ++i) dst[i] = _cvtss_sh(_cvtsh_ss(a[i]) * _cvtsh_ss(b[i]), HALF_RNE);
}

// Row-major C = op(A)op(B). No full-matrix promotion and no half partial sums.
// Integer flags keep the generated ABI independent of C/Go bool width.
void _h256_mm(int64_t ta, int64_t tb, int64_t m, int64_t n, int64_t k,
              const uint16_t *a, int64_t lda, const uint16_t *b, int64_t ldb,
              uint16_t *c, int64_t ldc) {
    for (int64_t i = 0; i < m; ++i) {
        int64_t j = 0;
        if (!tb) {
            // Eight independent output columns: retain FP32 accumulators for
            // the entire reduction, including transposed/strided A.
            for (; j + 8 <= n; j += 8) {
                __m256 sum = _mm256_setzero_ps();
                for (int64_t p = 0; p < k; ++p) {
                    float av = _cvtsh_ss(a[ta ? p*lda+i : i*lda+p]);
                    sum = _mm256_add_ps(sum, _mm256_mul_ps(_mm256_set1_ps(av), hload(b+p*ldb+j)));
                }
                hstore(c+i*ldc+j, sum);
            }
        }
        for (; j < n; ++j) {
            float sum = 0;
            int64_t p = 0;
            if (tb && !ta) {
                // Contiguous dot-product layout. Accumulate lanes in FP32;
                // reduce only once, then handle the scalar reduction tail.
                __m256 acc = _mm256_setzero_ps();
                for (; p + 8 <= k; p += 8)
                    acc = _mm256_add_ps(acc, _mm256_mul_ps(hload(a+i*lda+p), hload(b+j*ldb+p)));
                __m128 s = _mm_add_ps(_mm256_castps256_ps128(acc), _mm256_extractf128_ps(acc,1));
                s = _mm_hadd_ps(s,s);
                s = _mm_hadd_ps(s,s);
                sum = _mm_cvtss_f32(s);
            }
            for (; p < k; ++p) {
                float av = _cvtsh_ss(a[ta ? p*lda+i : i*lda+p]);
                float bv = _cvtsh_ss(b[tb ? j*ldb+p : p*ldb+j]);
                sum += av * bv;
            }
            c[i*ldc+j] = _cvtss_sh(sum, HALF_RNE);
        }
    }
}
