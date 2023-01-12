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

#include <arm_sve.h>
#include <stdint.h>

void svmul_const_add_to(float *a, float *b, float *c, long n)
{
    for (long i = 0; i < n; i += svcntw())
    {
        svbool_t pg = svwhilelt_b32(i, n);
        svfloat32_t a_seg = svld1(pg, a + i);
        svfloat32_t c_seg = svld1(pg, c + i);
        svst1(pg, c + i, svmla_x(pg, c_seg, a_seg, *b));
    }
}

void svmul_const_to(float *a, float *b, float *c, long n)
{
    for (long i = 0; i < n; i += svcntw())
    {
        svbool_t pg = svwhilelt_b32(i, n);
        svfloat32_t a_seg = svld1(pg, a + i);
        svst1(pg, c + i, svmul_x(pg, a_seg, *b));
    }
}

void svmul_const(float *a, float *b, long n)
{
    for (long i = 0; i < n; i += svcntw())
    {
        svbool_t pg = svwhilelt_b32(i, n);
        svfloat32_t a_seg = svld1(pg, a + i);
        svst1(pg, a + i, svmul_x(pg, a_seg, *b));
    }
}

void svmul_to(float *a, float *b, float *c, long n)
{
    for (long i = 0; i < n; i += svcntw())
    {
        svbool_t pg = svwhilelt_b32(i, n);
        svfloat32_t a_seg = svld1(pg, a + i);
        svfloat32_t b_seg = svld1(pg, b + i);
        svst1(pg, c + i, svmul_x(pg, a_seg, b_seg));
    }
}
