#include "munit.h"
#include "math.h"

const size_t kVectorLength = 63;
const size_t kIteration = 1;

/* no simd */

void mul_const_add_to(float *a, float *b, float *c, int64_t n)
{
  for (int64_t i = 0; i < n; i++)
  {
    c[i] += a[i] * (*b);
  }
}

void mul_const_to(float *a, float *b, float *c, int64_t n)
{
  for (int64_t i = 0; i < n; i++)
  {
    c[i] = a[i] * (*b);
  }
}

void mul_const(float *a, float *b, int64_t n)
{
  for (int64_t i = 0; i < n; i++)
  {
    a[i] *= *b;
  }
}

void mul_to(float *a, float *b, float *c, int64_t n)
{
  for (int64_t i = 0; i < n; i++)
  {
    c[i] = a[i] * b[i];
  }
}

void dot(float *a, float *b, int64_t n, float *ret)
{
  *ret = 0;
  for (int64_t i = 0; i < n; i++)
  {
    *ret += a[i] * b[i];
  }
}

void euclidean(float *a, float *b, int64_t n, float *ret)
{
  *ret = 0;
  for (int64_t i = 0; i < n; i++)
  {
    *ret += powf(a[i] - b[i], 2);
  }
  *ret = sqrtf(*ret);
}

int rand_float(float *a, int64_t n)
{
  for (int i = 0; i < n; i++)
  {
    a[i] = munit_rand_double();
  }
}

#if defined(__x86_64__)

void _mm256_mul_const_add_to(float *a, float *b, float *c, int64_t n);
void _mm256_mul_const_to(float *a, float *b, float *c, int64_t n);
void _mm256_mul_const(float *a, float *b, int64_t n);
void _mm256_mul_to(float *a, float *b, float *c, int64_t n);
void _mm256_dot(float *a, float *b, int64_t n, float *ret);
void _mm256_euclidean(float *a, float *b, int64_t n, float *ret);

void _mm512_mul_const_add_to(float *a, float *b, float *c, int64_t n);
void _mm512_mul_const_to(float *a, float *b, float *c, int64_t n);
void _mm512_mul_const(float *a, float *b, int64_t n);
void _mm512_mul_to(float *a, float *b, float *c, int64_t n);
void _mm512_dot(float *a, float *b, int64_t n, float *ret);
void _mm512_euclidean(float *a, float *b, int64_t n, float *ret);

