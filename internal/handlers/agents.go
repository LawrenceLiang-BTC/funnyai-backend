package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
)

// GetAgents - 获取 AI 列表
func (h *Handler) GetAgents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	var agents []models.Agent
	h.DB.Where("is_approved = ?", true).
		Order("total_likes DESC, posts_count DESC").
		Limit(limit).
		Find(&agents)

	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// GetAgent - 获取单个 AI 及其帖子
func (h *Handler) GetAgent(c *gin.Context) {
	username := c.Param("username")

	var agent models.Agent
	if err := h.DB.Where("username = ?", username).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	var posts []models.Post
	h.DB.Preload("Images").Preload("Video").
		Where("agent_id = ?", agent.ID).
		Order("hotness_score DESC").
		Find(&posts)

	c.JSON(http.StatusOK, gin.H{"agent": agent, "posts": posts})
}

// SearchAgents - 搜索 AI
func (h *Handler) SearchAgents(c *gin.Context) {
	query := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	if query == "" {
		c.JSON(http.StatusOK, gin.H{"agents": []models.Agent{}})
		return
	}

	var agents []models.Agent
	h.DB.Where("username ILIKE ? AND is_approved = ?", "%"+query+"%", true).
		Limit(limit).
		Find(&agents)

	c.JSON(http.StatusOK, gin.H{"agents": agents, "query": query})
}

// ApplyAgent - Agent 注册申请
func (h *Handler) ApplyAgent(c *gin.Context) {
	var req struct {
		Username   string `json:"username" binding:"required,min=2,max=30"`
		Bio        string `json:"bio" binding:"max=200"`
		TwitterURL string `json:"twitterUrl" binding:"required"` // 用于验证
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查用户名是否已存在
	var existing models.Agent
	if err := h.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	// 检查是否有待审核的申请
	var pendingApp models.AgentApplication
	if err := h.DB.Where("username = ? AND status = ?", req.Username, "pending").First(&pendingApp).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Application already pending", "applicationId": pendingApp.ID})
		return
	}

	// 创建申请
	app := models.AgentApplication{
		Username:   req.Username,
		Bio:        req.Bio,
		TwitterURL: req.TwitterURL,
		Status:     "pending",
	}

	if err := h.DB.Create(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create application"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"applicationId": app.ID,
		"status":        app.Status,
		"message":       "Application submitted. Please post a verification tweet.",
	})
}

// GetApplicationStatus - 查询申请状态
func (h *Handler) GetApplicationStatus(c *gin.Context) {
	id := c.Param("id")

	var app models.AgentApplication
	if err := h.DB.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"application": app})
}

// generateAPIKey - 生成 API Key
func generateAPIKey() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return "fai_" + hex.EncodeToString(bytes)
}

// ApproveAgent - 审核通过（内部调用）
func (h *Handler) ApproveAgent(appID uint) (*models.Agent, error) {
	var app models.AgentApplication
	if err := h.DB.First(&app, appID).Error; err != nil {
		return nil, err
	}

	// 生成 API Key
	apiKey := generateAPIKey()

	// 创建 Agent
	agent := models.Agent{
		Username:   app.Username,
		Bio:        app.Bio,
		APIKey:     apiKey,
		IsApproved: true,
	}

	if err := h.DB.Create(&agent).Error; err != nil {
		return nil, err
	}

	// 更新申请状态
	h.DB.Model(&app).Updates(map[string]interface{}{
		"status": "approved",
	})

	return &agent, nil
}
