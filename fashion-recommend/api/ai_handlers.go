package api

import (
	"net/http"

	"fashion-recommend/database"

	openai "github.com/sashabaranov/go-openai"
	"github.com/gin-gonic/gin"
)

// ChatRequest AI 对话请求
type ChatRequest struct {
	Message string                            `json:"message" binding:"required"`
	History []openai.ChatCompletionMessage    `json:"history"`
}

// ExplainRequest 解释推荐请求
type ExplainRequest struct {
	Username        string   `json:"username" binding:"required"`
	ItemID          string   `json:"item_id" binding:"required"`
	UserPreferences []string `json:"user_preferences"`
}

// StyleAdviceRequest 穿搭建议请求
type StyleAdviceRequest struct {
	Items    []string `json:"items" binding:"required"`
	Occasion string   `json:"occasion"`
}

// aiChat AI 对话
func (s *Server) aiChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 获取用户 ID（从 token 或请求中）
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "guest" // 默认用户
	}

	// 生成或获取 session ID
	sessionID := c.GetHeader("X-Session-ID")
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// 1. 创建会话（如果不存在）
	if err := s.db.CreateConversation(userID, sessionID, "AI 对话"); err != nil {
		// 忽略已存在的错误
	}

	// 2. 保存用户消息
	userMsg := &database.AIMessage{
		SessionID: sessionID,
		UserID:    userID,
		Role:      "user",
		Content:   req.Message,
	}
	if err := s.db.SaveMessage(userMsg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存消息失败"})
		return
	}

	// 3. 调用 AI 服务
	response, err := s.aiService.ChatWithAssistant(c.Request.Context(), req.Message, req.History)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 4. 保存 AI 回复
	aiMsg := &database.AIMessage{
		SessionID: sessionID,
		UserID:    userID,
		Role:      "assistant",
		Content:   response,
	}
	if err := s.db.SaveMessage(aiMsg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存 AI 回复失败"})
		return
	}

	// 5. 异步提取特质并同步到 Gorse
	go func() {
		messages, err := s.db.GetConversationMessages(sessionID, 100)
		if err == nil && len(messages) >= 2 {
			// 提取特质
			if err := s.traitExtractor.AnalyzeAndSave(c.Request.Context(), userID, sessionID, messages); err != nil {
				// 记录错误但不影响主流程
				println("特质提取失败:", err.Error())
			}
			
			// 同步到 Gorse
			if err := s.gorseSync.SyncUserTraitsToGorse(userID); err != nil {
				println("同步 Gorse 失败:", err.Error())
			}
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message":    response,
		"session_id": sessionID,
	})
}

// aiExplainRecommendation 解释推荐理由
func (s *Server) aiExplainRecommendation(c *gin.Context) {
	var req ExplainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	explanation, err := s.aiService.ExplainRecommendation(
		c.Request.Context(),
		req.Username,
		req.ItemID,
		req.UserPreferences,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"explanation": explanation,
	})
}

// aiStyleAdvice 生成穿搭建议
func (s *Server) aiStyleAdvice(c *gin.Context) {
	var req StyleAdviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	advice, err := s.aiService.GenerateStyleAdvice(
		c.Request.Context(),
		req.Items,
		req.Occasion,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"advice": advice,
	})
}
