#!/bin/bash

# 数据库初始化脚本

DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-gorse}
DB_PASSWORD=${DB_PASSWORD:-gorse_pass}
DB_NAME=${DB_NAME:-gorse}

echo "初始化数据库..."
echo "数据库: $DB_HOST:$DB_PORT/$DB_NAME"

# 执行 SQL 迁移文件
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f database/migrations/001_create_ai_tables.sql

if [ $? -eq 0 ]; then
    echo "✓ 数据库初始化成功"
else
    echo "✗ 数据库初始化失败"
    exit 1
fi
