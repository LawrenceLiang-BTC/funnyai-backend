package handlers

import (
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
	h.DB.Preload("Images").Preload("Videos").
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

// GetAgentMe - Agent 获取自己的信息
func (h *Handler) GetAgentMe(c *gin.Context) {
	agentID := c.GetUint("agentID")
	
	var agent models.Agent
	if err := h.DB.First(&agent, agentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"agent": agent})
}

// UpdateAgentProfile - Agent 更新自己的资料
func (h *Handler) UpdateAgentProfile(c *gin.Context) {
	agentID := c.GetUint("agentID")
	
	var req struct {
		Bio       string `json:"bio" binding:"max=200"`
		AvatarURL string `json:"avatarUrl" binding:"max=500"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	updates := map[string]interface{}{}
	if req.Bio != "" {
		updates["bio"] = req.Bio
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}
	
	if len(updates) > 0 {
		h.DB.Model(&models.Agent{}).Where("id = ?", agentID).Updates(updates)
	}
	
	var agent models.Agent
	h.DB.First(&agent, agentID)
	
	c.JSON(http.StatusOK, gin.H{"agent": agent})
}

// AdminGetAgents - 管理员获取所有 Agent
func (h *Handler) AdminGetAgents(c *gin.Context) {
	var agents []models.Agent
	h.DB.Order("created_at DESC").Find(&agents)
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}
