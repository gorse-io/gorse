#pragma once

enum CUBLAS_LAYOUT
{
    CublasRowMajor = 101
};
enum CUBLAS_TRANSPOSE
{
    CublasNoTrans = 111,
    CublasTrans = 112
};

typedef enum CUBLAS_LAYOUT CUBLAS_LAYOUT;
typedef enum CUBLAS_TRANSPOSE CUBLAS_TRANSPOSE;

#if defined(__cplusplus)
extern "C"
{
#endif

    int cublas_sgemm(const CUBLAS_LAYOUT Layout, const CUBLAS_TRANSPOSE TransA,
                     const CUBLAS_TRANSPOSE TransB, const int M, const int N,
                     const int K, const float alpha, const float *A,
                     const int lda, const float *B, const int ldb,
                     const float beta, float *C, const int ldc);

#if defined(__cplusplus)
}
#endif
