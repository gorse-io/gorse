GO ?= go
GOAT ?= $(GO) tool goat
GOAT_TARGET_AVX ?= amd64
GOAT_TARGET_NEON ?= arm64
GOAT_TARGET_RVV ?= riscv64
GOAT_TARGET_LASX ?= loong64
LASX_OBJDUMP ?= loongarch64-linux-gnu-objdump
QEMU_AARCH64 ?= qemu-aarch64
QEMU_AARCH64_NEON_FLAGS ?= -cpu max
RISCV64_QEMU ?= qemu-riscv64
RISCV64_QEMU_FLAGS ?= -cpu max,v=true
LOONG64_QEMU ?= qemu-loongarch64
LOONG64_QEMU_FLAGS ?= -cpu max
TEST_NEON_BINARY ?= .test-neon.arm64.test
TEST_RVV_BINARY ?= .test-rvv.riscv64.test
TEST_LASX_BINARY ?= .test-lasx.loong64.test
PKG ?= .

.PHONY: avx neon rvv lasx regen test test-neon test-rvv test-lasx clean

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

lasx:
	@set -e; \
	tmp=$$(mktemp -d); \
	out=$$tmp/bfloats; \
	mkdir -p $$out; \
	OBJDUMP=$(LASX_OBJDUMP) $(GOAT) -o $$out --target $(GOAT_TARGET_LASX) -O 3 -m lasx src/bfloats_lasx.c; \
	cp $$out/bfloats_lasx.go $$out/bfloats_lasx.s .

regen: avx avx512 neon rvv lasx

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

test-lasx:
	@set -e; \
	trap '$(RM) ./$(TEST_LASX_BINARY)' EXIT; \
	GOOS=linux GOARCH=loong64 CGO_ENABLED=0 $(GO) test -c -o $(TEST_LASX_BINARY) .; \
	$(LOONG64_QEMU) $(LOONG64_QEMU_FLAGS) ./$(TEST_LASX_BINARY) -test.run '^TestEuclideanLASXMatchesScalar$$'

clean:
	$(RM) bfloats_avx.go bfloats_avx.s bfloats_avx512.go bfloats_avx512.s bfloats_neon.go bfloats_neon.s bfloats_rvv.go bfloats_rvv.s bfloats_lasx.go bfloats_lasx.s
