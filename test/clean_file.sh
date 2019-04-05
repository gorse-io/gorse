#!/usr/bin/env bash

LOCATION=$(dirname "$0")

# Remove files
rm -rf ${LOCATION}/ml-100k
rm ${LOCATION}/ml-100k.zip
rm ${LOCATION}/gorse
