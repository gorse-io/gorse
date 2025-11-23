#!/bin/bash
set -e

# Download config
if [ ! -f ./config.toml ]; then
    wget https://github.com/gorse-io/gorse/raw/refs/heads/master/client/config.toml
fi

# Create docker-compose.yml
if [ ! -f ./docker-compose.yml ]; then
    cat > docker-compose.yml <<EOF
version: '3'
services:
    master:
        image: zhenghaoz/gorse-in-one:latest
        ports:
            - 8088:8088
        volumes:
            - ./config.toml:/etc/gorse/config.toml
        environment:
            - GORSE_DATA_STORE=sqlite:///var/lib/gorse/data.sqlite3
            - GORSE_CACHE_STORE=sqlite:///var/lib/gorse/cache.sqlite3
        command: ["-c", "/etc/gorse/config.toml", "--cache-path", "/var/lib/gorse"]
EOF
fi

# Start Gorse
if [ "$(docker compose ps -q master | xargs docker inspect -f '{{.State.Running}}')" != "true" ]; then
    docker compose up -d
fi

# Download MovieLens 100k dataset
if [ ! -f ./ml-100k.bin ]; then
    wget https://cdn.gorse.io/example/ml-100k.bin.gz
    gzip -d ml-100k.bin.gz
fi

# Wait for Gorse to be ready
max_attempts=6
attempt=0
until curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:8088/api/health/ready" | grep -q "200" || [ $attempt -ge $max_attempts ]; do
    echo "Waiting for Gorse to be ready... (attempt $((attempt+1))/$max_attempts)"
    sleep 5
    attempt=$((attempt+1))
done
if [ $attempt -ge $max_attempts ]; then
    echo "Warning: Maximum attempts reached. Gorse may not be ready."
fi

# Import MovieLens 100k dataset
curl -X POST "http://localhost:8088/api/restore" --data-binary "@./ml-100k.bin"

# Restart Gorse to apply changes
docker compose restart master
max_attempts=6
attempt=0
until curl -s "http://localhost:8088/api/dashboard/tasks" | grep -q "Train Collaborative Filtering Model" || [ $attempt -ge $max_attempts ]; do
    echo "Waiting for recommendation... (attempt $((attempt+1))/$max_attempts)"
    sleep 10
    attempt=$((attempt+1))
done
if [ $attempt -ge $max_attempts ]; then
    echo "Warning: Maximum attempts reached. Recommendation may not be ready."
fi
