COMMON_SOURCES = munit.c floats_test.c
COMMON_CFLAGS = -O3 -std=gnu11
LDLIBS = -lm
HOST_ARCH := $(shell uname -m)

HOST_SOURCES = $(COMMON_SOURCES)
HOST_ARCH_CFLAGS =

ifeq ($(HOST_ARCH),x86_64)
	HOST_SOURCES += floats_avx.c floats_avx512.c
	HOST_ARCH_CFLAGS += -mavx -mavx512f -mavx512dq
else ifeq ($(HOST_ARCH),aarch64)
	HOST_SOURCES += floats_neon.c floats_sve2.c
	HOST_ARCH_CFLAGS += -march=armv8-a+sve
else ifeq ($(HOST_ARCH),riscv64)
	HOST_SOURCES += floats_rvv.c
	HOST_ARCH_CFLAGS += -march=rv64gcv -mabi=lp64d
endif

HOST_BUILD_DIR = build/host
HOST_OBJECTS = $(addprefix $(HOST_BUILD_DIR)/,$(HOST_SOURCES:.c=.o))
HOST_DEPENDENCES = $(HOST_OBJECTS:.o=.d)
HOST_CFLAGS = $(COMMON_CFLAGS) $(HOST_ARCH_CFLAGS)
EXECUTE = floats_test

AMD64_CC ?= x86_64-linux-gnu-gcc
AMD64_QEMU ?= qemu-x86_64
AMD64_QEMU_FLAGS ?= -cpu max
AMD64_BUILD_DIR = build/amd64
AMD64_SOURCES = $(COMMON_SOURCES) floats_avx.c floats_avx512.c
AMD64_OBJECTS = $(addprefix $(AMD64_BUILD_DIR)/,$(AMD64_SOURCES:.c=.o))
AMD64_DEPENDENCES = $(AMD64_OBJECTS:.o=.d)
AMD64_EXECUTE = floats_test_amd64
AMD64_CFLAGS = $(COMMON_CFLAGS) -mavx -mavx512f -mavx512dq

RISCV64_CC ?= riscv64-linux-gnu-gcc
RISCV64_QEMU ?= qemu-riscv64
RISCV64_QEMU_FLAGS ?= -cpu max
RISCV64_BUILD_DIR = build/riscv64
RISCV64_SOURCES = $(COMMON_SOURCES) floats_rvv.c
RISCV64_OBJECTS = $(addprefix $(RISCV64_BUILD_DIR)/,$(RISCV64_SOURCES:.c=.o))
RISCV64_DEPENDENCES = $(RISCV64_OBJECTS:.o=.d)
RISCV64_EXECUTE = floats_test_riscv64
RISCV64_CFLAGS = $(COMMON_CFLAGS) -march=rv64gcv -mabi=lp64d
RISCV64_LDFLAGS = -static

ARM64_CC ?= aarch64-linux-gnu-gcc
ARM64_SYSROOT ?=
ARM64_QEMU ?= qemu-aarch64
ARM64_QEMU_FLAGS ?= -cpu max
ARM64_BUILD_DIR = build/arm64
ARM64_SOURCES = $(COMMON_SOURCES) floats_neon.c floats_sve2.c
ARM64_OBJECTS = $(addprefix $(ARM64_BUILD_DIR)/,$(ARM64_SOURCES:.c=.o))
ARM64_DEPENDENCES = $(ARM64_OBJECTS:.o=.d)
ARM64_EXECUTE = floats_test_arm64
ARM64_CFLAGS = $(COMMON_CFLAGS) -march=armv8-a+sve
ARM64_LDFLAGS = -static
ifneq ($(ARM64_SYSROOT),)
ARM64_CFLAGS += --sysroot=$(ARM64_SYSROOT)
ARM64_LDFLAGS += --sysroot=$(ARM64_SYSROOT)
endif

$(EXECUTE): $(HOST_OBJECTS)
	$(CC) $(HOST_OBJECTS) $(LDLIBS) -o $(EXECUTE)

test: $(EXECUTE)
	./$(EXECUTE)

$(AMD64_EXECUTE): $(AMD64_OBJECTS)
	$(AMD64_CC) $(AMD64_OBJECTS) $(LDLIBS) -o $(AMD64_EXECUTE)

# Unlike `make test`, this target runs the AMD64 binary under qemu.
test-amd64: $(AMD64_EXECUTE)
	@echo "Running AMD64 tests under qemu (make test runs natively on the host)"
	$(AMD64_QEMU) $(AMD64_QEMU_FLAGS) ./$(AMD64_EXECUTE)

$(RISCV64_EXECUTE): $(RISCV64_OBJECTS)
	$(RISCV64_CC) $(RISCV64_OBJECTS) $(RISCV64_LDFLAGS) $(LDLIBS) -o $(RISCV64_EXECUTE)

test-riscv64: $(RISCV64_EXECUTE)
	$(RISCV64_QEMU) $(RISCV64_QEMU_FLAGS) ./$(RISCV64_EXECUTE)

$(ARM64_EXECUTE): $(ARM64_OBJECTS)
	$(ARM64_CC) $(ARM64_OBJECTS) $(ARM64_LDFLAGS) $(LDLIBS) -o $(ARM64_EXECUTE)

test-arm64: $(ARM64_EXECUTE)
	$(ARM64_QEMU) $(ARM64_QEMU_FLAGS) ./$(ARM64_EXECUTE)

$(HOST_BUILD_DIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(CC) $(HOST_CFLAGS) -MMD -MP -c $< -o $@

$(AMD64_BUILD_DIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(AMD64_CC) $(AMD64_CFLAGS) -MMD -MP -c $< -o $@

$(RISCV64_BUILD_DIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(RISCV64_CC) $(RISCV64_CFLAGS) -MMD -MP -c $< -o $@

$(ARM64_BUILD_DIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(ARM64_CC) $(ARM64_CFLAGS) -MMD -MP -c $< -o $@

clean:
	rm -rf $(HOST_BUILD_DIR) $(AMD64_BUILD_DIR) $(RISCV64_BUILD_DIR) $(ARM64_BUILD_DIR) $(EXECUTE) $(AMD64_EXECUTE) $(RISCV64_EXECUTE) $(ARM64_EXECUTE)

-include $(HOST_DEPENDENCES) $(AMD64_DEPENDENCES) $(RISCV64_DEPENDENCES) $(ARM64_DEPENDENCES)
