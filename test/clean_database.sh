#!/usr/bin/env bash

# Parse arguments
if test $# -ne 3; then
    echo '--- FAIL  Usage:' $0 '[user] [pass] [database]'
    exit 1
fi

USER=$1
PASS=$2
DATABASE=$3
TABLES=("items" "neighbors" "ratings" "recommends" "status")

# Drop all tables
for table in "${TABLES[@]}"
do
    mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; DROP TABLE ${table};"
done
