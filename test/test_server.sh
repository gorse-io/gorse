#!/usr/bin/env bash
set -e

# Build executable
ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." >/dev/null 2>&1 && pwd )"
go install ${ROOT_DIR}/...

# Create temp files
TMP_DB=$(mktemp /tmp/test_server.XXXXXX)
TMP_CONFIG=$(mktemp /tmp/test_server.XXXXXX)

# Create config file
cp ${ROOT_DIR}/example/file_config/config.toml ${TMP_CONFIG}
sed -i "s:database:${TMP_DB}:g" ${TMP_CONFIG}

# Import ml-100k data
gorse import-feedback ${TMP_DB} ~/.gorse/dataset/ml-100k/u1.base --sep $'\t'

# Start cmd
gorse serve -c ${TMP_CONFIG} &

# Test cmd
python3 ${ROOT_DIR}/test/test_server.py 127.0.0.1 8080

# Kill cmd
pkill gorse

echo '--- PASS  Test Server'