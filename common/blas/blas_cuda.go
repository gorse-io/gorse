//go:build cgo && cuda

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

package blas

// #cgo LDFLAGS: -Lcublas -lcublas_sgemm -L/usr/local/cuda-12.8/lib64 -lcublas -lcudart
// #include "cublas/cublas_sgemm.h"
import "C"

type Error int

const (
	CublasStatusSuccess                     Error = 0
	CublasStatusNotInitialized              Error = 1
	CublasStatusAllocFailed                 Error = 3
	CublasStatusInvalidValue                Error = 7
	CublasStatusArchMismatch                Error = 8
	CublasStatusMappingError                Error = 11
	CublasStatusExecutionFailed             Error = 13
	CublasStatusInternalError               Error = 14
	CublasStatusNotSupported                Error = 15
	CublasStatusLicenseError                Error = 16
	CudaSuccess                             Error = 0
	CudaErrorInvalidValue                   Error = -1
	CudaErrorMemoryAllocation               Error = -2
	CudaErrorInitializationError            Error = -3
	CudaErrorCudartUnloading                Error = -4
	CudaErrorProfilerDisabled               Error = -5
	CudaErrorProfilerNotInitialized         Error = -6
	CudaErrorProfilerAlreadyStarted         Error = -7
	CudaErrorProfilerAlreadyStopped         Error = -8
	CudaErrorInvalidConfiguration           Error = -9
	CudaErrorInvalidPitchValue              Error = -12
	CudaErrorInvalidSymbol                  Error = -13
	CudaErrorInvalidHostPointer             Error = -16
	CudaErrorInvalidDevicePointer           Error = -17
	CudaErrorInvalidTexture                 Error = -18
	CudaErrorInvalidTextureBinding          Error = -19
	CudaErrorInvalidChannelDescriptor       Error = -20
	CudaErrorInvalidMemcpyDirection         Error = -21
	CudaErrorAddressOfConstant              Error = -22
	CudaErrorTextureFetchFailed             Error = -23
	CudaErrorTextureNotBound                Error = -24
	CudaErrorSynchronizationError           Error = -25
	CudaErrorInvalidFilterSetting           Error = -26
	CudaErrorInvalidNormSetting             Error = -27
	CudaErrorMixedDeviceExecution           Error = -28
	CudaErrorNotYetImplemented              Error = -31
	CudaErrorMemoryValueTooLarge            Error = -32
	CudaErrorStubLibrary                    Error = -34
	CudaErrorInsufficientDriver             Error = -35
	CudaErrorCallRequiresNewerDriver        Error = -36
	CudaErrorInvalidSurface                 Error = -37
	CudaErrorDuplicateVariableName          Error = -43
	CudaErrorDuplicateTextureName           Error = -44
	CudaErrorDuplicateSurfaceName           Error = -45
	CudaErrorDevicesUnavailable             Error = -46
	CudaErrorIncompatibleDriverContext      Error = -49
	CudaErrorMissingConfiguration           Error = -52
	CudaErrorPriorLaunchFailure             Error = -53
	CudaErrorLaunchMaxDepthExceeded         Error = -65
	CudaErrorLaunchFileScopedTex            Error = -66
	CudaErrorLaunchFileScopedSurf           Error = -67
	CudaErrorSyncDepthExceeded              Error = -68
	CudaErrorLaunchPendingCountExceeded     Error = -69
	CudaErrorInvalidDeviceFunction          Error = -98
	CudaErrorNoDevice                       Error = -100
	CudaErrorInvalidDevice                  Error = -101
	CudaErrorDeviceNotLicensed              Error = -102
	CudaErrorSoftwareValidityNotEstablished Error = -103
	CudaErrorStartupFailure                 Error = -127
	CudaErrorInvalidKernelImage             Error = -200
	CudaErrorDeviceUninitialized            Error = -201
	CudaErrorMapBufferObjectFailed          Error = -205
	CudaErrorUnmapBufferObjectFailed        Error = -206
	CudaErrorArrayIsMapped                  Error = -207
	CudaErrorAlreadyMapped                  Error = -208
	CudaErrorNoKernelImageForDevice         Error = -209
	CudaErrorAlreadyAcquired                Error = -210
	CudaErrorNotMapped                      Error = -211
	CudaErrorNotMappedAsArray               Error = -212
	CudaErrorNotMappedAsPointer             Error = -213
	CudaErrorECCUncorrectable               Error = -214
	CudaErrorUnsupportedLimit               Error = -215
	CudaErrorDeviceAlreadyInUse             Error = -216
	CudaErrorPeerAccessUnsupported          Error = -217
	CudaErrorInvalidPtx                     Error = -218
	CudaErrorInvalidGraphicsContext         Error = -219
	CudaErrorNvlinkUncorrectable            Error = -220
	CudaErrorJitCompilerNotFound            Error = -221
	CudaErrorUnsupportedPtxVersion          Error = -222
	CudaErrorJitCompilationDisabled         Error = -223
	CudaErrorUnsupportedExecAffinity        Error = -224
	CudaErrorUnsupportedDevSideSync         Error = -225
	CudaErrorContained                      Error = -226
	CudaErrorInvalidSource                  Error = -300
	CudaErrorFileNotFound                   Error = -301
	CudaErrorSharedObjectSymbolNotFound     Error = -302
	CudaErrorSharedObjectInitFailed         Error = -303
	CudaErrorOperatingSystem                Error = -304
	CudaErrorInvalidResourceHandle          Error = -400
	CudaErrorIllegalState                   Error = -401
	CudaErrorLossyQuery                     Error = -402
	CudaErrorSymbolNotFound                 Error = -500
	CudaErrorNotReady                       Error = -600
	CudaErrorIllegalAddress                 Error = -700
	CudaErrorLaunchOutOfResources           Error = -701
	CudaErrorLaunchTimeout                  Error = -702
	CudaErrorLaunchIncompatibleTexturing    Error = -703
	CudaErrorPeerAccessAlreadyEnabled       Error = -704
	CudaErrorPeerAccessNotEnabled           Error = -705
	CudaErrorSetOnActiveProcess             Error = -708
	CudaErrorContextIsDestroyed             Error = -709
	CudaErrorAssert                         Error = -710
	CudaErrorTooManyPeers                   Error = -711
	CudaErrorHostMemoryAlreadyRegistered    Error = -712
	CudaErrorHostMemoryNotRegistered        Error = -713
	CudaErrorHardwareStackError             Error = -714
	CudaErrorIllegalInstruction             Error = -715
	CudaErrorMisalignedAddress              Error = -716
	CudaErrorInvalidAddressSpace            Error = -717
	CudaErrorInvalidPc                      Error = -718
	CudaErrorLaunchFailure                  Error = -719
	CudaErrorCooperativeLaunchTooLarge      Error = -720
	CudaErrorTensorMemoryLeak               Error = -721
	CudaErrorNotPermitted                   Error = -800
	CudaErrorNotSupported                   Error = -801
	CudaErrorSystemNotReady                 Error = -802
	CudaErrorSystemDriverMismatch           Error = -803
	CudaErrorCompatNotSupportedOnDevice     Error = -804
	CudaErrorMpsConnectionFailed            Error = -805
	CudaErrorMpsRpcFailure                  Error = -806
	CudaErrorMpsServerNotReady              Error = -807
	CudaErrorMpsMaxClientsReached           Error = -808
	CudaErrorMpsMaxConnectionsReached       Error = -809
	CudaErrorMpsClientTerminated            Error = -810
	CudaErrorCdpNotSupported                Error = -811
	CudaErrorCdpVersionMismatch             Error = -812
	CudaErrorStreamCaptureUnsupported       Error = -900
	CudaErrorStreamCaptureInvalidated       Error = -901
	CudaErrorStreamCaptureMerge             Error = -902
	CudaErrorStreamCaptureUnmatched         Error = -903
	CudaErrorStreamCaptureUnjoined          Error = -904
	CudaErrorStreamCaptureIsolation         Error = -905
	CudaErrorStreamCaptureImplicit          Error = -906
	CudaErrorCapturedEvent                  Error = -907
	CudaErrorStreamCaptureWrongThread       Error = -908
	CudaErrorTimeout                        Error = -909
	CudaErrorGraphExecUpdateFailure         Error = -910
	CudaErrorExternalDevice                 Error = -911
	CudaErrorInvalidClusterSize             Error = -912
	CudaErrorFunctionNotLoaded              Error = -913
	CudaErrorInvalidResourceType            Error = -914
	CudaErrorInvalidResourceConfiguration   Error = -915
	CudaErrorUnknown                        Error = -999
	CudaErrorApiFailureBase                 Error = -10000
)

