package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// likeProduct 点赞商品
func (s *Server) likeProduct(c *gin.Context) {
	itemID := c.Param("item_id")
	if itemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item ID is required"})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "guest"
	}

	if err := s.db.LikeProduct(itemID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to like product"})
		return
	}

	// 获取最新点赞数
	count, _ := s.db.GetProductLikeCount(itemID)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Product liked successfully",
		"like_count": count,
	})
}

// unlikeProduct 取消点赞商品
func (s *Server) unlikeProduct(c *gin.Context) {
	itemID := c.Param("item_id")
	if itemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item ID is required"})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "guest"
	}

	if err := s.db.UnlikeProduct(itemID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlike product"})
		return
	}

	// 获取最新点赞数
	count, _ := s.db.GetProductLikeCount(itemID)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Product unliked successfully",
		"like_count": count,
	})
}

// getProductLikes 获取商品点赞信息
func (s *Server) getProductLikes(c *gin.Context) {
	itemID := c.Param("item_id")
	if itemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item ID is required"})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "guest"
	}

	count, err := s.db.GetProductLikeCount(itemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get like count"})
		return
	}

	isLiked, err := s.db.IsProductLiked(itemID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check like status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"item_id":    itemID,
		"like_count": count,
		"is_liked":   isLiked,
	})
}

// getBatchProductLikes 批量获取商品点赞信息
func (s *Server) getBatchProductLikes(c *gin.Context) {
	var req struct {
		ItemIDs []string `json:"item_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "guest"
	}

	likeInfo, err := s.db.GetProductsLikeInfo(req.ItemIDs, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get like info"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"likes": likeInfo,
	})
}
