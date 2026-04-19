COMMON_CFLAGS = -O3 -std=gnu11
HOST_ARCH := $(shell uname -m)

AMD64_CC ?= x86_64-linux-gnu-gcc
AMD64_CFLAGS = $(COMMON_CFLAGS) -mavx2
AMD64_BUILD_DIR = build/amd64
AMD64_SOURCES = bfloats_avx.c
AMD64_OBJECTS = $(addprefix $(AMD64_BUILD_DIR)/,$(AMD64_SOURCES:.c=.o))
AMD64_DEPENDENCES = $(AMD64_OBJECTS:.o=.d)
AMD64_ASSEMBLIES = $(addprefix $(AMD64_BUILD_DIR)/,$(AMD64_SOURCES:.c=.s))

ARM64_CC ?= aarch64-linux-gnu-gcc
ARM64_CFLAGS = $(COMMON_CFLAGS)
ARM64_BUILD_DIR = build/arm64
ARM64_SOURCES = bfloats_neon.c
ARM64_OBJECTS = $(addprefix $(ARM64_BUILD_DIR)/,$(ARM64_SOURCES:.c=.o))
ARM64_DEPENDENCES = $(ARM64_OBJECTS:.o=.d)
ARM64_ASSEMBLIES = $(addprefix $(ARM64_BUILD_DIR)/,$(ARM64_SOURCES:.c=.s))

RISCV64_CC ?= riscv64-linux-gnu-gcc
RISCV64_CFLAGS = $(COMMON_CFLAGS) -march=rv64gcv -mabi=lp64d
RISCV64_BUILD_DIR = build/riscv64
RISCV64_SOURCES = bfloats_rvv.c
RISCV64_OBJECTS = $(addprefix $(RISCV64_BUILD_DIR)/,$(RISCV64_SOURCES:.c=.o))
RISCV64_DEPENDENCES = $(RISCV64_OBJECTS:.o=.d)
RISCV64_ASSEMBLIES = $(addprefix $(RISCV64_BUILD_DIR)/,$(RISCV64_SOURCES:.c=.s))

ifeq ($(HOST_ARCH),x86_64)
	HOST_OBJECTS = $(AMD64_OBJECTS)
	HOST_ASSEMBLIES = $(AMD64_ASSEMBLIES)
else ifeq ($(HOST_ARCH),aarch64)
	HOST_OBJECTS = $(ARM64_OBJECTS)
	HOST_ASSEMBLIES = $(ARM64_ASSEMBLIES)
else ifeq ($(HOST_ARCH),riscv64)
	HOST_OBJECTS = $(RISCV64_OBJECTS)
	HOST_ASSEMBLIES = $(RISCV64_ASSEMBLIES)
endif

.PHONY: all asm amd64 asm-amd64 arm64 asm-arm64 riscv64 asm-riscv64 clean

all: $(HOST_OBJECTS)

asm: $(HOST_ASSEMBLIES)

amd64: $(AMD64_OBJECTS)

asm-amd64: $(AMD64_ASSEMBLIES)

arm64: $(ARM64_OBJECTS)

asm-arm64: $(ARM64_ASSEMBLIES)

riscv64: $(RISCV64_OBJECTS)

asm-riscv64: $(RISCV64_ASSEMBLIES)

$(AMD64_BUILD_DIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(AMD64_CC) $(AMD64_CFLAGS) -MMD -MP -c $< -o $@

$(AMD64_BUILD_DIR)/%.s: %.c
	@mkdir -p $(dir $@)
	$(AMD64_CC) $(AMD64_CFLAGS) -S $< -o $@

$(ARM64_BUILD_DIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(ARM64_CC) $(ARM64_CFLAGS) -MMD -MP -c $< -o $@

$(ARM64_BUILD_DIR)/%.s: %.c
	@mkdir -p $(dir $@)
	$(ARM64_CC) $(ARM64_CFLAGS) -S $< -o $@

$(RISCV64_BUILD_DIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(RISCV64_CC) $(RISCV64_CFLAGS) -MMD -MP -c $< -o $@

$(RISCV64_BUILD_DIR)/%.s: %.c
	@mkdir -p $(dir $@)
	$(RISCV64_CC) $(RISCV64_CFLAGS) -S $< -o $@

clean:
	rm -rf $(AMD64_BUILD_DIR) $(ARM64_BUILD_DIR) $(RISCV64_BUILD_DIR)

-include $(AMD64_DEPENDENCES) $(ARM64_DEPENDENCES) $(RISCV64_DEPENDENCES)