func (err Error) Error() string {
	switch err {
	case CublasStatusNotInitialized:
		return "CUBLAS_STATUS_NOT_INITIALIZED"
	case CublasStatusAllocFailed:
		return "CUBLAS_STATUS_ALLOC_FAILED"
	case CublasStatusInvalidValue:
		return "CUBLAS_STATUS_INVALID_VALUE"
	case CublasStatusArchMismatch:
		return "CUBLAS_STATUS_ARCH_MISMATCH"
	case CublasStatusMappingError:
		return "CUBLAS_STATUS_MAPPING_ERROR"
	case CublasStatusExecutionFailed:
		return "CUBLAS_STATUS_EXECUTION_FAILED"
	case CublasStatusInternalError:
		return "CUBLAS_STATUS_INTERNAL_ERROR"
	case CublasStatusNotSupported:
		return "CUBLAS_STATUS_NOT_SUPPORTED"
	case CublasStatusLicenseError:
		return "CUBLAS_STATUS_LICENSE_ERROR"
	case CudaErrorInvalidValue:
		return "CudaErrorInvalidValue"
	case CudaErrorMemoryAllocation:
		return "CudaErrorMemoryAllocation"
	case CudaErrorInitializationError:
		return "CudaErrorInitializationError"
	case CudaErrorCudartUnloading:
		return "CudaErrorCudartUnloading"
	case CudaErrorProfilerDisabled:
		return "CudaErrorProfilerDisabled"
	case CudaErrorProfilerNotInitialized:
		return "CudaErrorProfilerNotInitialized"
	case CudaErrorProfilerAlreadyStarted:
		return "CudaErrorProfilerAlreadyStarted"
	case CudaErrorProfilerAlreadyStopped:
		return "CudaErrorProfilerAlreadyStopped"
	case CudaErrorInvalidConfiguration:
		return "CudaErrorInvalidConfiguration"
	case CudaErrorInvalidPitchValue:
		return "CudaErrorInvalidPitchValue"
	case CudaErrorInvalidSymbol:
		return "CudaErrorInvalidSymbol"
	case CudaErrorInvalidHostPointer:
		return "CudaErrorInvalidHostPointer"
	case CudaErrorInvalidDevicePointer:
		return "CudaErrorInvalidDevicePointer"
	case CudaErrorInvalidTexture:
		return "CudaErrorInvalidTexture"
	case CudaErrorInvalidTextureBinding:
		return "CudaErrorInvalidTextureBinding"
	case CudaErrorInvalidChannelDescriptor:
		return "CudaErrorInvalidChannelDescriptor"
	case CudaErrorInvalidMemcpyDirection:
		return "CudaErrorInvalidMemcpyDirection"
	case CudaErrorAddressOfConstant:
		return "CudaErrorAddressOfConstant"
	case CudaErrorTextureFetchFailed:
		return "CudaErrorTextureFetchFailed"
	case CudaErrorTextureNotBound:
		return "CudaErrorTextureNotBound"
	case CudaErrorSynchronizationError:
		return "CudaErrorSynchronizationError"
	case CudaErrorInvalidFilterSetting:
		return "CudaErrorInvalidFilterSetting"
	case CudaErrorInvalidNormSetting:
		return "CudaErrorInvalidNormSetting"
	case CudaErrorMixedDeviceExecution:
		return "CudaErrorMixedDeviceExecution"
	case CudaErrorNotYetImplemented:
		return "CudaErrorNotYetImplemented"
	case CudaErrorMemoryValueTooLarge:
		return "CudaErrorMemoryValueTooLarge"
	case CudaErrorStubLibrary:
		return "CudaErrorStubLibrary"
	case CudaErrorInsufficientDriver:
		return "CudaErrorInsufficientDriver"
	case CudaErrorCallRequiresNewerDriver:
		return "CudaErrorCallRequiresNewerDriver"
	case CudaErrorInvalidSurface:
		return "CudaErrorInvalidSurface"
	case CudaErrorDuplicateVariableName:
		return "CudaErrorDuplicateVariableName"
	case CudaErrorDuplicateTextureName:
		return "CudaErrorDuplicateTextureName"
	case CudaErrorDuplicateSurfaceName:
		return "CudaErrorDuplicateSurfaceName"
	case CudaErrorDevicesUnavailable:
		return "CudaErrorDevicesUnavailable"
	case CudaErrorIncompatibleDriverContext:
		return "CudaErrorIncompatibleDriverContext"
	case CudaErrorMissingConfiguration:
		return "CudaErrorMissingConfiguration"
	case CudaErrorPriorLaunchFailure:
		return "CudaErrorPriorLaunchFailure"
	case CudaErrorLaunchMaxDepthExceeded:
		return "CudaErrorLaunchMaxDepthExceeded"
	case CudaErrorLaunchFileScopedTex:
		return "CudaErrorLaunchFileScopedTex"
	case CudaErrorLaunchFileScopedSurf:
		return "CudaErrorLaunchFileScopedSurf"
	case CudaErrorSyncDepthExceeded:
		return "CudaErrorSyncDepthExceeded"
	case CudaErrorLaunchPendingCountExceeded:
		return "CudaErrorLaunchPendingCountExceeded"
	case CudaErrorInvalidDeviceFunction:
		return "CudaErrorInvalidDeviceFunction"
	case CudaErrorNoDevice:
		return "CudaErrorNoDevice"
	case CudaErrorInvalidDevice:
		return "CudaErrorInvalidDevice"
	case CudaErrorDeviceNotLicensed:
		return "CudaErrorDeviceNotLicensed"
	case CudaErrorSoftwareValidityNotEstablished:
		return "CudaErrorSoftwareValidityNotEstablished"
	case CudaErrorStartupFailure:
		return "CudaErrorStartupFailure"
	case CudaErrorInvalidKernelImage:
		return "CudaErrorInvalidKernelImage"
	case CudaErrorDeviceUninitialized:
		return "CudaErrorDeviceUninitialized"
	case CudaErrorMapBufferObjectFailed:
		return "CudaErrorMapBufferObjectFailed"
	case CudaErrorUnmapBufferObjectFailed:
		return "CudaErrorUnmapBufferObjectFailed"
	case CudaErrorArrayIsMapped:
		return "CudaErrorArrayIsMapped"
	case CudaErrorAlreadyMapped:
		return "CudaErrorAlreadyMapped"
	case CudaErrorNoKernelImageForDevice:
		return "CudaErrorNoKernelImageForDevice"
	case CudaErrorAlreadyAcquired:
		return "CudaErrorAlreadyAcquired"
	case CudaErrorNotMapped:
		return "CudaErrorNotMapped"
	case CudaErrorNotMappedAsArray:
		return "CudaErrorNotMappedAsArray"
	case CudaErrorNotMappedAsPointer:
		return "CudaErrorNotMappedAsPointer"
	case CudaErrorECCUncorrectable:
		return "CudaErrorECCUncorrectable"
	case CudaErrorUnsupportedLimit:
		return "CudaErrorUnsupportedLimit"
	case CudaErrorDeviceAlreadyInUse:
		return "CudaErrorDeviceAlreadyInUse"
	case CudaErrorPeerAccessUnsupported:
		return "CudaErrorPeerAccessUnsupported"
	case CudaErrorInvalidPtx:
		return "CudaErrorInvalidPtx"
	case CudaErrorInvalidGraphicsContext:
		return "CudaErrorInvalidGraphicsContext"
	case CudaErrorNvlinkUncorrectable:
		return "CudaErrorNvlinkUncorrectable"
	case CudaErrorJitCompilerNotFound:
		return "CudaErrorJitCompilerNotFound"
	case CudaErrorUnsupportedPtxVersion:
		return "CudaErrorUnsupportedPtxVersion"
	case CudaErrorJitCompilationDisabled:
		return "CudaErrorJitCompilationDisabled"
	case CudaErrorUnsupportedExecAffinity:
		return "CudaErrorUnsupportedExecAffinity"
	case CudaErrorUnsupportedDevSideSync:
		return "CudaErrorUnsupportedDevSideSync"
	case CudaErrorContained:
		return "CudaErrorContained"
	case CudaErrorInvalidSource:
		return "CudaErrorInvalidSource"
	case CudaErrorFileNotFound:
		return "CudaErrorFileNotFound"
	case CudaErrorSharedObjectSymbolNotFound:
		return "CudaErrorSharedObjectSymbolNotFound"
	case CudaErrorSharedObjectInitFailed:
		return "CudaErrorSharedObjectInitFailed"
	case CudaErrorOperatingSystem:
		return "CudaErrorOperatingSystem"
	case CudaErrorInvalidResourceHandle:
		return "CudaErrorInvalidResourceHandle"
	case CudaErrorIllegalState:
		return "CudaErrorIllegalState"
	case CudaErrorLossyQuery:
		return "CudaErrorLossyQuery"
	case CudaErrorSymbolNotFound:
		return "CudaErrorSymbolNotFound"
	case CudaErrorNotReady:
		return "CudaErrorNotReady"
	case CudaErrorIllegalAddress:
		return "CudaErrorIllegalAddress"
	case CudaErrorLaunchOutOfResources:
		return "CudaErrorLaunchOutOfResources"
	case CudaErrorLaunchTimeout:
		return "CudaErrorLaunchTimeout"
	case CudaErrorLaunchIncompatibleTexturing:
		return "CudaErrorLaunchIncompatibleTexturing"
	case CudaErrorPeerAccessAlreadyEnabled:
		return "CudaErrorPeerAccessAlreadyEnabled"
	case CudaErrorPeerAccessNotEnabled:
		return "CudaErrorPeerAccessNotEnabled"
	case CudaErrorSetOnActiveProcess:
		return "CudaErrorSetOnActiveProcess"
	case CudaErrorContextIsDestroyed:
		return "CudaErrorContextIsDestroyed"
	case CudaErrorAssert:
		return "CudaErrorAssert"
	case CudaErrorTooManyPeers:
		return "CudaErrorTooManyPeers"
	case CudaErrorHostMemoryAlreadyRegistered:
		return "CudaErrorHostMemoryAlreadyRegistered"
	case CudaErrorHostMemoryNotRegistered:
		return "CudaErrorHostMemoryNotRegistered"
	case CudaErrorHardwareStackError:
		return "CudaErrorHardwareStackError"
	case CudaErrorIllegalInstruction:
		return "CudaErrorIllegalInstruction"
	case CudaErrorMisalignedAddress:
		return "CudaErrorMisalignedAddress"
	case CudaErrorInvalidAddressSpace:
		return "CudaErrorInvalidAddressSpace"
	case CudaErrorInvalidPc:
		return "CudaErrorInvalidPc"
	case CudaErrorLaunchFailure:
		return "CudaErrorLaunchFailure"
	case CudaErrorCooperativeLaunchTooLarge:
		return "CudaErrorCooperativeLaunchTooLarge"
	case CudaErrorTensorMemoryLeak:
		return "CudaErrorTensorMemoryLeak"
	case CudaErrorNotPermitted:
		return "CudaErrorNotPermitted"
	case CudaErrorNotSupported:
		return "CudaErrorNotSupported"
	case CudaErrorSystemNotReady:
		return "CudaErrorSystemNotReady"
	case CudaErrorSystemDriverMismatch:
		return "CudaErrorSystemDriverMismatch"
	case CudaErrorCompatNotSupportedOnDevice:
		return "CudaErrorCompatNotSupportedOnDevice"
	case CudaErrorMpsConnectionFailed:
		return "CudaErrorMpsConnectionFailed"
	case CudaErrorMpsRpcFailure:
		return "CudaErrorMpsRpcFailure"
	case CudaErrorMpsServerNotReady:
		return "CudaErrorMpsServerNotReady"
	case CudaErrorMpsMaxClientsReached:
		return "CudaErrorMpsMaxClientsReached"
	case CudaErrorMpsMaxConnectionsReached:
		return "CudaErrorMpsMaxConnectionsReached"
	case CudaErrorMpsClientTerminated:
		return "CudaErrorMpsClientTerminated"
	case CudaErrorCdpNotSupported:
		return "CudaErrorCdpNotSupported"
	case CudaErrorCdpVersionMismatch:
		return "CudaErrorCdpVersionMismatch"
	case CudaErrorStreamCaptureUnsupported:
		return "CudaErrorStreamCaptureUnsupported"
	case CudaErrorStreamCaptureInvalidated:
		return "CudaErrorStreamCaptureInvalidated"
	case CudaErrorStreamCaptureMerge:
		return "CudaErrorStreamCaptureMerge"
	case CudaErrorStreamCaptureUnmatched:
		return "CudaErrorStreamCaptureUnmatched"
	case CudaErrorStreamCaptureUnjoined:
		return "CudaErrorStreamCaptureUnjoined"
	case CudaErrorStreamCaptureIsolation:
		return "CudaErrorStreamCaptureIsolation"
	case CudaErrorStreamCaptureImplicit:
		return "CudaErrorStreamCaptureImplicit"
	case CudaErrorCapturedEvent:
		return "CudaErrorCapturedEvent"
	case CudaErrorStreamCaptureWrongThread:
		return "CudaErrorStreamCaptureWrongThread"
	case CudaErrorTimeout:
		return "CudaErrorTimeout"
	case CudaErrorGraphExecUpdateFailure:
		return "CudaErrorGraphExecUpdateFailure"
	case CudaErrorExternalDevice:
		return "CudaErrorExternalDevice"
	case CudaErrorInvalidClusterSize:
		return "CudaErrorInvalidClusterSize"
	case CudaErrorFunctionNotLoaded:
		return "CudaErrorFunctionNotLoaded"
	case CudaErrorInvalidResourceType:
		return "CudaErrorInvalidResourceType"
	case CudaErrorInvalidResourceConfiguration:
		return "CudaErrorInvalidResourceConfiguration"
	case CudaErrorUnknown:
		return "CudaErrorUnknown"
	default:
		return "Unknown error code: " + string(err)
	}
}

func SGEMM(order Order, transA, transB Transpose, m, n, k int, alpha float32, a []float32, lda int, b []float32, ldb int, beta float32, c []float32, ldc int) *Error {
	err := Error(C.cublas_sgemm(C.CUBLAS_LAYOUT(order), C.CUBLAS_TRANSPOSE(transA), C.CUBLAS_TRANSPOSE(transB), C.int(m), C.int(n), C.int(k), C.float(alpha),
		(*C.float)(&a[0]), C.int(lda), (*C.float)(&b[0]), C.int(ldb), C.float(beta), (*C.float)(&c[0]), C.int(ldc)))
	if err != 0 {
		return &err
	}
	return nil
}
