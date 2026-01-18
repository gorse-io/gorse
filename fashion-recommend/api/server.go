package api

import (
	"net/http"
	"strconv"

	"fashion-recommend/ai"
	"fashion-recommend/auth"
	"fashion-recommend/client"
	"fashion-recommend/database"
	"fashion-recommend/models"
	"fashion-recommend/traits"

	"github.com/gin-gonic/gin"
)

// Server API 服务器
type Server struct {
	gorseClient     *client.GorseClient
	authService     *auth.AuthService
	aiService       *ai.Service
	db              *database.DB
	traitExtractor  *traits.Extractor
	gorseSync       *traits.GorseSync
	router          *gin.Engine
}

// NewServer 创建 API 服务器
func NewServer(gorseEndpoint, gorseAPIKey string, aiConfig ai.Config, db *database.DB) *Server {
	gorseClient := client.NewGorseClient(gorseEndpoint, gorseAPIKey)
	aiService := ai.NewService(aiConfig)
	
	s := &Server{
		gorseClient:    gorseClient,
		authService:    auth.NewAuthService(),
		aiService:      aiService,
		db:             db,
		traitExtractor: traits.NewExtractor(aiService, db),
		gorseSync:      traits.NewGorseSync(gorseClient, db),
		router:         gin.Default(),
	}
	s.setupRoutes()
	return s
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	api := s.router.Group("/api")
	{
		// 认证相关
		api.POST("/auth/register", s.register)
		api.POST("/auth/login", s.login)
		api.POST("/auth/logout", s.logout)
		api.GET("/auth/me", s.getCurrentUser)

		// AI 相关
		api.POST("/ai/chat", s.aiChat)
		api.POST("/ai/explain", s.aiExplainRecommendation)
		api.POST("/ai/style-advice", s.aiStyleAdvice)

		// 评论相关
		api.POST("/comments", s.createComment)
		api.GET("/comments/:item_id", s.getComments)
		api.POST("/comments/:comment_id/like", s.likeComment)
		api.DELETE("/comments/:comment_id/like", s.unlikeComment)

		// 商品点赞相关
		api.POST("/products/:item_id/like", s.likeProduct)
		api.DELETE("/products/:item_id/like", s.unlikeProduct)
		api.GET("/products/:item_id/likes", s.getProductLikes)
		api.POST("/products/likes/batch", s.getBatchProductLikes)

		// 用户相关
		api.POST("/user", s.createUser)
		api.GET("/user/:user_id", s.getUser)
		api.POST("/users", s.createUsers)

		// 商品相关
		api.POST("/item", s.createItem)
		api.GET("/item/:item_id", s.getItem)
		api.POST("/items", s.createItems)

		// 反馈相关
		api.POST("/feedback", s.createFeedback)

		// 推荐相关
		api.GET("/recommend/:user_id", s.getRecommend)
		api.GET("/similar/:item_id", s.getSimilarItems)
	}

	// 健康检查
	s.router.GET("/health", s.healthCheck)

	// 静态文件服务 - 服务前端构建的文件
	s.router.Static("/assets", "./frontend/dist/assets")
	s.router.Static("/images", "./public/images")
	s.router.StaticFile("/", "./frontend/dist/index.html")
	
	// 所有未匹配的路由都返回 index.html（支持前端路由）
	s.router.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})
}

// createUser 创建用户
func (s *Server) createUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.gorseClient.InsertUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user created successfully", "user_id": user.UserId})
}

// getUser 获取用户
func (s *Server) getUser(c *gin.Context) {
	userId := c.Param("user_id")

	user, err := s.gorseClient.GetUser(userId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// createUsers 批量创建用户
func (s *Server) createUsers(c *gin.Context) {
	var users []models.User
	if err := c.ShouldBindJSON(&users); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.gorseClient.InsertUsers(users); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "users created successfully", "count": len(users)})
}

// createItem 创建商品
func (s *Server) createItem(c *gin.Context) {
	var item models.Item
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.gorseClient.InsertItem(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "item created successfully", "item_id": item.ItemId})
}

// getItem 获取商品
func (s *Server) getItem(c *gin.Context) {
	itemId := c.Param("item_id")

	item, err := s.gorseClient.GetItem(itemId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, item)
}

// createItems 批量创建商品
func (s *Server) createItems(c *gin.Context) {
	var items []models.Item
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.gorseClient.InsertItems(items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "items created successfully", "count": len(items)})
}

// createFeedback 创建反馈
func (s *Server) createFeedback(c *gin.Context) {
	var feedback []models.Feedback
	if err := c.ShouldBindJSON(&feedback); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.gorseClient.InsertFeedback(feedback); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "feedback created successfully", "count": len(feedback)})
}

// getRecommend 获取推荐
func (s *Server) getRecommend(c *gin.Context) {
	userId := c.Param("user_id")
	category := c.DefaultQuery("category", "")
	n, _ := strconv.Atoi(c.DefaultQuery("n", "10"))

	items, err := s.gorseClient.GetRecommend(userId, category, n)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": userId,
		"items":   items,
		"total":   len(items),
	})
}

// getSimilarItems 获取相似商品
func (s *Server) getSimilarItems(c *gin.Context) {
	itemId := c.Param("item_id")
	n, _ := strconv.Atoi(c.DefaultQuery("n", "10"))

	items, err := s.gorseClient.GetItemNeighbors(itemId, n)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"item_id": itemId,
		"items":   items,
		"total":   len(items),
	})
}

// healthCheck 健康检查
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "fashion-recommend-api",
	})
}

// Run 启动服务器
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// GetRouter 获取路由器（用于测试）
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}
