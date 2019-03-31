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

echo '=== RUN   Test Initialization'

# Build executable
go build -o ${LOCATION}/main ${LOCATION}/../app/main.go

# Initialize database
${LOCATION}/main init ${USER}:${PASS}@${DATABASE}

echo '--- PASS  Test Initialization'