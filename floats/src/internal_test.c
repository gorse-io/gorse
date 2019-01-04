#include "minunit.h"
#include "internal.h"

MU_TEST(test_mul_const_to) {
    double a[] = {0,1,2,3,4,5,6,7,8,9,10};
    double dst[11];
    double target[] = {0,2,4,6,8,10,12,14,16,18,20};
    _MulConstTo(a,2,dst,11);
    for (int i = 0; i < 11; i++) {
        mu_check(dst[i] == target[i]);
    }
}

MU_TEST(test_mul_const_add_to) {
    double a[] = {0,1,2,3,4,5,6,7,8,9,10};
    double dst[] = {0,1,2,3,4,5,6,7,8,9,10};
    double target[] = {0,3,6,9,12,15,18,21,14,27,30};
    _MulConstAddTo(a,2,dst,11);
    for (int i = 0; i < 11; i++) {
        mu_check(dst[i] == target[i]);
    }
}

MU_TEST(test_dot) {
    double a[] = {0,1,2,3,4,5,6,7,8,9,10};
    double b[] = {0,2,4,6,8,10,12,14,16,18,20};
    mu_check(_Dot(a, b, 11) == 770);
}

int main(int argc, char const *argv[])
{
    MU_RUN_TEST(test_mul_const_to);
    MU_RUN_TEST(test_mul_const_add_to);
    MU_RUN_TEST(test_dot);
    MU_REPORT();
    return 0;
}
