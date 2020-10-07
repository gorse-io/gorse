#!/usr/bin/env bash
set -e

# Build executable
ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." >/dev/null 2>&1 && pwd )"
go install ${ROOT_DIR}/...

# Create temp files
TMP_DB=$(mktemp -d /tmp/test_server_XXXXXX)
TMP_CONFIG=$(mktemp /tmp/test_server_XXXXXX)

# Create config file
cp ${ROOT_DIR}/example/config/config.toml ${TMP_CONFIG}
sed -i "s:\"database\":\"${TMP_DB}\":g" ${TMP_CONFIG}

# Import ml-100k data
gorse import-items ${TMP_DB} ~/.gorse/dataset/ml-1m/movies.dat --sep '::' --label 2 --label-sep $'|'
gorse import-feedback ${TMP_DB} ~/.gorse/dataset/ml-1m/ratings.dat --sep '::'

# Start cmd
gorse serve -c ${TMP_CONFIG}
