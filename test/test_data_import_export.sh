#!/usr/bin/env bash
set -e

echo '=== RUN   Test Import Ratings'

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

# Download ml0-100k
wget https://cdn.sine-x.com/datasets/movielens/ml-100k.zip -P ${LOCATION}

# Extract files
unzip ${LOCATION}/ml-100k.zip -d ${LOCATION}

# Initialize database
${LOCATION}/gorse init ${USER}:${PASS}@/${DATABASE}

# Import ratings to database
${LOCATION}/gorse data ${USER}:${PASS}@/${DATABASE} --import-ratings-csv ${LOCATION}/ml-100k/u.data

# Check tables
NUM_RATING=$(mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; select count(*) from ratings;" | grep -P '\d+' -o)
NUM_ITEM=$(mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; select count(*) from items;" | grep -P '\d+' -o)

if [[ ${NUM_RATING} != 100000 ]]; then
    echo "--- FAIL  the number of ratings (${NUM_RATING}) doesn't match"
    exit 1
else
    echo $(date +'%Y/%m/%d %H:%M:%S') found ${NUM_RATING} ratings in database
fi

if [[ ${NUM_ITEM} != 1682 ]]; then
    echo "--- FAIL  the number of items (${NUM_ITEM}) doesn't match"
    exit 1
else
    echo $(date +'%Y/%m/%d %H:%M:%S') found ${NUM_ITEM} items in database
fi

# Clear items
mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; delete from items;"

echo '=== RUN   Test Import Items'

# Import items
${LOCATION}/gorse data ${USER}:${PASS}@/${DATABASE} --import-items-csv ${LOCATION}/ml-100k/u.item

# Check tables
NUM_ITEM=$(mysql -u ${USER} -p${PASS} -e "use ${DATABASE}; select count(*) from items;" | grep -P '\d+' -o)

if [[ ${NUM_ITEM} != 1682 ]]; then
    echo "--- FAIL  the number of items (${NUM_ITEM}) doesn't match"
    exit 1
else
    echo $(date +'%Y/%m/%d %H:%M:%S') found ${NUM_ITEM} items in database
fi

# Clear
bash ${LOCATION}/clean_file.sh
bash ${LOCATION}/clean_database.sh ${USER} ${PASS} ${DATABASE}

echo '--- PASS  Test Data Import/Export'