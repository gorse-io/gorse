package api

import (
	"net/http"
	"strconv"

	"fashion-recommend/database"

	"github.com/gin-gonic/gin"
)

// CreateCommentRequest 创建评论请求
type CreateCommentRequest struct {
	ItemID          string  `json:"item_id" binding:"required"`
	Content         string  `json:"content" binding:"required"`
	ParentID        *int    `json:"parent_id"`
	ReplyToUserID   *string `json:"reply_to_user_id"`
	ReplyToUsername *string `json:"reply_to_username"`
}

// createComment 创建评论
func (s *Server) createComment(c *gin.Context) {
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误", "details": err.Error()})
		return
	}

	// 获取用户信息
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	username := userID // 简化处理，实际应该从用户表获取

	comment := &database.Comment{
		ItemID:          req.ItemID,
		UserID:          userID,
		Username:        username,
		Content:         req.Content,
		ParentID:        req.ParentID,
		ReplyToUserID:   req.ReplyToUserID,
		ReplyToUsername: req.ReplyToUsername,
	}

	if err := s.db.CreateComment(comment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建评论失败"})
		return
	}

	c.JSON(http.StatusOK, comment)
}

// getComments 获取评论列表
func (s *Server) getComments(c *gin.Context) {
	itemID := c.Param("item_id")
	if itemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "商品 ID 不能为空"})
		return
	}

	// 获取用户 ID（用于判断是否点赞）
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "guest"
	}

	// 分页参数
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	comments, err := s.db.GetComments(itemID, userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取评论失败"})
		return
	}

	// 获取评论总数
	total, _ := s.db.GetCommentCount(itemID)

	c.JSON(http.StatusOK, gin.H{
		"comments": comments,
		"total":    total,
	})
}

// likeComment 点赞评论
func (s *Server) likeComment(c *gin.Context) {
	commentIDStr := c.Param("comment_id")
	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论 ID 无效"})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if err := s.db.LikeComment(commentID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "点赞失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "点赞成功"})
}

// unlikeComment 取消点赞评论
func (s *Server) unlikeComment(c *gin.Context) {
	commentIDStr := c.Param("comment_id")
	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论 ID 无效"})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if err := s.db.UnlikeComment(commentID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "取消点赞失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "取消点赞成功"})
}
