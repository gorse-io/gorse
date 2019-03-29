#!/usr/bin/env bash
echo '=== RUN   Test Initialization'

LOCATION=$(dirname "$0")

# Build executable
go build -o ${LOCATION}/main ${LOCATION}/../app/main.go

echo '--- PASS  Test Initialization'