#!/usr/bin/env bash
set -e

echo '=== RUN   Test Cross Validation'

# Configuration
EPSILON=0.005

# Build executable
ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." >/dev/null 2>&1 && pwd )"
go install ${ROOT_DIR}/...

# Test executable
declare RESULT=($(gorse test als --load-builtin ml-100k \
    --splitter k-fold \
    --eval-precision \
    --eval-recall \
    --eval-ndcg \
    --set-n-epochs 10 \
    --set-reg 0.015 \
    --set-n-factors 20 \
    --n-negative 0 | grep -P '\d+\.\d+(?=\()' -o))

# Check result
if [[ -z ${RESULT[0]} || -z ${RESULT[1]} ]]; then
    echo '--- FAIL  runtime error'
    exit 1
fi

echo $(date +'%Y/%m/%d %H:%M:%S') Precision@10 = ${RESULT[0]}, Recall@10 = ${RESULT[1]}, NDCG@10 = ${RESULT[2]}

# Check precision
if [[ $(echo "${RESULT[0]}-0.32083>-${EPSILON}" | bc) != 1 ]]; then
    echo "--- FAIL  unexpected Precision@10: ${RESULT[0]} - 0.32083 <= -${EPSILON}"
    exit 1
fi

# Check recall
if [[ $(echo "${RESULT[1]}-0.20906>-${EPSILON}" | bc) != 1 ]]; then
    echo "--- FAIL  unexpected Recall@10: ${RESULT[1]} - 0.20906 <= -${EPSILON}"
    exit 1
fi

# Check ndcg
if [[ $(echo "${RESULT[2]}-0.37643>-${EPSILON}" | bc) != 1 ]]; then
    echo "--- FAIL  unexpected NDCG@10: ${RESULT[2]} - 0.37643 <= -${EPSILON}"
    exit 1
fi

# Print success
echo '--- PASS  Test Cross Validation'
