#include "munit.h"
#include "math.h"

const size_t kVectorLength = 63;
const size_t kIteration = 1;

/* no simd */

void mul_const_add_to(float *a, float *b, float *c, float *dst, int64_t n)
{
  for (int64_t i = 0; i < n; i++)
  {
    dst[i] = a[i] * (*b) + c[i];
  }
}

void mul_const_add(float *a, float *b, float *c, int64_t n)
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

void sub_to(float *a, float *b, float *c, int64_t n)
{
  for (int64_t i = 0; i < n; i++)
  {
    c[i] = a[i] - b[i];
  }
}

void sub(float *a, float *b, int64_t n)
{
  for (int64_t i = 0; i < n; i++)
  {
    a[i] -= b[i];
  }
}

void mul_to(float *a, float *b, float *c, int64_t n)
{
  for (int64_t i = 0; i < n; i++)
  {
    c[i] = a[i] * b[i];
  }
}

void div_to(float *a, float *b, float *c, int64_t n)
{
  for (int64_t i = 0; i < n; i++)
  {
    c[i] = a[i] / b[i];
  }
}

void sqrt_to(float *a, float *b, int64_t n)
{
  for (int64_t i = 0; i < n; i++)
  {
    b[i] = sqrtf(a[i]);
  }
}

float dot(float *a, float *b, int64_t n)
{
  float sum = 0;
  for (int64_t i = 0; i < n; i++)
  {
    sum += a[i] * b[i];
  }
  return sum;
}

float euclidean(float *a, float *b, int64_t n)
{
  float sum = 0;
  for (int64_t i = 0; i < n; i++)
  {
    sum += powf(a[i] - b[i], 2);
  }
  return sqrtf(sum);
}

int rand_float(float *a, int64_t n)
{
  for (int i = 0; i < n; i++)
  {
    a[i] = munit_rand_double();
  }
}

#if defined(__x86_64__)

void _mm256_mul_const_add_to(float *a, float *b, float *c, float *dst, int64_t n);
void _mm256_mul_const_add(float *a, float *b, float *c, int64_t n);
void _mm256_mul_const_to(float *a, float *b, float *c, int64_t n);
void _mm256_mul_const(float *a, float *b, int64_t n);
void _mm256_sub_to(float *a, float *b, float *c, int64_t n);
void _mm256_sub(float *a, float *b, int64_t n);
void _mm256_mul_to(float *a, float *b, float *c, int64_t n);
void _mm256_div_to(float *a, float *b, float *c, int64_t n);
void _mm256_sqrt_to(float *a, float *b, int64_t n);
float _mm256_dot(float *a, float *b, int64_t n);
float _mm256_euclidean(float *a, float *b, int64_t n);

void _mm512_mul_const_add_to(float *a, float *b, float *c, float *dst, int64_t n);
void _mm512_mul_const_add(float *a, float *b, float *c, int64_t n);
void _mm512_mul_const_to(float *a, float *b, float *c, int64_t n);
void _mm512_mul_const(float *a, float *b, int64_t n);
void _mm512_sub_to(float *a, float *b, float *c, int64_t n);
void _mm512_sub(float *a, float *b, int64_t n);
void _mm512_mul_to(float *a, float *b, float *c, int64_t n);
void _mm512_div_to(float *a, float *b, float *c, int64_t n);
void _mm512_sqrt_to(float *a, float *b, int64_t n);
float _mm512_dot(float *a, float *b, int64_t n);
float _mm512_euclidean(float *a, float *b, int64_t n);

