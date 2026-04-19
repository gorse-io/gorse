#include "munit.h"

#include <math.h>
#include <stdint.h>
#include <stdlib.h>

#if defined(__x86_64__)
float _mm256_euclidean_bf16(uint16_t *a, uint16_t *b, int64_t n);

static float simd_euclidean_bf16(uint16_t *a, uint16_t *b, int64_t n) {
  return _mm256_euclidean_bf16(a, b, n);
}
#elif defined(__aarch64__) || defined(__riscv)
float veuclidean_bf16(uint16_t *a, uint16_t *b, int64_t n);

static float simd_euclidean_bf16(uint16_t *a, uint16_t *b, int64_t n) {
  return veuclidean_bf16(a, b, n);
}
#else
#error "unsupported architecture"
#endif

static inline uint16_t fp32_to_bf16(float value) {
  return (uint16_t) (((union {
    float f;
    uint32_t u;
  }) { .f = value })
                         .u >>
                     16);
}

static inline float bf16_to_fp32(uint16_t value) {
  return ((union {
    uint32_t u;
    float f;
  }) { .u = (uint32_t) value << 16 })
      .f;
}

static float scalar_euclidean_bf16(const uint16_t *a, const uint16_t *b,
                                   int64_t n) {
  float sum = 0;
  for (int64_t i = 0; i < n; i++) {
    float ai = bf16_to_fp32(a[i]);
    float bi = bf16_to_fp32(b[i]);
    float diff = ai - bi;
    sum += diff * diff;
  }
  return sqrtf(sum);
}

static MunitResult test_euclidean_fixed(const MunitParameter params[],
                                        void *data) {
  (void) params;
  (void) data;

  uint16_t a[] = {fp32_to_bf16(0),   fp32_to_bf16(1),     fp32_to_bf16(-2.5f),
                  fp32_to_bf16(3.25), fp32_to_bf16(4),    fp32_to_bf16(5.5f),
                  fp32_to_bf16(6),   fp32_to_bf16(7.75f), fp32_to_bf16(8),
                  fp32_to_bf16(9),   fp32_to_bf16(10)};
  uint16_t b[] = {fp32_to_bf16(1),   fp32_to_bf16(2),     fp32_to_bf16(-1.5f),
                  fp32_to_bf16(1.25), fp32_to_bf16(8),    fp32_to_bf16(3.5f),
                  fp32_to_bf16(12),  fp32_to_bf16(0.75f), fp32_to_bf16(16),
                  fp32_to_bf16(18),  fp32_to_bf16(20)};

  float expect = scalar_euclidean_bf16(a, b, (int64_t) (sizeof(a) / sizeof(a[0])));
  float actual = simd_euclidean_bf16(a, b, (int64_t) (sizeof(a) / sizeof(a[0])));
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

static MunitResult test_euclidean_random(const MunitParameter params[],
                                         void *data) {
  (void) params;
  (void) data;

  for (size_t n = 1; n <= 65; n++) {
    uint16_t *a = malloc(sizeof(uint16_t) * n);
    uint16_t *b = malloc(sizeof(uint16_t) * n);
    munit_assert_not_null(a);
    munit_assert_not_null(b);
    for (size_t i = 0; i < n; i++) {
      a[i] = fp32_to_bf16((float) (munit_rand_double() * 32.0 - 16.0));
      b[i] = fp32_to_bf16((float) (munit_rand_double() * 32.0 - 16.0));
    }

    float expect = scalar_euclidean_bf16(a, b, (int64_t) n);
    float actual = simd_euclidean_bf16(a, b, (int64_t) n);
    munit_assert(fabsf(expect - actual) < 1e-3f);
    free(a);
    free(b);
  }

  return MUNIT_OK;
}

static MunitTest tests[] = {
    {(char *) "/euclidean/fixed", test_euclidean_fixed, NULL, NULL,
     MUNIT_TEST_OPTION_NONE, NULL},
    {(char *) "/euclidean/random", test_euclidean_random, NULL, NULL,
     MUNIT_TEST_OPTION_NONE, NULL},
    {NULL, NULL, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
};

static const MunitSuite suite = {
    (char *) "",
    tests,
    NULL,
    1,
    MUNIT_SUITE_OPTION_NONE,
};

int main(int argc, char *argv[]) { return munit_suite_main(&suite, NULL, argc, argv); }
