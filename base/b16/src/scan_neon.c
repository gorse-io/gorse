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

void _scan(char *a, long b, long n, long *ret, long *n_ret)
{
    const uint8x16_t v1 = vld1q_u8(a);
    const uint8x16_t v2 = vdupq_n_s8(b);
    const uint16x8_t result = vreinterpretq_u16_u8(vceqq_u8(v1, v2));
    uint64_t mask = vget_lane_u64(vreinterpret_u64_u8(vshrn_n_u16(result, 4)), 0);
    mask &= 0x8888888888888888ull;
    *n_ret = 0;
    for (; mask > 0; mask &= mask - 1)
    {
        int64_t i = __builtin_ctzll(mask) >> 2;
        if (i >= n)
        {
            return;
        }
        ret[*n_ret] = i;
        (*n_ret)++;
    }
}
