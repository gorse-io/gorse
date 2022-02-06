#!/bin/sh

set -e

clang -S -O3 -mavx2 -mfma -masm=intel -mno-red-zone -mstackrealign -mllvm -inline-threshold=1000 \
  -fno-asynchronous-unwind-tables -fno-exceptions -fno-rtti -c src/floats_amd64.c -o src/floats_amd64.s

c2goasm -a -f src/floats_amd64.s floats_amd64.s

rm src/floats_amd64.s