MunitResult mm256_mul_const_add_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);
  float d = munit_rand_double();

  mul_const_add_to(a, &d, b, expect, kVectorLength);
  _mm256_mul_const_add_to(a, &d, b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm256_mul_const_add_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const_add(a, &b, expect, kVectorLength);
  _mm256_mul_const_add(a, &b, actual, kVectorLength);
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

MunitResult mm256_sub_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  sub_to(a, b, expect, kVectorLength);
  _mm256_sub_to(a, b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm256_sub_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float expected[kVectorLength], actual[kVectorLength], b[kVectorLength];
  rand_float(b, kVectorLength);
  rand_float(expected, kVectorLength);
  memcpy(expected, actual, sizeof(float) * kVectorLength);

  sub(expected, b, kVectorLength);
  _mm256_sub(actual, b, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expected, actual);
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

MunitResult mm256_div_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  div_to(a, b, expect, kVectorLength);
  _mm256_div_to(a, b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm256_sqrt_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);

  sqrt_to(a, expect, kVectorLength);
  _mm256_sqrt_to(a, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm256_dot_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  float expect = dot(a, b, kVectorLength);
  float actual = _mm256_dot(a, b, kVectorLength);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitResult mm256_euclidean_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  float expect = euclidean(a, b, kVectorLength);
  float actual = _mm256_euclidean(a, b, kVectorLength);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitTest mm256_tests[] = {
    {"mul_const_add_to", mm256_mul_const_add_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const_add", mm256_mul_const_add_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const_to", mm256_mul_const_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const", mm256_mul_const_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"sub", mm256_sub_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"sub_to", mm256_sub_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_to", mm256_mul_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"div_to", mm256_div_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"sqrt_to", mm256_sqrt_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"dot", mm256_dot_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"euclidean", mm256_euclidean_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {NULL, NULL, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL}};

static const MunitSuite mm256_suite = {
    "mm256_", mm256_tests, NULL, kIteration, MUNIT_SUITE_OPTION_NONE};

MunitResult mm512_mul_const_add_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], c[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);
  rand_float(c, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float d = munit_rand_double();

  mul_const_add_to(a, &d, b, expect, kVectorLength);
  _mm512_mul_const_add_to(a, &d, b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm512_mul_const_add_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const_add(a, &b, expect, kVectorLength);
  _mm512_mul_const_add(a, &b, actual, kVectorLength);
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

MunitResult mm512_sub_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float expected[kVectorLength], actual[kVectorLength], b[kVectorLength];
  rand_float(b, kVectorLength);
  rand_float(expected, kVectorLength);
  memcpy(expected, actual, sizeof(float) * kVectorLength);

  sub(expected, b, kVectorLength);
  _mm512_sub(actual, b, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expected, actual);
  return MUNIT_OK;
}

MunitResult mm512_sub_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  sub_to(a, b, expect, kVectorLength);
  _mm512_sub_to(a, b, actual, kVectorLength);
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

MunitResult mm512_div_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  div_to(a, b, expect, kVectorLength);
  _mm512_div_to(a, b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm512_sqrt_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);

  sqrt_to(a, expect, kVectorLength);
  _mm512_sqrt_to(a, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult mm512_dot_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  float expect = dot(a, b, kVectorLength);
  float actual = _mm512_dot(a, b, kVectorLength);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitResult mm512_euclidean_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  float expect = euclidean(a, b, kVectorLength);
  float actual = _mm512_euclidean(a, b, kVectorLength);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitTest mm512_tests[] = {
    {"mul_const_add_to", mm512_mul_const_add_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const_add", mm512_mul_const_add_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const_to", mm512_mul_const_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_const", mm512_mul_const_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"sub_to", mm512_sub_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"sub", mm512_sub_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"mul_to", mm512_mul_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"div_to", mm512_div_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
    {"sqrt_to", mm512_sqrt_to_test, NULL, NULL, MUNIT_TEST_OPTION_NONE, NULL},
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

void vmul_const_add_to(float *a, float *b, float *c, float *dst, int64_t n);
void vmul_const_add(float *a, float *b, float *c, int64_t n);
void vmul_const_to(float *a, float *b, float *c, int64_t n);
void vmul_const(float *a, float *b, int64_t n);
void vsub_to(float *a, float *b, float *c, int64_t n);
void vsub(float *a, float *b, int64_t n);
void vmul_to(float *a, float *b, float *c, int64_t n);
void vdiv_to(float *a, float *b, float *c, int64_t n);
void vsqrt_to(float *a, float *b, int64_t n);
float vdot(float *a, float *b, int64_t n);
float veuclidean(float *a, float *b, int64_t n);

MunitResult vmul_const_add_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);
  float d = munit_rand_double();

  mul_const_add_to(a, &d, b, expect, kVectorLength);
  vmul_const_add_to(a, &d, b, actual, kVectorLength);
  munit_assert_floats_equal(kVectorLength, expect, actual);
  return MUNIT_OK;
}

MunitResult vmul_const_add_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(expect, kVectorLength);
  memcpy(expect, actual, sizeof(float) * kVectorLength);
  float b = munit_rand_double();

  mul_const_add(a, &b, expect, kVectorLength);
  vmul_const_add(a, &b, actual, kVectorLength);
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
  float a[kVectorLength], b[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  float expect = dot(a, b, kVectorLength);
  float actual = vdot(a, b, kVectorLength);
  munit_assert_float_equal(expect, actual, 5);
  return MUNIT_OK;
}

MunitResult veuclidean_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);

  float expect = euclidean(a, b, kVectorLength);
  float actual = veuclidean(a, b, kVectorLength);
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

void svmul_const_add_to(float *a, float *b, float *c, float *dst, long n);
void svmul_const_to(float *a, float *b, float *c, long n);
void svmul_const(float *a, float *b, long n);
void svmul_to(float *a, float *b, float *c, long n);

MunitResult svmul_const_add_to_test(const MunitParameter params[], void *user_data_or_fixture)
{
  float a[kVectorLength], b[kVectorLength], expect[kVectorLength], actual[kVectorLength];
  rand_float(a, kVectorLength);
  rand_float(b, kVectorLength);
  float d = munit_rand_double();

  mul_const_add_to(a, &d, b, expect, kVectorLength);
  svmul_const_add_to(a, &d, b, actual, kVectorLength);
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
