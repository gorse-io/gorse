package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"fashion-recommend/ai"
	"fashion-recommend/api"
	"fashion-recommend/database"
	
	_ "github.com/lib/pq"
)

func main() {
	// 解析命令行参数
	gorseEndpoint := flag.String("gorse", getEnv("GORSE_ENDPOINT", "http://localhost:8088"), "Gorse endpoint URL")
	gorseAPIKey := flag.String("apikey", getEnv("GORSE_API_KEY", ""), "Gorse API key")
	port := flag.String("port", getEnv("PORT", "5000"), "API server port")
	dbConnStr := flag.String("db", getEnv("DATABASE_URL", "host=localhost port=5432 user=gorse password=gorse_pass dbname=gorse sslmode=disable"), "Database connection string")
	flag.Parse()

	fmt.Println("========================================")
	fmt.Println("时尚品牌推荐系统 API 服务")
	fmt.Println("========================================")
	fmt.Printf("Gorse Endpoint: %s\n", *gorseEndpoint)
	fmt.Printf("API Port: %s\n", *port)
	fmt.Println("========================================")

	// 连接数据库
	fmt.Println("连接数据库...")
	db, err := database.NewDB(*dbConnStr)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer db.Close()
	fmt.Println("✓ 数据库连接成功")

	// AI 配置
	aiConfig := ai.Config{
		APIKey:      getEnv("AI_API_KEY", "sk-7fe1db8b36914b26b64555375d58ab7d"),
		BaseURL:     getEnv("AI_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		Model:       getEnv("AI_MODEL", "qwen-plus"),
		MaxTokens:   2000,
		Temperature: 0.7,
	}
	fmt.Printf("AI Model: %s\n", aiConfig.Model)
	fmt.Println("========================================")

	// 创建并启动服务器
	server := api.NewServer(*gorseEndpoint, *gorseAPIKey, aiConfig, db)
	addr := fmt.Sprintf(":%s", *port)

	fmt.Printf("\n✓ API 服务启动成功！\n")
	fmt.Printf("访问地址: http://localhost:%s\n", *port)
	fmt.Printf("健康检查: http://localhost:%s/health\n\n", *port)

	if err := server.Run(addr); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
