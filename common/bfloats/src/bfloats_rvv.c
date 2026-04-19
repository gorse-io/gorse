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

#include <math.h>
#include <stdint.h>

float veuclidean_bf16(uint16_t *a, uint16_t *b, int64_t n)
{
    float ret = 0;
    for (int64_t i = 0; i < n; i++)
    {
        float ai = *((float *)(uint32_t[]){(uint32_t)a[i] << 16});
        float bi = *((float *)(uint32_t[]){(uint32_t)b[i] << 16});
        ret += (ai - bi) * (ai - bi);
    }
    return sqrtf(ret);
}
