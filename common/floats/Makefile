APT ?= apt-get
SUDO ?= $(shell if command -v sudo >/dev/null 2>&1; then echo sudo; fi)
APT_INSTALL = $(SUDO) $(APT) install -y
GO ?= go
GOAT ?= goat
GOAT_TARGET_AVX ?= amd64
GOAT_TARGET_NEON ?= arm64
GOAT_TARGET_RVV ?= riscv64
CLANG_RESOURCE_DIR ?= $(shell clang -print-resource-dir)
GOAT_RVV_INCLUDE ?= $(CLANG_RESOURCE_DIR)/include
QEMU_X86_64 ?= qemu-x86_64
QEMU_X86_64_AVX_FLAGS ?= -cpu max,avx=on,fma=on
QEMU_X86_64_AVX512_FLAGS ?= -cpu max,avx=on,fma=on,avx512f=on,avx512dq=on
QEMU_AARCH64 ?= qemu-aarch64
QEMU_AARCH64_NEON_FLAGS ?= -cpu max
RISCV64_QEMU ?= qemu-riscv64
RISCV64_QEMU_FLAGS ?= -cpu max,v=true
TEST_AVX_BINARY ?= .test-avx.amd64.test
TEST_AVX512_BINARY ?= .test-avx512.amd64.test
TEST_NEON_BINARY ?= .test-neon.arm64.test
TEST_RVV_BINARY ?= .test-rvv.riscv64.test

.PHONY: avx avx-512 neon rvv deps test-avx test-avx512 test-neon test-rvv

avx:
	@set -e; \
	tmpdir="$$(mktemp -d)"; \
	outdir="$$tmpdir/floats"; \
	trap '$(RM) -r "$$tmpdir"' EXIT; \
	mkdir -p "$$outdir"; \
	perl -0pe 's/#include <immintrin.h>/#define _IMMINTRIN_H_INCLUDED\n#define __IMMINTRIN_H\n#include <xmmintrin.h>\n#include <emmintrin.h>\n#include <avxintrin.h>/' src/floats_avx.c > "$$tmpdir/floats_avx.c"; \
	$(GOAT) "$$tmpdir/floats_avx.c" -o "$$outdir" --target $(GOAT_TARGET_AVX) -O3 -mavx; \
	perl -0pi -e 's@^// source: .*floats_avx\.c$$@// source: src/floats_avx.c@m' "$$outdir/floats_avx.go" "$$outdir/floats_avx.s"; \
	cp "$$outdir/floats_avx.go" "$$outdir/floats_avx.s" .

avx512:
	$(GOAT) src/floats_avx512.c -o . --target $(GOAT_TARGET_AVX) -O3 -mavx -mfma -mavx512f

neon:
	$(GOAT) src/floats_neon.c -o . --target $(GOAT_TARGET_NEON) -O3

rvv:
	$(GOAT) src/floats_rvv.c -o . --target $(GOAT_TARGET_RVV) -O3 -march=rv64imafdv -e=-isystem -e=$(GOAT_RVV_INCLUDE)

deps:
	$(SUDO) $(APT) update
	$(APT_INSTALL) \
		gcc-x86-64-linux-gnu \
		gcc-aarch64-linux-gnu \
		gcc-riscv64-linux-gnu \
		qemu-user

test-avx:
	@set -e; \
	trap '$(RM) ./$(TEST_AVX_BINARY)' EXIT; \
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) test -c -o $(TEST_AVX_BINARY) .; \
	$(QEMU_X86_64) $(QEMU_X86_64_AVX_FLAGS) ./$(TEST_AVX_BINARY) -test.run '^TestAVX$$'

test-avx512:
	@set -e; \
	trap '$(RM) ./$(TEST_AVX512_BINARY)' EXIT; \
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) test -c -o $(TEST_AVX512_BINARY) .; \
	$(QEMU_X86_64) $(QEMU_X86_64_AVX512_FLAGS) ./$(TEST_AVX512_BINARY) -test.run '^TestAVX512$$'

test-neon:
	@set -e; \
	trap '$(RM) ./$(TEST_NEON_BINARY)' EXIT; \
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) test -c -o $(TEST_NEON_BINARY) .; \
	$(QEMU_AARCH64) $(QEMU_AARCH64_NEON_FLAGS) ./$(TEST_NEON_BINARY) -test.run '^TestASIMD$$'

test-rvv:
	@set -e; \
	trap '$(RM) ./$(TEST_RVV_BINARY)' EXIT; \
	GOOS=linux GOARCH=riscv64 CGO_ENABLED=0 $(GO) test -c -o $(TEST_RVV_BINARY) .; \
	$(RISCV64_QEMU) $(RISCV64_QEMU_FLAGS) ./$(TEST_RVV_BINARY) -test.run '^TestV$$'
