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
    size_t vl = __riscv_vsetvl_e32m1(1);
    vfloat32m1_t s = __riscv_vfmv_v_f_f32m1(0, vl);
    for (; n > 0; a += vl, b += vl, n -= vl) {
        vl = __riscv_vsetvl_e32m1(n);
        vfloat32m1_t v1 = __riscv_vle32_v_f32m1(a, vl);
        vfloat32m1_t v2 = __riscv_vle32_v_f32m1(b, vl);
        s = __riscv_vfmacc_vv_f32m1(s, v1, v2, vl);
    }
    float sum = 0;
    vl = __riscv_vsetvlmax_e32m1();
    for (int i = 0; i < vl; i++) {
        sum += __riscv_vfmv_f_s_f32m1_f32(s);
        s = __riscv_vslidedown_vx_f32m1(s, 1, vl);
    }
    return sum;
}

float vdot(float *a, float *b, long n) {
    return dot(a, b, n);
}

float veuclidean(float *a, float *b, long n) {
    size_t vl = __riscv_vsetvl_e32m1(1);
    vfloat32m1_t s = __riscv_vfmv_v_f_f32m1(0, vl);
    for (; n > 0; a += vl, b += vl, n -= vl) {
        vl = __riscv_vsetvl_e32m1(n);
        vfloat32m1_t v1 = __riscv_vle32_v_f32m1(a, vl);
        vfloat32m1_t v2 = __riscv_vle32_v_f32m1(b, vl);
        vfloat32m1_t v = __riscv_vfsub_vv_f32m1(v1, v2, vl);
        s = __riscv_vfmacc_vv_f32m1(s, v, v, vl);
    }
    float sum = 0;
    vl = __riscv_vsetvlmax_e32m1();
    for (int i = 0; i < vl; i++) {
        sum += __riscv_vfmv_f_s_f32m1_f32(s);
        s = __riscv_vslidedown_vx_f32m1(s, 1, vl);
    }
    vl = __riscv_vsetvl_e32m1(1);
    s = __riscv_vfmv_v_f_f32m1(sum, vl);
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
