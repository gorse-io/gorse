#!/usr/bin/env bash

# Parse arguments
if test $# -ne 3; then
    echo '--- FAIL  Usage:' $0 '[user] [pass] [database]'
    exit 1
fi

LOCATION=$(dirname "$0")
USER=$1
PASS=$2
DATABASE=$3

echo '=== RUN   Test Data Import/Export'

# Build executable
go build -o ${LOCATION}/main ${LOCATION}/../app/main.go

# Download ml0-100k
wget https://cdn.sine-x.com/datasets/movielens/ml-100k.zip -P ${LOCATION}

# Extract files
unzip ${LOCATION}/ml-100k.zip

# Remove files
rm -rf ${LOCATION}/ml-100k
rm ${LOCATION}/ml-100k.zip
rm ${LOCATION}/main

echo '--- PASS  Test Data Import/Export'