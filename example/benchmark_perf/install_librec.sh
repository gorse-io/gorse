#!/usr/bin/env bash

echo 'download librec'

wget https://cdn.sine-x.com/backups/librec-3.0.0.zip -P ~/.gorse/

unzip ~/.gorse/librec-3.0.0.zip -d ~/.gorse/

rm ~/.gorse/librec-3.0.0.zip

echo 'complete'