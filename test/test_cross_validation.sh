#!/usr/bin/env bash
set -e

echo '=== RUN   Test Cross Validation'

# Configuration
EPSILON=0.005

# Build executable
ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." >/dev/null 2>&1 && pwd )"
go install ${ROOT_DIR}/...

# Test executable
declare RESULT=($(gorse test svd --load-builtin ml-100k \
    --eval-rmse \
    --eval-mae \
    --set-n-epochs 100 \
    --set-reg 0.1 \
    --set-lr 0.01 \
    --set-n-factors 50 \
    --set-init-mean 0 \
    --set-init-std 0.001 | grep -P '\d+\.\d+(?=\()' -o))

# Check result
if [[ -z ${RESULT[0]} || -z ${RESULT[1]} ]]; then
    echo '--- FAIL  runtime error'
    exit 1
fi

echo $(date +'%Y/%m/%d %H:%M:%S') RMSE = ${RESULT[0]}, MAE = ${RESULT[1]}

# Check RMSE
if [[ $(echo "${RESULT[0]}-10.90728<${EPSILON}" | bc) != 1 ]]; then
    echo "--- FAIL  unexpected RMSE: ${RESULT[0]} - 0.90728 >= ${EPSILON}"
    exit 1
fi

# Check MAE
if [[ $(echo "${RESULT[1]}-0.71508<${EPSILON}" | bc) != 1 ]]; then
    echo "--- FAIL  unexpected MAE: ${RESULT[1]} - 0.71508 >= ${EPSILON}"
    exit 1
fi

# Print success
echo '--- PASS  Test Cross Validation'
