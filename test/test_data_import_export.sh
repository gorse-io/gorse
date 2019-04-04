#!/usr/bin/env bash
set -e

echo '=== RUN   Test Data Import/Export'

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
go build -o ${LOCATION}/main ${LOCATION}/../app/main.go

# Download ml0-100k
wget https://cdn.sine-x.com/datasets/movielens/ml-100k.zip -P ${LOCATION}

# Extract files
unzip ${LOCATION}/ml-100k.zip -d ${LOCATION}

# Initialize database
${LOCATION}/main init ${USER}:${PASS}@/${DATABASE}

# Import ratings to database
${LOCATION}/main data ${USER}:${PASS}@/${DATABASE} --import-ratings-csv ${LOCATION}/ml-100k/u.data

# Check tables
NUM_RATING=$(mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; select count(*) from ratings;" | grep -P '\d+' -o)
NUM_ITEM=$(mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; select count(*) from items;" | grep -P '\d+' -o)

if [[ ${NUM_RATING} != 100000 ]]; then
    echo "--- FAIL  the number of ratings (${NUM_RATING}) doesn't match"
    exit 1
fi

if [[ ${NUM_ITEM} != 1682 ]]; then
    echo "--- FAIL  the number of items (${NUM_ITEM}) doesn't match"
    exit 1
fi

# Clear items
mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; delete from items;"

# Import items
${LOCATION}/main data ${USER}:${PASS}@/${DATABASE} --import-items-csv ${LOCATION}/ml-100k/u.item

# Check tables
NUM_ITEM=$(mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; select count(*) from items;" | grep -P '\d+' -o)

if [[ ${NUM_ITEM} != 1682 ]]; then
    echo "--- FAIL  the number of items (${NUM_ITEM}) doesn't match"
    exit 1
fi

# Drop all tables
for table in "${TABLES[@]}"
do
    mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; DROP TABLE ${table};"
done

# Remove files
rm -rf ${LOCATION}/ml-100k
rm ${LOCATION}/ml-100k.zip
rm ${LOCATION}/main

echo '--- PASS  Test Data Import/Export'