#include "cublas_sgemm.h"
#include "cublas_v2.h"

int cublas_sgemm(const CUBLAS_LAYOUT Layout, const CUBLAS_TRANSPOSE TransA,
                 const CUBLAS_TRANSPOSE TransB, const int M, const int N,
                 const int K, const float alpha, const float *A,
                 const int lda, const float *B, const int ldb,
                 const float beta, float *C, const int ldc)
{
    cudaError_t cudaStat;
    cublasStatus_t stat;
    cublasHandle_t handle;

    float *devPtrA, *devPtrB, *devPtrC;
    cudaStat = cudaMalloc((void **)&devPtrA, M * K * sizeof(*A));
    if (cudaStat != cudaSuccess)
    {
        return -cudaStat;
    }
    cudaStat = cudaMalloc((void **)&devPtrB, K * N * sizeof(*B));
    if (cudaStat != cudaSuccess)
    {
        cudaFree(devPtrA);
        return -cudaStat;
    }
    cudaStat = cudaMalloc((void **)&devPtrC, M * N * sizeof(*C));
    if (cudaStat != cudaSuccess)
    {
        cudaFree(devPtrA);
        cudaFree(devPtrB);
        return -cudaStat;
    }

    stat = cublasCreate(&handle);
    if (stat != CUBLAS_STATUS_SUCCESS)
    {
        cudaFree(devPtrA);
        cudaFree(devPtrB);
        cudaFree(devPtrC);
        return stat;
    }
    if (TransA == CublasNoTrans)
    {
        stat = cublasSetMatrix(K, M, sizeof(*A), A, K, devPtrA, K);
    }
    else if (TransA == CublasTrans)
    {
        stat = cublasSetMatrix(M, K, sizeof(*A), A, M, devPtrA, M);
    }
    if (stat != CUBLAS_STATUS_SUCCESS)
    {
        cublasDestroy(handle);
        cudaFree(devPtrA);
        cudaFree(devPtrB);
        cudaFree(devPtrC);
        return stat;
    }
    if (TransB == CublasNoTrans)
    {
        stat = cublasSetMatrix(N, K, sizeof(*B), B, N, devPtrB, N);
    }
    else if (TransB == CublasTrans)
    {
        stat = cublasSetMatrix(K, N, sizeof(*B), B, K, devPtrB, K);
    }
    if (stat != CUBLAS_STATUS_SUCCESS)
    {
        cublasDestroy(handle);
        cudaFree(devPtrA);
        cudaFree(devPtrB);
        cudaFree(devPtrC);
        return stat;
    }

    if (TransA == CublasNoTrans && TransB == CublasNoTrans)
    {
        stat = cublasSgemm(handle, CUBLAS_OP_N, CUBLAS_OP_N, N, M, K, &alpha, devPtrB, N, devPtrA, K, &beta, devPtrC, N);
    }
    else if (TransA == CublasNoTrans && TransB == CublasTrans)
    {
        stat = cublasSgemm(handle, CUBLAS_OP_T, CUBLAS_OP_N, N, M, K, &alpha, devPtrB, K, devPtrA, K, &beta, devPtrC, N);
    }
    else if (TransA == CublasTrans && TransB == CublasNoTrans)
    {
        stat = cublasSgemm(handle, CUBLAS_OP_N, CUBLAS_OP_T, N, M, K, &alpha, devPtrB, N, devPtrA, M, &beta, devPtrC, N);
    }
    else if (TransA == CublasTrans && TransB == CublasTrans)
    {
        stat = cublasSgemm(handle, CUBLAS_OP_T, CUBLAS_OP_T, N, M, K, &alpha, devPtrB, K, devPtrA, M, &beta, devPtrC, N);
    }
    if (stat != CUBLAS_STATUS_SUCCESS)
    {
        cublasDestroy(handle);
        cudaFree(devPtrA);
        cudaFree(devPtrB);
        cudaFree(devPtrC);
        return stat;
    }

    stat = cublasGetMatrix(M, N, sizeof(*C), devPtrC, M, C, M);
    if (stat != CUBLAS_STATUS_SUCCESS)
    {
        cublasDestroy(handle);
        cudaFree(devPtrA);
        cudaFree(devPtrB);
        cudaFree(devPtrC);
        return stat;
    }

    cublasDestroy(handle);
    cudaFree(devPtrA);
    cudaFree(devPtrB);
    cudaFree(devPtrC);
    return CUBLAS_STATUS_SUCCESS;
}
