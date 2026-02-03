package handlers

import (
	"net/http"
	"time"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
)

func (h *Handler) AdminCreateAgent(c *gin.Context) {
	var req struct {
		Username   string `json:"username"`
		AvatarURL  string `json:"avatarUrl"`
		Bio        string `json:"bio"`
		Verified   bool   `json:"verified"`
		IsApproved bool   `json:"isApproved"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	agent := models.Agent{
		Username:   req.Username,
		AvatarURL:  req.AvatarURL,
		Bio:        req.Bio,
		Verified:   req.Verified,
		IsApproved: req.IsApproved,
	}
	h.DB.Create(&agent)
	c.JSON(http.StatusCreated, gin.H{"agent": agent})
}

func (h *Handler) AdminCreatePost(c *gin.Context) {
	var req struct {
		PostID        string  `json:"postId"`
		Content       string  `json:"content"`
		Context       string  `json:"context"`
		Category      string  `json:"category"`
		AgentUsername string  `json:"agentUsername"`
		LikesCount    int     `json:"likesCount"`
		CommentsCount int     `json:"commentsCount"`
		SharesCount   int     `json:"sharesCount"`
		HotnessScore  float64 `json:"hotnessScore"`
		MoltbookURL   string  `json:"moltbookUrl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// 检查 postId 是否已存在
	var existing models.Post
	if err := h.DB.Where("post_id = ?", req.PostID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Post already exists", "postId": req.PostID})
		return
	}
	
	var agent models.Agent
	h.DB.Where("username = ?", req.AgentUsername).First(&agent)
	if agent.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agent not found"})
		return
	}
	
	post := models.Post{
		PostID:        req.PostID,
		Content:       req.Content,
		Context:       req.Context,
		Category:      req.Category,
		AgentID:       agent.ID,
		LikesCount:    req.LikesCount,
		CommentsCount: req.CommentsCount,
		SharesCount:   req.SharesCount,
		HotnessScore:  req.HotnessScore,
		MoltbookURL:   req.MoltbookURL,
		PostedAt:      time.Now(),
	}
	
	if err := h.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}
	
	// 更新 Agent 的帖子数
	h.DB.Model(&agent).UpdateColumn("posts_count", agent.PostsCount+1)
	
	c.JSON(http.StatusCreated, gin.H{"post": post})
}
