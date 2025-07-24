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

#include <arm_neon.h>
#include <stdint.h>

void vmul_const_add_to(float *a, float *b, float *c, float *dst, long n) {
    for (int i = 0; i < n; i++) {
        dst[i] = a[i] * (*b) + c[i];
    }
}

void vmul_const_add(float *a, float *b, float *c, long n) {
    for (int i = 0; i < n; i++) {
        c[i] += a[i] * b[0];
    }
}

void vmul_const_to(float *a, float *b, float *c, long n) {
    for (int i = 0; i < n; i++) {
        c[i] = a[i] * b[0];
    }
}

void vmul_const(float *a, float *b, long n) {
    for (int i = 0; i < n; i++) {
        a[i] *= b[0];
    }
}

void vadd_const(float *a, float *b, long n) {
    for (int i = 0; i < n; i++) {
        a[i] += b[0];
    }
}

void vsub_to(float *a, float *b, float *c, long n) {
    for (long i = 0; i < n; i++) {
        c[i] = a[i] - b[i];
    }
}

void vsub(float *a, float *b, long n) {
    for (long i = 0; i < n; i++) {
        a[i] -= b[i];
    }
}

void vmul_to(float *a, float *b, float *c, long n) {
    for (long i = 0; i < n; i++) {
        c[i] = a[i] * b[i];
    }
}

void vdiv_to(float *a, float *b, float *c, long n) {
    for (int64_t i = 0; i < n; i++) {
        c[i] = a[i] / b[i];
    }
}

void vsqrt_to(float *a, float *b, long n) {
    int epoch = n / 4;
    int remain = n % 4;
    for (int i = 0; i < epoch; i++) {
        float32x4_t v1 = vld1q_f32(a);
        float32x4_t v2 = vsqrtq_f32(v1);
        vst1q_f32(b, v2);
        a += 4;
        b += 4;
    }
    for (int i = 0; i < remain; i++) {
        float32x2_t v = vdup_n_f32(a[i]);
        float32x2_t r = vsqrt_f32(v);
        b[i] = vget_lane_f32(r, 0);
    }
}

inline float dot(float *a, float *b, long n) {
    int epoch = n / 4;
    int remain = n % 4;
    float32x4_t s = vdupq_n_f32(0);
    if (epoch > 0) {
        float32x4_t v1 = vld1q_f32(a);
        float32x4_t v2 = vld1q_f32(b);
        s = vmulq_f32(v1, v2);
        a += 4;
        b += 4;
    }
    for (int i = 1; i < epoch; i++) {
        float32x4_t v1 = vld1q_f32(a);
        float32x4_t v2 = vld1q_f32(b);
        s = vmlaq_f32(s, v1, v2);
        a += 4;
        b += 4;
    }
    float partial[4];
    vst1q_f32(partial, s);
    float sum = 0;
    for (int i = 0; i < 4; i++) {
        sum += partial[i];
    }
    for (int i = 0; i < remain; i++) {
        sum += a[i] * b[i];
    }
    return sum;
}

float vdot(float *a, float *b, long n) {
    return dot(a, b, n);
}

float veuclidean(float *a, float *b, long n) {
    int epoch = n / 4;
    int remain = n % 4;
    float32x4_t s = vdupq_n_f32(0);
    if (epoch > 0) {
        float32x4_t v1 = vld1q_f32(a);
        float32x4_t v2 = vld1q_f32(b);
        float32x4_t v = vsubq_f32(v1, v2);
        s = vmulq_f32(v, v);
        a += 4;
        b += 4;
    }
    for (int i = 1; i < epoch; i++) {
        float32x4_t v1 = vld1q_f32(a);
        float32x4_t v2 = vld1q_f32(b);
        float32x4_t v = vsubq_f32(v1, v2);
        s = vmlaq_f32(s, v, v);
        a += 4;
        b += 4;
    }
    float partial[4];
    vst1q_f32(partial, s);
    float sum = 0;
    for (int i = 0; i < 4; i++) {
        sum += partial[i];
    }
    for (int i = 0; i < remain; i++) {
        sum += (a[i] - b[i]) * (a[i] - b[i]);
    }
    float32x2_t v = vld1_f32(&sum);
    float32x2_t r = vsqrt_f32(v);
    return vget_lane_f32(r, 0);
}

void vmm(_Bool transA, _Bool transB, long m, long n, long k, float *a, long lda, float *b, long ldb, float *c, long ldc) {
    if (!transA && !transB)
    {
        for (int i = 0; i < m; i++) {
            for (int l = 0; l < k; l++) {
                for (int j = 0; j < n; j++) {
                    c[i * ldc + j] += a[i * lda + l] * b[l * ldb + j];
                }
            }
        }
    } else if (!transA && transB)
    {
        for (int i = 0; i < m; i++) {
            for (int j = 0; j < n; j++) {
                c[i * ldc + j] = dot(a + i * lda, b + j * ldb, k);
            }
        }
    } else if (transA && !transB)
    {
        for (int i = 0; i < m; i++) {
            for (int l = 0; l < k; l++) {
                for (int j = 0; j < n; j++) {
                    c[i * ldc + j] += a[l * lda + i] * b[l * ldb + j];
                }
            }
        }
    } else if (transA && transB)
    {
        for (int i = 0; i < m; i++) {
            for (int l = 0; l < k; l++) {
                for (int j = 0; j < n; j++) {
                    c[i * ldc + j] += a[l * lda + i] * b[j * ldb + l];
                }
            }
        }
    }
}
