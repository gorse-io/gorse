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
TABLES=("items" "neighbors" "ratings" "recommends" "status")

echo '=== RUN   Test Initialization'

# Build executable
go build -o ${LOCATION}/main ${LOCATION}/../app/main.go

# Initialize database
${LOCATION}/main init ${USER}:${PASS}@/${DATABASE}

# Check tables
declare RESULT=($(mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; show tables;" | grep -P '^[a-z]+' -o))

for ((i=1; i<=${#TABLES[@]}; i++))
do
	if [[ ${RESULT[$i]} != ${TABLES[$i]} ]]; then
        echo "--- FAIL  require table: ${TABLES[$i]}"
        exit 1
    fi
done

# Drop all tables
for table in "${TABLES[@]}"
do
    mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; DROP TABLE ${table};"
done

# Remove executable
rm ${LOCATION}/main

echo '--- PASS  Test Initialization'