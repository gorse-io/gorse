GO ?= go
GOAT ?= goat
GOAT_TARGET_AVX ?= amd64
GOAT_TARGET_NEON ?= arm64
GOAT_TARGET_RVV ?= riscv64
QEMU_AARCH64 ?= qemu-aarch64
QEMU_AARCH64_NEON_FLAGS ?= -cpu max
RISCV64_QEMU ?= qemu-riscv64
RISCV64_QEMU_FLAGS ?= -cpu max,v=true
TEST_NEON_BINARY ?= .test-neon.arm64.test
TEST_RVV_BINARY ?= .test-rvv.riscv64.test
PKG ?= .

.PHONY: avx neon rvv regen test test-neon test-rvv clean

avx:
	@set -e; \
	tmp=$$(mktemp -d); \
	out=$$tmp/bfloats; \
	mkdir -p $$out; \
	$(GOAT) -o $$out --target $(GOAT_TARGET_AVX) -O 3 -m avx2 src/bfloats_avx.c; \
	cp $$out/bfloats_avx.go $$out/bfloats_avx.s .

avx512:
	@set -e; \
	tmp=$$(mktemp -d); \
	out=$$tmp/bfloats; \
	mkdir -p $$out; \
	$(GOAT) -o $$out --target $(GOAT_TARGET_AVX) -O 3 -m avx -m avx512f -m avx512bw src/bfloats_avx512.c; \
	cp $$out/bfloats_avx512.go $$out/bfloats_avx512.s .

neon:
	@set -e; \
	tmp=$$(mktemp -d); \
	out=$$tmp/bfloats; \
	mkdir -p $$out; \
	$(GOAT) -o $$out --target $(GOAT_TARGET_NEON) -O 3 src/bfloats_neon.c; \
	cp $$out/bfloats_neon.go $$out/bfloats_neon.s .

rvv:
	@set -e; \
	tmp=$$(mktemp -d); \
	out=$$tmp/bfloats; \
	mkdir -p $$out; \
	$(GOAT) -o $$out --target $(GOAT_TARGET_RVV) -O 3 -e=-march=rv64gv -e=-mabi=lp64d src/bfloats_rvv.c; \
	cp $$out/bfloats_rvv.go $$out/bfloats_rvv.s .

regen: avx avx512 neon rvv

test:
	$(GO) test $(PKG)

test-neon:
	@set -e; \
	trap '$(RM) ./$(TEST_NEON_BINARY)' EXIT; \
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) test -c -o $(TEST_NEON_BINARY) .; \
	$(QEMU_AARCH64) $(QEMU_AARCH64_NEON_FLAGS) ./$(TEST_NEON_BINARY) -test.run '^TestEuclideanNEONMatchesScalar$$'

test-rvv:
	@set -e; \
	trap '$(RM) ./$(TEST_RVV_BINARY)' EXIT; \
	GOOS=linux GOARCH=riscv64 CGO_ENABLED=0 $(GO) test -c -o $(TEST_RVV_BINARY) .; \
	$(RISCV64_QEMU) $(RISCV64_QEMU_FLAGS) ./$(TEST_RVV_BINARY) -test.run '^TestEuclideanVMatchesScalar$$'

clean:
	$(RM) bfloats_avx.go bfloats_avx.s bfloats_avx512.go bfloats_avx512.s bfloats_neon.go bfloats_neon.s bfloats_rvv.go bfloats_rvv.s
