#!/usr/bin/env bash
set -e

# Parse arguments
if test $# -ne 3; then
    echo '--- FAIL  Usage:' $0 '[user] [pass] [database]'
    exit 1
fi

LOCATION=$(dirname "$0")
USER=$1
PASS=$2
DATABASE=$3

bash ${LOCATION}/test_cross_validation.sh
bash ${LOCATION}/test_initialization.sh ${USER} ${PASS} ${DATABASE}
bash ${LOCATION}/test_data_import_export.sh ${USER} ${PASS} ${DATABASE}