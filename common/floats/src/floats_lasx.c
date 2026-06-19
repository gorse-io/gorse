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

#include <lasxintrin.h>

void lasx_mul_const_add_to(float *a, float *b, float *c, float *dst, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    __m256 vb = (__m256)__lasx_xvldrepl_w(b, 0);
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfadd_s(__lasx_xvfmul_s((__m256)__lasx_xvld(a, 0), vb), (__m256)__lasx_xvld(c, 0));
        __lasx_xvst((__m256i)v, dst, 0);
        a += 8;
        c += 8;
        dst += 8;
    }
    for (long i = 0; i < remain; i++) {
        dst[i] = a[i] * b[0] + c[i];
    }
}

void lasx_mul_const_add(float *a, float *b, float *c, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    __m256 vb = (__m256)__lasx_xvldrepl_w(b, 0);
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfadd_s(__lasx_xvfmul_s((__m256)__lasx_xvld(a, 0), vb), (__m256)__lasx_xvld(c, 0));
        __lasx_xvst((__m256i)v, c, 0);
        a += 8;
        c += 8;
    }
    for (long i = 0; i < remain; i++) {
        c[i] += a[i] * b[0];
    }
}

void lasx_mul_const_to(float *a, float *b, float *c, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    __m256 vb = (__m256)__lasx_xvldrepl_w(b, 0);
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfmul_s((__m256)__lasx_xvld(a, 0), vb);
        __lasx_xvst((__m256i)v, c, 0);
        a += 8;
        c += 8;
    }
    for (long i = 0; i < remain; i++) {
        c[i] = a[i] * b[0];
    }
}

void lasx_mul_const(float *a, float *b, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    __m256 vb = (__m256)__lasx_xvldrepl_w(b, 0);
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfmul_s((__m256)__lasx_xvld(a, 0), vb);
        __lasx_xvst((__m256i)v, a, 0);
        a += 8;
    }
    for (long i = 0; i < remain; i++) {
        a[i] *= b[0];
    }
}

void lasx_add_const(float *a, float *b, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    __m256 vb = (__m256)__lasx_xvldrepl_w(b, 0);
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfadd_s((__m256)__lasx_xvld(a, 0), vb);
        __lasx_xvst((__m256i)v, a, 0);
        a += 8;
    }
    for (long i = 0; i < remain; i++) {
        a[i] += b[0];
    }
}

void lasx_sub_to(float *a, float *b, float *c, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfsub_s((__m256)__lasx_xvld(a, 0), (__m256)__lasx_xvld(b, 0));
        __lasx_xvst((__m256i)v, c, 0);
        a += 8;
        b += 8;
        c += 8;
    }
    for (long i = 0; i < remain; i++) {
        c[i] = a[i] - b[i];
    }
}

void lasx_sub(float *a, float *b, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfsub_s((__m256)__lasx_xvld(a, 0), (__m256)__lasx_xvld(b, 0));
        __lasx_xvst((__m256i)v, a, 0);
        a += 8;
        b += 8;
    }
    for (long i = 0; i < remain; i++) {
        a[i] -= b[i];
    }
}

void lasx_mul_to(float *a, float *b, float *c, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfmul_s((__m256)__lasx_xvld(a, 0), (__m256)__lasx_xvld(b, 0));
        __lasx_xvst((__m256i)v, c, 0);
        a += 8;
        b += 8;
        c += 8;
    }
    for (long i = 0; i < remain; i++) {
        c[i] = a[i] * b[i];
    }
}

void lasx_div_to(float *a, float *b, float *c, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfdiv_s((__m256)__lasx_xvld(a, 0), (__m256)__lasx_xvld(b, 0));
        __lasx_xvst((__m256i)v, c, 0);
        a += 8;
        b += 8;
        c += 8;
    }
    for (long i = 0; i < remain; i++) {
        c[i] = a[i] / b[i];
    }
}

void lasx_sqrt_to(float *a, float *b, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfsqrt_s((__m256)__lasx_xvld(a, 0));
        __lasx_xvst((__m256i)v, b, 0);
        a += 8;
        b += 8;
    }
    for (long i = 0; i < remain; i++) {
        float partial[8];
        __m256 v = __lasx_xvfsqrt_s((__m256)__lasx_xvldrepl_w(&a[i], 0));
        __lasx_xvst((__m256i)v, partial, 0);
        b[i] = partial[0];
    }
}

float lasx_dot(float *a, float *b, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    __m256 s = (__m256)__lasx_xvldi(0);
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfmul_s((__m256)__lasx_xvld(a, 0), (__m256)__lasx_xvld(b, 0));
        s = __lasx_xvfadd_s(s, v);
        a += 8;
        b += 8;
    }
    float partial[8];
    __lasx_xvst((__m256i)s, partial, 0);
    float sum = 0;
    for (long i = 0; i < 8; i++) {
        sum += partial[i];
    }
    for (long i = 0; i < remain; i++) {
        sum += a[i] * b[i];
    }
    return sum;
}

float lasx_euclidean(float *a, float *b, long n) {
    long epoch = n / 8;
    long remain = n % 8;
    __m256 s = (__m256)__lasx_xvldi(0);
    for (long i = 0; i < epoch; i++) {
        __m256 v = __lasx_xvfsub_s((__m256)__lasx_xvld(a, 0), (__m256)__lasx_xvld(b, 0));
        s = __lasx_xvfadd_s(s, __lasx_xvfmul_s(v, v));
        a += 8;
        b += 8;
    }
    float partial[8];
    __lasx_xvst((__m256i)s, partial, 0);
    float sum = 0;
    for (long i = 0; i < 8; i++) {
        sum += partial[i];
    }
    for (long i = 0; i < remain; i++) {
        sum += (a[i] - b[i]) * (a[i] - b[i]);
    }
    float partial_sqrt[8];
    __m256 v = __lasx_xvfsqrt_s((__m256)__lasx_xvldrepl_w(&sum, 0));
    __lasx_xvst((__m256i)v, partial_sqrt, 0);
    return partial_sqrt[0];
}

void lasx_mm(_Bool transA, _Bool transB, long m, long n, long k, float *a, long lda, float *b, long ldb, float *c, long ldc) {
    for (long i = 0; i < m; i++) {
        for (long j = 0; j < n; j++) {
            float sum = 0;
            if (!transA && !transB) {
                for (long l = 0; l < k; l++) {
                    sum += ((volatile float *)a)[i * lda + l] * ((volatile float *)b)[l * ldb + j];
                }
            } else if (!transA && transB) {
                for (long l = 0; l < k; l++) {
                    sum += ((volatile float *)a)[i * lda + l] * ((volatile float *)b)[j * ldb + l];
                }
            } else if (transA && !transB) {
                for (long l = 0; l < k; l++) {
                    sum += ((volatile float *)a)[l * lda + i] * ((volatile float *)b)[l * ldb + j];
                }
            } else {
                for (long l = 0; l < k; l++) {
                    sum += ((volatile float *)a)[l * lda + i] * ((volatile float *)b)[j * ldb + l];
                }
            }
            ((volatile float *)c)[i * ldc + j] = sum;
        }
    }
}
