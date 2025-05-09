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

void vmul_const_add_to(float *a, float *b, float *c, long n) {
    int epoch = n / 4;
    int remain = n % 4;
    for (int i = 0; i < epoch; i++) {
        float32x4_t v1 = vld1q_f32(a);
        float32x4_t v3 = vld1q_f32(c);
        float32x4_t v = vmlaq_n_f32(v3, v1, *b);
        vst1q_f32(c, v);
        a += 4;
        c += 4;
    }
    for (int i = 0; i < remain; i++) {
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

void vmul_to(float *a, float *b, float *c, long n) {
    int epoch = n / 4;
    int remain = n % 4;
    for (int i = 0; i < epoch; i++) {
        float32x4_t v1 = vld1q_f32(a);
        float32x4_t v2 = vld1q_f32(b);
        float32x4_t v = vmulq_f32(v1, v2);
        vst1q_f32(c, v);
        a += 4;
        b += 4;
        c += 4;
    }
    for (int i = 0; i < remain; i++) {
        c[i] = a[i] * b[i];
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

void vmm(float *a, float *b, float *c, long m, long n, long k, _Bool transA, _Bool transB) {
    if (!transA && !transB)
    {
        for (int i = 0; i < m; i++) {
            for (int l = 0; l < k; l++) {
                for (int j = 0; j < n; j++) {
                    c[i * n + j] += a[i * k + l] * b[l * n + j];
                }
            }
        }
    } else if (!transA && transB)
    {
        for (int i = 0; i < m; i++) {
            for (int j = 0; j < n; j++) {
                c[i * n + j] = dot(a + i * k, b + j * k, k);
            }
        }
    } else if (transA && !transB)
    {
        for (int i = 0; i < m; i++) {
            for (int l = 0; l < k; l++) {
                for (int j = 0; j < n; j++) {
                    c[i * n + j] += a[l * m + i] * b[l * n + j];
                }
            }
        }
    } else if (transA && transB)
    {
        for (int i = 0; i < m; i++) {
            for (int l = 0; l < k; l++) {
                for (int j = 0; j < n; j++) {
                    c[i * n + j] += a[l * m + i] * b[j * k + l];
                }
            }
        }
    }
}