MunitResult mm256_mul_const_add_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const_add_to(a, &b, expect, kVectorLength);
  _mm256_mul_const_add_to(a, &b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm256_mul_const_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  float b = munit_rand_double();

  mul_const_to(a, &b, expect, kVectorLength);
  _mm256_mul_const_to(a, &b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm256_mul_const_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float expect[kVectorLength], actual[kVectorLength];
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const(expect, &b, kVectorLength);
  _mm256_mul_const(actual, &b, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm256_mul_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  mul_to(a, b, expect, kVectorLength);
  _mm256_mul_to(a, b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm256_dot_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect, actual;
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  dot(a, b, kVectorLength, &expect);
  _mm256_dot(a, b, kVectorLength, &actual);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitResult mm256_euclidean_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect, actual;
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  euclidean(a, b, kVectorLength, &expect);
  _mm256_euclidean(a, b, kVectorLength, &actual);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitTest mm256_tests[] = {
    {"mul_const_add_to", mm256_mul_const_add_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const_to", mm256_mul_const_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const", mm256_mul_const_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_to", mm256_mul_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"dot", mm256_dot_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"euclidean", mm256_euclidean_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {NULL, NULL, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL}};

static const MunitSuite mm256_suite = {
    "mm256_", mm256_tests, NULL, kIteration, MUNIT_SUITE_OPTION_NONE};

MunitResult mm512_mul_const_add_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const_add_to(a, &b, expect, kVectorLength);
  _mm512_mul_const_add_to(a, &b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm512_mul_const_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  float b = munit_rand_double();

  mul_const_to(a, &b, expect, kVectorLength);
  _mm512_mul_const_to(a, &b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm512_mul_const_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float expect[kVectorLength], actual[kVectorLength];
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const(expect, &b, kVectorLength);
  _mm512_mul_const(actual, &b, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm512_mul_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  mul_to(a, b, expect, kVectorLength);
  _mm512_mul_to(a, b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm512_dot_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect, actual;
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  dot(a, b, kVectorLength, &expect);
  _mm512_dot(a, b, kVectorLength, &actual);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitResult mm512_euclidean_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect, actual;
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  euclidean(a, b, kVectorLength, &expect);
  _mm512_euclidean(a, b, kVectorLength, &actual);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitTest mm512_tests[] = {
    {"mul_const_add_to", mm512_mul_const_add_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const_to", mm512_mul_const_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const", mm512_mul_const_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_to", mm512_mul_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"dot", mm512_dot_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"euclidean", mm512_euclidean_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {NULL, NULL, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL}};

static const MunitSuite mm512_suite = {
    "mm512_", mm512_tests, NULL, kIteration, MUNIT_SUITE_OPTION_NONE};

int main(int argc, char *const argv[MUNIT_ARRAY_PARAM(argc + 1)])
{
  munit_suite_main(&mm256_suite, NULL, argc, argv);
  munit_suite_main(&mm512_suite, NULL, argc, argv);
  return 0;
}

#elif defined(__aarch64__)

void vmul_const_add_to(float *a, float *b, float *c, int64_t n);
void vmul_const_to(float *a, float *b, float *c, int64_t n);
void vmul_const(float *a, float *b, int64_t n);
void vmul_to(float *a, float *b, float *c, int64_t n);
void vdot(float *a, float *b, int64_t n, float *ret);
void veuclidean(float *a, float *b, int64_t n, float *ret);

MunitResult vmul_const_add_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const_add_to(a, &b, expect, kVectorLength);
  vmul_const_add_to(a, &b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult vmul_const_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  float b = munit_rand_double();

  mul_const_to(a, &b, expect, kVectorLength);
  vmul_const_to(a, &b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult vmul_const_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float expect[kVectorLength], actual[kVectorLength];
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const(expect, &b, kVectorLength);
  vmul_const(actual, &b, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult vmul_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  mul_to(a, b, expect, kVectorLength);
  vmul_to(a, b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult vdot_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect, actual;
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  dot(a, b, kVectorLength, &expect);
  vdot(a, b, kVectorLength, &actual);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitResult veuclidean_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect, actual;
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  euclidean(a, b, kVectorLength, &expect);
  veuclidean(a, b, kVectorLength, &actual);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitTest vtests[] = {
    {"mul_const_add_to", vmul_const_add_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const_to", vmul_const_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const", vmul_const_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_to", vmul_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"dot", vdot_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"euclidean", veuclidean_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {NULL, NULL, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL}};

static const MunitSuite vsuite = {
    "v", vtests, NULL, kIteration, MUNIT_SUITE_OPTION_NONE};

void svmul_const_add_to(float *a, float *b, float *c, long n);
void svmul_const_to(float *a, float *b, float *c, long n);
void svmul_const(float *a, float *b, long n);
void svmul_to(float *a, float *b, float *c, long n);

MunitResult svmul_const_add_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const_add_to(a, &b, expect, kVectorLength);
  svmul_const_add_to(a, &b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult svmul_const_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  float b = munit_rand_double();

  mul_const_to(a, &b, expect, kVectorLength);
  svmul_const_to(a, &b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult svmul_const_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float expect[kVectorLength], actual[kVectorLength];
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const(expect, &b, kVectorLength);
  svmul_const(actual, &b, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult svmul_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  mul_to(a, b, expect, kVectorLength);
  svmul_to(a, b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitTest svtests[] = {
    {"mul_const_add_to", svmul_const_add_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const_to", svmul_const_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const", svmul_const_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_to", svmul_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {NULL, NULL, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL}};

static const MunitSuite svsuite = {
    "sv", svtests, NULL, kIteration, MUNIT_SUITE_OPTION_NONE};

int main(int argc, char *const argv[MUNIT_ARRAY_PARAM(argc + 1)])
{
  munit_suite_main(&vsuite, NULL, argc, argv);
  munit_suite_main(&svsuite, NULL, argc, argv);
  return 0;
}

#endif
