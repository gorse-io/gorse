// Copyright 2025 gorse Project Authors
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

#include <riscv_vector.h>

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
    for (long i = 0; i < n; i++) {
        c[i] = a[i] / b[i];
    }
}

void vsqrt_to(float *a, float *b, long n) {
    for (size_t vl; n > 0; a += vl, b += vl, n -= vl) {
        vl = __riscv_vsetvl_e32m1(n);
        vfloat32m1_t v1 = __riscv_vle32_v_f32m1(a, vl);
        vfloat32m1_t v2 = __riscv_vfsqrt_v_f32m1(v1, vl);
        __riscv_vse32_v_f32m1(b, v2, vl);
    }
}

inline float dot(float *a, float *b, long n) {
    size_t vlmax = __riscv_vsetvlmax_e32m1();
    int epoch = n / vlmax;
    int remain = n % vlmax;
    vfloat32m1_t s1 = __riscv_vfmv_v_f_f32m1(0, vlmax);
    for (int i = 0; i < epoch; i++) {
        vfloat32m1_t v1 = __riscv_vle32_v_f32m1(a, vlmax);
        vfloat32m1_t v2 = __riscv_vle32_v_f32m1(b, vlmax);
        s1 = __riscv_vfmacc_vv_f32m1(s1, v1, v2, vlmax);
        a += vlmax;
        b += vlmax;
    }
    vfloat32m1_t s = __riscv_vfmv_v_f_f32m1(0, vlmax);
    s = __riscv_vfredosum_vs_f32m1_f32m1(s1, s, vlmax);
    size_t vl = __riscv_vsetvl_e32m1(remain);
    vfloat32m1_t v1 = __riscv_vle32_v_f32m1(a, vl);
    vfloat32m1_t v2 = __riscv_vle32_v_f32m1(b, vl);
    vfloat32m1_t s2 = __riscv_vfmul_vv_f32m1(v1, v2, vl);
    s = __riscv_vfredosum_vs_f32m1_f32m1(s2, s, vl);
    return __riscv_vfmv_f_s_f32m1_f32(s);
}

float vdot(float *a, float *b, long n) {
    return dot(a, b, n);
}

float veuclidean(float *a, float *b, long n) {
    size_t vlmax = __riscv_vsetvlmax_e32m1();
    int epoch = n / vlmax;
    int remain = n % vlmax;
    vfloat32m1_t s1 = __riscv_vfmv_v_f_f32m1(0, vlmax);
    for (int i = 0; i < epoch; i++) {
        vfloat32m1_t v1 = __riscv_vle32_v_f32m1(a, vlmax);
        vfloat32m1_t v2 = __riscv_vle32_v_f32m1(b, vlmax);
        vfloat32m1_t v = __riscv_vfsub_vv_f32m1(v1, v2, vlmax);
        s1 = __riscv_vfmacc_vv_f32m1(s1, v, v, vlmax);
        a += vlmax;
        b += vlmax;
    }
    vfloat32m1_t s = __riscv_vfmv_v_f_f32m1(0, vlmax);
    s = __riscv_vfredosum_vs_f32m1_f32m1(s1, s, vlmax);
    size_t vl = __riscv_vsetvl_e32m1(remain);
    vfloat32m1_t v1 = __riscv_vle32_v_f32m1(a, vl);
    vfloat32m1_t v2 = __riscv_vle32_v_f32m1(b, vl);
    vfloat32m1_t v = __riscv_vfsub_vv_f32m1(v1, v2, vlmax);
    vfloat32m1_t s2 = __riscv_vfmul_vv_f32m1(v, v, vl);
    s = __riscv_vfredosum_vs_f32m1_f32m1(s2, s, vl);
    s = __riscv_vfsqrt_v_f32m1(s, vl);
    return __riscv_vfmv_f_s_f32m1_f32(s);
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
