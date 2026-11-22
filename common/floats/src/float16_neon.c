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

#include <arm_neon.h>
#include <stdint.h>

// Keep FP32 multiply and add separate, matching the scalar accumulation.
#pragma clang fp contract(off)

static inline float32x4_t hload(const uint16_t *p) {
    return vcvt_f32_f16(vreinterpret_f16_u16(vld1_u16(p)));
}

static inline float32x4_t hsplat(uint16_t x) {
    return vcvt_f32_f16(vreinterpret_f16_u16(vdup_n_u16(x)));
}

static inline void hstore(uint16_t *p, float32x4_t x) {
    vst1_u16(p, vreinterpret_u16_f16(vcvt_f16_f32(x)));
}

static inline uint16_t hfirst(float32x4_t x) {
    return vget_lane_u16(vreinterpret_u16_f16(vcvt_f16_f32(x)), 0);
}

void vhadd_float(uint16_t *dst, const uint16_t *s, int64_t n) {
    int64_t i = 0;
    for (; i + 4 <= n; i += 4) {
        hstore(dst + i, vaddq_f32(hload(dst + i), hload(s + i)));
    }
    for (; i < n; i++) {
        dst[i] = hfirst(vaddq_f32(hsplat(dst[i]), hsplat(s[i])));
    }
}

void vhadd_const(uint16_t *dst, const float *c, int64_t n) {
    float32x4_t cv = vdupq_n_f32(*c);
    int64_t i = 0;
    for (; i + 4 <= n; i += 4) {
        hstore(dst + i, vaddq_f32(hload(dst + i), cv));
    }
    for (; i < n; i++) {
        dst[i] = hfirst(vaddq_f32(hsplat(dst[i]), cv));
    }
}

void vhmul_to(const uint16_t *a, const uint16_t *b, uint16_t *dst, int64_t n) {
    int64_t i = 0;
    for (; i + 4 <= n; i += 4) {
        hstore(dst + i, vmulq_f32(hload(a + i), hload(b + i)));
    }
    for (; i < n; i++) {
        dst[i] = hfirst(vmulq_f32(hsplat(a[i]), hsplat(b[i])));
    }
}

// Each lane owns an output column and accumulates the entire k in FP32.
// A transposed B is gathered four halves at a time; no matrix-sized scratch.
void vhmm(_Bool transA, _Bool transB, int64_t m, int64_t n, int64_t k,
          const uint16_t *a, int64_t lda, const uint16_t *b, int64_t ldb,
          uint16_t *c, int64_t ldc) {
    for (int64_t i = 0; i < m; i++) {
        int64_t j = 0;
        for (; j + 4 <= n; j += 4) {
            float32x4_t sum = vdupq_n_f32(0);
            for (int64_t l = 0; l < k; l++) {
                float32x4_t av = hsplat(a[transA ? l * lda + i : i * lda + l]);
                float32x4_t bv;
                if (transB) {
                    uint16x4_t h = vdup_n_u16(b[j * ldb + l]);
                    h = vset_lane_u16(b[(j + 1) * ldb + l], h, 1);
                    h = vset_lane_u16(b[(j + 2) * ldb + l], h, 2);
                    h = vset_lane_u16(b[(j + 3) * ldb + l], h, 3);
                    bv = vcvt_f32_f16(vreinterpret_f16_u16(h));
                } else {
                    bv = hload(b + l * ldb + j);
                }
                sum = vaddq_f32(sum, vmulq_f32(av, bv));
            }
            hstore(c + i * ldc + j, sum);
        }
        for (; j < n; j++) {
            float32x4_t sum = vdupq_n_f32(0);
            for (int64_t l = 0; l < k; l++) {
                float32x4_t av = hsplat(a[transA ? l * lda + i : i * lda + l]);
                float32x4_t bv = hsplat(b[transB ? j * ldb + l : l * ldb + j]);
                sum = vaddq_f32(sum, vmulq_f32(av, bv));
            }
            c[i * ldc + j] = hfirst(sum);
        }
    }
}
