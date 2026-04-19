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

#include <stdint.h>
#include <riscv_vector.h>

float veuclidean_bf16(uint16_t *a, uint16_t *b, int64_t n)
{
    size_t vlmax = __riscv_vsetvlmax_e16m1();
    int64_t epoch = n / vlmax;
    int64_t remain = n % vlmax;

    vfloat32m2_t sum = __riscv_vfmv_v_f_f32m2(0, vlmax);
    for (int64_t i = 0; i < epoch; i++) {
        vuint16m1_t raw_a = __riscv_vle16_v_u16m1(a, vlmax);
        vuint16m1_t raw_b = __riscv_vle16_v_u16m1(b, vlmax);
        vuint32m2_t expanded_a = __riscv_vzext_vf2_u32m2(raw_a, vlmax);
        vuint32m2_t expanded_b = __riscv_vzext_vf2_u32m2(raw_b, vlmax);
        expanded_a = __riscv_vsll_vx_u32m2(expanded_a, 16, vlmax);
        expanded_b = __riscv_vsll_vx_u32m2(expanded_b, 16, vlmax);
        vfloat32m2_t v1 = __riscv_vreinterpret_v_u32m2_f32m2(expanded_a);
        vfloat32m2_t v2 = __riscv_vreinterpret_v_u32m2_f32m2(expanded_b);
        vfloat32m2_t v = __riscv_vfsub_vv_f32m2(v1, v2, vlmax);
        sum = __riscv_vfmacc_vv_f32m2(sum, v, v, vlmax);
        a += vlmax;
        b += vlmax;
    }

    vfloat32m1_t reduced = __riscv_vfmv_v_f_f32m1(0, 1);
    reduced = __riscv_vfredosum_vs_f32m2_f32m1(sum, reduced, vlmax);

    if (remain > 0) {
        size_t vl = __riscv_vsetvl_e16m1(remain);
        vuint16m1_t raw_a = __riscv_vle16_v_u16m1(a, vl);
        vuint16m1_t raw_b = __riscv_vle16_v_u16m1(b, vl);
        vuint32m2_t expanded_a = __riscv_vzext_vf2_u32m2(raw_a, vl);
        vuint32m2_t expanded_b = __riscv_vzext_vf2_u32m2(raw_b, vl);
        expanded_a = __riscv_vsll_vx_u32m2(expanded_a, 16, vl);
        expanded_b = __riscv_vsll_vx_u32m2(expanded_b, 16, vl);
        vfloat32m2_t v1 = __riscv_vreinterpret_v_u32m2_f32m2(expanded_a);
        vfloat32m2_t v2 = __riscv_vreinterpret_v_u32m2_f32m2(expanded_b);
        vfloat32m2_t v = __riscv_vfsub_vv_f32m2(v1, v2, vl);
        vfloat32m2_t partial = __riscv_vfmul_vv_f32m2(v, v, vl);
        reduced = __riscv_vfredosum_vs_f32m2_f32m1(partial, reduced, vl);
        reduced = __riscv_vfsqrt_v_f32m1(reduced, 1);
    } else {
        reduced = __riscv_vfsqrt_v_f32m1(reduced, 1);
    }

    return __riscv_vfmv_f_s_f32m1_f32(reduced);
}
