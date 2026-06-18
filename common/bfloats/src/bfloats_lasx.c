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

static inline float bf16_to_fp32(unsigned short v)
{
    union {
        unsigned int u;
        float f;
    } value = {(unsigned int)v << 16};
    return value.f;
}

float lasx_euclidean_bf16(unsigned short *a, unsigned short *b, long n)
{
    long epoch = n / 8;
    long remain = n % 8;
    __m256 sum = (__m256)__lasx_xvldi(0);
    for (long i = 0; i < epoch; i++) {
        float av[8];
        float bv[8];
        for (long j = 0; j < 8; j++) {
            av[j] = bf16_to_fp32(a[j]);
            bv[j] = bf16_to_fp32(b[j]);
        }
        __m256 v1 = (__m256)__lasx_xvld(av, 0);
        __m256 v2 = (__m256)__lasx_xvld(bv, 0);
        __m256 v = __lasx_xvfsub_s(v1, v2);
        sum = __lasx_xvfadd_s(sum, __lasx_xvfmul_s(v, v));
        a += 8;
        b += 8;
    }

    float partial[8];
    __lasx_xvst((__m256i)sum, partial, 0);
    float ret = 0;
    for (long i = 0; i < 8; i++) {
        ret += partial[i];
    }
    for (long i = 0; i < remain; i++) {
        float ai = bf16_to_fp32(a[i]);
        float bi = bf16_to_fp32(b[i]);
        ret += (ai - bi) * (ai - bi);
    }

    float partial_sqrt[8];
    __m256 root = __lasx_xvfsqrt_s((__m256)__lasx_xvldrepl_w(&ret, 0));
    __lasx_xvst((__m256i)root, partial_sqrt, 0);
    return partial_sqrt[0];
}
