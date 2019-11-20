#!/usr/bin/env bash
set -e

echo '=== RUN   Test Import Ratings'

# Create temp files
TMP_FEEDBACK_DB=$(mktemp /tmp/test_data_import_export.XXXXXX)
TMP_ITEMS_DB=$(mktemp /tmp/test_data_import_export.XXXXXX)
TMP_FEEDBACK_CSV=$(mktemp /tmp/test_data_import_export.XXXXXX)
TMP_ITEMS_CSV=$(mktemp /tmp/test_data_import_export.XXXXXX)

# Build executable
ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." >/dev/null 2>&1 && pwd )"
go install ${ROOT_DIR}/...

# Import/export feedback
gorse import-feedback ${TMP_FEEDBACK_DB} ${ROOT_DIR}/example/file_data/feedback_explicit.csv
gorse export-feedback ${TMP_FEEDBACK_DB} ${TMP_FEEDBACK_CSV}

# Import/export items
gorse import-items ${TMP_ITEMS_DB} ${ROOT_DIR}/example/file_data/items_id_only.csv
gorse export-items ${TMP_ITEMS_DB} ${TMP_ITEMS_CSV}

# MD5 sum
FEEDBACK_MD5_IMPORT=$(md5sum ${ROOT_DIR}/example/file_data/feedback_explicit.csv | awk '{ print $1 }')
FEEDBACK_MD5_EXPORT=$(md5sum ${TMP_FEEDBACK_CSV} | awk '{ print $1 }')
ITEMS_MD5_IMPORT=$(md5sum ${ROOT_DIR}/example/file_data/items_id_only.csv | awk '{ print $1 }')
ITEMS_MD5_EXPORT=$(md5sum ${TMP_ITEMS_CSV} | awk '{ print $1 }')

# Compare MD5
if [[ ${FEEDBACK_MD5_EXPORT} != ${FEEDBACK_MD5_IMPORT} ]]; then
    echo '--- FAIL  import/export feedback don'\''t match'
    exit 1
fi

if [[ ${ITEMS_MD5_EXPORT} != ${ITEMS_MD5_IMPORT} ]]; then
    echo '--- FAIL  import/export items don'\''t match'
    exit 1
fi

echo '--- PASS  Test Data Import/Export'