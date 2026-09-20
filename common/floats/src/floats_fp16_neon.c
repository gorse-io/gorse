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

void vfrom_float32(float *a, uint16_t *dst, int64_t n) {
    for (int64_t i = 0; i < n; i += 8) {
        float16x4_t low = vcvt_f16_f32(vld1q_f32(a + i));
        float16x4_t high = vcvt_f16_f32(vld1q_f32(a + i + 4));
        vst1q_u16(dst + i, vreinterpretq_u16_f16(vcombine_f16(low, high)));
    }
}

void vto_float32(uint16_t *a, float *dst, int64_t n) {
    for (int64_t i = 0; i < n; i += 8) {
        float16x8_t value = vreinterpretq_f16_u16(vld1q_u16(a + i));
        vst1q_f32(dst + i, vcvt_f32_f16(vget_low_f16(value)));
        vst1q_f32(dst + i + 4, vcvt_f32_f16(vget_high_f16(value)));
    }
}
