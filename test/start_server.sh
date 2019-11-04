#!/usr/bin/env bash
set -e

# Build executable
go install ../...

# Create temp files
TMP_DB=$(mktemp /tmp/test_server.XXXXXX)
TMP_CONFIG=$(mktemp /tmp/test_server.XXXXXX)

# Create config file
cp ../example/file_config/config.toml ${TMP_CONFIG}
sed -i "s:gorse.db:${TMP_DB}:g" ${TMP_CONFIG}

# Import ml-100k data
gorse import-feedback ${TMP_DB} ~/.gorse/dataset/ml-100k/u1.base --sep $'\t'

# Start server
gorse serve -c ${TMP_CONFIG}
