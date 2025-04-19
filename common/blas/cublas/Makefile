CUDA_PATH = /usr/local/cuda-12.8
NVCC = $(CUDA_PATH)/bin/nvcc
AR = ar
CFLAGS = -O3
TARGET = libcublas_sgemm.a
SOURCES = cublas_sgemm.cu
OBJECTS = $(SOURCES:.cu=.o)

all: $(TARGET)

$(OBJECTS): $(SOURCES)
	$(NVCC) -c $< -o $@ $(CFLAGS)

$(TARGET): $(OBJECTS)
	$(AR) crs $@ $^

clean:
	rm -f $(OBJECTS) $(TARGET)

.PHONY: all clean
