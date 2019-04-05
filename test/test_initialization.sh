#!/usr/bin/env bash
set -e

echo '=== RUN   Test Initialization'

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

# Build executable
go build -o ${LOCATION}/gorse ${LOCATION}/../cmd/gorse.go

# Initialize database
${LOCATION}/gorse init ${USER}:${PASS}@/${DATABASE}

# Check tables
declare RESULT=($(mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; show tables;" | grep -P '^[a-z]+' -o))

for ((i=0; i<${#TABLES[@]}; i++))
do
	if [[ ${RESULT[$i]} != ${TABLES[$i]} ]]; then
        echo "--- FAIL  require table: ${TABLES[$i]}"
        exit 1
    else
       echo $(date +'%Y/%m/%d %H:%M:%S') found table ${RESULT[$i]}
    fi
done

# Clear
bash ${LOCATION}/clean_file.sh
bash ${LOCATION}/clean_database.sh ${USER} ${PASS} ${DATABASE}

echo '--- PASS  Test Initialization'