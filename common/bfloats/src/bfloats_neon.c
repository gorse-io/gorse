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

static inline float32x4_t bf16_to_fp32_4(const uint16_t *src)
{
    uint16x4_t values = vld1_u16(src);
    uint32x4_t expanded = vmovl_u16(values);
    expanded = vshlq_n_u32(expanded, 16);
    return vreinterpretq_f32_u32(expanded);
}

float veuclidean_bf16(uint16_t *a, uint16_t *b, int64_t n)
{
    int64_t epoch = n / 4;
    int64_t remain = n % 4;
    float32x4_t sum = vdupq_n_f32(0);
    for (int64_t i = 0; i < epoch; i++)
    {
        float32x4_t v1 = bf16_to_fp32_4(a);
        float32x4_t v2 = bf16_to_fp32_4(b);
        float32x4_t v = vsubq_f32(v1, v2);
        sum = vmlaq_f32(sum, v, v);
        a += 4;
        b += 4;
    }

    float partial[4];
    vst1q_f32(partial, sum);
    float ret = partial[0] + partial[1] + partial[2] + partial[3];
    for (int64_t i = 0; i < remain; i++)
    {
        float ai = *((float *)(uint32_t[]){(uint32_t)a[i] << 16});
        float bi = *((float *)(uint32_t[]){(uint32_t)b[i] << 16});
        ret += (ai - bi) * (ai - bi);
    }

    float32x2_t v = vdup_n_f32(ret);
    float32x2_t r = vsqrt_f32(v);
    return vget_lane_f32(r, 0);
}
