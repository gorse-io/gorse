#!/usr/bin/env bash
echo '=== RUN   Test Data Import/Export'

LOCATION=$(dirname "$0")

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