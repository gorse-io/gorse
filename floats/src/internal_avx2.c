#include <immintrin.h>

#include "internal.h"

void _MulConstTo(double *a, double c, double *dst, int len) {
    int loop = len / 4;
    int rest = len % 4;
    // Vector part
    __m256d scalar = _mm256_broadcast_sd(&c);
    for (int i = 0; i < loop; i++) {
        __m256d vec = _mm256_load_pd(a);
        __m256d res = _mm256_mul_pd(vec, scalar);
        _mm256_storeu_pd(dst, res);
        // Increase pointers
        a += 4;
        dst += 4;
    }
    // Rest part
    for (int i = 0; i < rest; i++) {
        *dst = (*a) * c;
        // Increase pinter
        a ++;
        dst ++;
    }
}

void _MulConstAddTo(double *a, double c, double *dst, int len) {
    int loop = len / 4;
    int rest = len % 4;
    // Vector part
    __m256d scalar = _mm256_broadcast_sd(&c);
    for (int i = 0; i < loop; i++) {
        __m256d vec1 = _mm256_load_pd(a);
        __m256d vec2 = _mm256_load_pd(dst);
        __m256d res = _mm256_fmadd_pd(vec1, scalar, vec2);
        _mm256_storeu_pd(dst, res);
        // Increase pointers
        a += 4;
        dst += 4;
    }
    // Rest part
    for (int i = 0; i < rest; i++) {
        *dst += (*a) * c;
        // Increase pinter
        a ++;
        dst ++;
    }
}

double _Dot(double *a, double *b, int len) {
    int loop = len / 4;
    int rest = len % 4;
    double ret = 0;
    // Vector part
    __m256d vecSum;
    if (loop > 0) {
        __m256d vec1 = _mm256_load_pd(a);
        __m256d vec2 = _mm256_load_pd(b);
        vecSum = _mm256_mul_pd(vec1, vec2);
        a += 4;
        b += 4;
    }
    for (int i = 1; i < loop; i++) {
        __m256d vec1 = _mm256_load_pd(a);
        __m256d vec2 = _mm256_load_pd(b);
        vecSum = _mm256_fmadd_pd(vec1, vec2, vecSum);
        a += 4;
        b += 4;
    }
    __m256d temp = _mm256_hadd_pd(vecSum, vecSum);
    __m128d high = _mm256_extractf128_pd(temp, 1);
    __m128d dp = _mm_add_pd(high, _mm256_castpd256_pd128(temp));
    double res = _mm_cvtsd_f64(dp);
    // Rest part
    for (int i = 0; i < rest; i++) {
        ret += (*a) * (*b);
        // Increase pinter
        a ++;
        b ++;
    }
    return res;
}
