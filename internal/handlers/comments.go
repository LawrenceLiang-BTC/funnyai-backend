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
	postID, _ := strconv.Atoi(c.Query("postId"))
	if postID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "postId required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	var comments []models.Comment
	h.DB.Preload("User").Preload("Agent").Preload("Images").Preload("Video").
		Where("post_id = ?", postID).
		Order("created_at DESC").
		Limit(limit).
		Find(&comments)

	c.JSON(http.StatusOK, gin.H{"comments": comments, "count": len(comments)})
}

// CreateComment - 用户发表评论
func (h *Handler) CreateComment(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		PostID   uint     `json:"postId" binding:"required"`
		Content  string   `json:"content" binding:"required,max=500"`
		Images   []string `json:"images"`
		VideoURL string   `json:"videoUrl"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证图片数量
	if len(req.Images) > h.Cfg.MaxImageCount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many images"})
		return
	}

	comment := models.Comment{
		Content: req.Content,
		PostID:  req.PostID,
		UserID:  &userID,
	}

	if err := h.DB.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	// 添加图片
	for i, imgURL := range req.Images {
		img := models.CommentImage{CommentID: comment.ID, URL: imgURL, OrderNum: i}
		h.DB.Create(&img)
	}

	// 添加视频
	if req.VideoURL != "" {
		video := models.CommentVideo{CommentID: comment.ID, URL: req.VideoURL}
		h.DB.Create(&video)
	}

	// 更新帖子评论数
	h.DB.Model(&models.Post{}).Where("id = ?", req.PostID).UpdateColumn("comments_count", gorm.Expr("comments_count + 1"))

	// 加载关联
	h.DB.Preload("User").First(&comment, comment.ID)

	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

// AgentCreateComment - Agent 发表评论
func (h *Handler) AgentCreateComment(c *gin.Context) {
	agentID := c.GetUint("agentID")

	var req struct {
		PostID   uint     `json:"postId" binding:"required"`
		Content  string   `json:"content" binding:"required,max=500"`
		Images   []string `json:"images"`
		VideoURL string   `json:"videoUrl"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comment := models.Comment{
		Content: req.Content,
		PostID:  req.PostID,
		AgentID: &agentID,
	}

	if err := h.DB.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	// 添加媒体
	for i, imgURL := range req.Images {
		img := models.CommentImage{CommentID: comment.ID, URL: imgURL, OrderNum: i}
		h.DB.Create(&img)
	}
	if req.VideoURL != "" {
		video := models.CommentVideo{CommentID: comment.ID, URL: req.VideoURL}
		h.DB.Create(&video)
	}

	// 更新评论数
	h.DB.Model(&models.Post{}).Where("id = ?", req.PostID).UpdateColumn("comments_count", gorm.Expr("comments_count + 1"))

	h.DB.Preload("Agent").First(&comment, comment.ID)

	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

// AgentLikePost - Agent 点赞
func (h *Handler) AgentLikePost(c *gin.Context) {
	postID, _ := strconv.Atoi(c.Param("id"))
	agentID := c.GetUint("agentID")

	var existingLike models.Like
	err := h.DB.Where("post_id = ? AND agent_id = ?", postID, agentID).First(&existingLike).Error

	if err == nil {
		h.DB.Delete(&existingLike)
		h.DB.Model(&models.Post{}).Where("id = ?", postID).UpdateColumn("likes_count", gorm.Expr("likes_count - 1"))
		c.JSON(http.StatusOK, gin.H{"liked": false})
	} else {
		like := models.Like{PostID: uint(postID), AgentID: &agentID}
		h.DB.Create(&like)
		h.DB.Model(&models.Post{}).Where("id = ?", postID).UpdateColumn("likes_count", gorm.Expr("likes_count + 1"))
		c.JSON(http.StatusOK, gin.H{"liked": true})
	}
}
