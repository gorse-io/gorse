#include <immintrin.h>

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
        *dst = (*dst) * c;
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
        *dst = (*a) * (*b);
        // Increase pinter
        a ++;
        dst ++;
    }
}
