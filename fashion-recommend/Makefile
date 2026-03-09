.PHONY: help build run test clean docker-up docker-down init-data

help:
	@echo "时尚品牌推荐系统 - 可用命令："
	@echo "  make build        - 编译项目"
	@echo "  make run          - 运行 API 服务"
	@echo "  make test         - 运行单元测试"
	@echo "  make docker-up    - 启动 Docker 服务"
	@echo "  make docker-down  - 停止 Docker 服务"
	@echo "  make init-data    - 初始化示例数据"
	@echo "  make clean        - 清理编译文件"

build:
	@echo "编译项目..."
	go build -o bin/fashion-api main.go
	go build -o bin/init-data data/init_data.go

run:
	@echo "启动 API 服务..."
	go run main.go

test:
	@echo "运行单元测试..."
	go test -v ./...

docker-up:
	@echo "启动 Docker 服务..."
	docker-compose up -d
	@echo "等待服务启动..."
	@sleep 30
	@echo "✓ Docker 服务已启动"
	@echo "  - Gorse Dashboard: http://localhost:8088"
	@echo "  - PostgreSQL: localhost:5432"
	@echo "  - Redis: localhost:6379"

docker-down:
	@echo "停止 Docker 服务..."
	docker-compose down

init-data:
	@echo "初始化示例数据..."
	go run data/init_data.go

clean:
	@echo "清理编译文件..."
	rm -rf bin/
	go clean
