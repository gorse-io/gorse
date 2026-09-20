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

void lasx_from_float32(float *a, unsigned short *dst, long n) {
    long i = 0;
    for (; i + 16 <= n; i += 16) {
        unsigned short partial[16];
        __m256 low = (__m256)__lasx_xvld(a + i, 0);
        __m256 high = (__m256)__lasx_xvld(a + i + 8, 0);
        __m256i converted = __lasx_xvfcvt_h_s(high, low);
        __lasx_xvst(converted, partial, 0);
        for (long j = 0; j < 4; j++) {
            dst[i + j] = partial[j];
            dst[i + j + 4] = partial[j + 8];
            dst[i + j + 8] = partial[j + 4];
            dst[i + j + 12] = partial[j + 12];
        }
    }
    for (; i < n; i++) {
        __m256 value = (__m256)__lasx_xvldrepl_w(a + i, 0);
        __m256i converted = __lasx_xvfcvt_h_s(value, value);
        dst[i] = (unsigned short)__lasx_xvpickve2gr_wu(converted, 0);
    }
}

void lasx_to_float32(unsigned short *a, float *dst, long n) {
    long i = 0;
    for (; i + 16 <= n; i += 16) {
        float low[8];
        float high[8];
        __m256i value = __lasx_xvld(a + i, 0);
        __lasx_xvst((__m256i)__lasx_xvfcvtl_s_h(value), low, 0);
        __lasx_xvst((__m256i)__lasx_xvfcvth_s_h(value), high, 0);
        for (long j = 0; j < 4; j++) {
            dst[i + j] = low[j];
            dst[i + j + 4] = high[j];
            dst[i + j + 8] = low[j + 4];
            dst[i + j + 12] = high[j + 4];
        }
    }
    for (; i < n; i++) {
        __m256i value = __lasx_xvldrepl_h(a + i, 0);
        __m256 converted = __lasx_xvfcvtl_s_h(value);
        union {
            unsigned int bits;
            float value;
        } scalar = { .bits = __lasx_xvpickve2gr_wu((__m256i)converted, 0) };
        dst[i] = scalar.value;
    }
}

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
    if (!transB) {
        long i = 0;
        for (; i + 4 <= m; i += 4) {
            long j = 0;
            for (; j + 8 <= n; j += 8) {
                __m256 c0 = (__m256)__lasx_xvld(c + i * ldc + j, 0);
                __m256 c1 = (__m256)__lasx_xvld(c + (i + 1) * ldc + j, 0);
                __m256 c2 = (__m256)__lasx_xvld(c + (i + 2) * ldc + j, 0);
                __m256 c3 = (__m256)__lasx_xvld(c + (i + 3) * ldc + j, 0);
                for (long l = 0; l < k; l++) {
                    __m256 bv = (__m256)__lasx_xvld(b + l * ldb + j, 0);
                    long a0 = transA ? l * lda + i : i * lda + l;
                    long a1 = transA ? l * lda + i + 1 : (i + 1) * lda + l;
                    long a2 = transA ? l * lda + i + 2 : (i + 2) * lda + l;
                    long a3 = transA ? l * lda + i + 3 : (i + 3) * lda + l;
                    c0 = __lasx_xvfadd_s(c0, __lasx_xvfmul_s((__m256)__lasx_xvldrepl_w(a + a0, 0), bv));
                    c1 = __lasx_xvfadd_s(c1, __lasx_xvfmul_s((__m256)__lasx_xvldrepl_w(a + a1, 0), bv));
                    c2 = __lasx_xvfadd_s(c2, __lasx_xvfmul_s((__m256)__lasx_xvldrepl_w(a + a2, 0), bv));
                    c3 = __lasx_xvfadd_s(c3, __lasx_xvfmul_s((__m256)__lasx_xvldrepl_w(a + a3, 0), bv));
                }
                __lasx_xvst((__m256i)c0, c + i * ldc + j, 0);
                __lasx_xvst((__m256i)c1, c + (i + 1) * ldc + j, 0);
                __lasx_xvst((__m256i)c2, c + (i + 2) * ldc + j, 0);
                __lasx_xvst((__m256i)c3, c + (i + 3) * ldc + j, 0);
            }
            for (; j < n; j++) {
                for (long l = 0; l < k; l++) {
                    float bv = b[l * ldb + j];
                    c[i * ldc + j] += a[(transA ? l * lda + i : i * lda + l)] * bv;
                    c[(i + 1) * ldc + j] += a[(transA ? l * lda + i + 1 : (i + 1) * lda + l)] * bv;
                    c[(i + 2) * ldc + j] += a[(transA ? l * lda + i + 2 : (i + 2) * lda + l)] * bv;
                    c[(i + 3) * ldc + j] += a[(transA ? l * lda + i + 3 : (i + 3) * lda + l)] * bv;
                }
            }
        }
        for (; i < m; i++) {
            long j = 0;
            for (; j + 8 <= n; j += 8) {
                __m256 cv = (__m256)__lasx_xvld(c + i * ldc + j, 0);
                for (long l = 0; l < k; l++) {
                    __m256 bv = (__m256)__lasx_xvld(b + l * ldb + j, 0);
                    long ai = transA ? l * lda + i : i * lda + l;
                    cv = __lasx_xvfadd_s(cv, __lasx_xvfmul_s((__m256)__lasx_xvldrepl_w(a + ai, 0), bv));
                }
                __lasx_xvst((__m256i)cv, c + i * ldc + j, 0);
            }
            for (; j < n; j++) {
                for (long l = 0; l < k; l++) {
                    long ai = transA ? l * lda + i : i * lda + l;
                    c[i * ldc + j] += a[ai] * b[l * ldb + j];
                }
            }
        }
    } else {
        for (long i = 0; i < m; i++) {
            for (long j = 0; j < n; j++) {
                float sum = 0;
                if (!transA) {
                    for (long l = 0; l < k; l++) {
                        sum += ((volatile float *)a)[i * lda + l] * ((volatile float *)b)[j * ldb + l];
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
}
