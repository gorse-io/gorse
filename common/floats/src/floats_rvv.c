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
    for (long i = 0; i < n; i++) {
        b[i] = a[i] < 0 ? 0 : sqrtf(a[i]);
    }
}

inline float dot(float *a, float *b, long n) {
    float sum = 0;
    for (int i = 0; i < n; i++) {
        sum += a[i] * b[i];
    }
    return sum;
}

float vdot(float *a, float *b, long n) {
    return dot(a, b, n);
}

float veuclidean(float *a, float *b, long n) {
    float sum = 0;
    for (int i = 0; i < n; i++) {
        float diff = a[i] - b[i];
        sum += diff * diff;
    }
    return sqrtf(sum);
}
