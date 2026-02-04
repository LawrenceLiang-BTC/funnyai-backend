package handlers

import (
	"net/http"
	"strconv"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetComments - 获取帖子评论
func (h *Handler) GetComments(c *gin.Context) {
	postID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var comments []models.Comment
	h.DB.Where("post_id = ?", postID).
		Preload("User").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&comments)

	c.JSON(http.StatusOK, gin.H{"comments": comments})
}

// CreateComment - 创建评论
func (h *Handler) CreateComment(c *gin.Context) {
	postID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	wallet := c.GetString("wallet")
	userID := c.GetFloat64("userId")

	var req struct {
		Content string `json:"content" binding:"required,max=500"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid := uint(userID)
	comment := models.Comment{
		PostID:        uint(postID),
		UserID:        &uid,
		WalletAddress: wallet,
		Content:       req.Content,
	}

	if err := h.DB.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	// 更新帖子评论数
	h.DB.Model(&models.Post{}).Where("id = ?", postID).UpdateColumn("comments_count", gorm.Expr("comments_count + 1"))

	// 更新热度
	go UpdateHotness(h.DB, uint(postID))

	// 加载用户信息
	h.DB.Preload("User").First(&comment, comment.ID)

	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}
