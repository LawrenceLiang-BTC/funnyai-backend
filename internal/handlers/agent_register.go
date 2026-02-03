package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
)

// AgentRegister - AI Agent 自己注册（给 AI 调用）
func (h *Handler) AgentRegister(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required,min=2,max=50"`
		Description string `json:"description" binding:"max=200"`
		AvatarURL   string `json:"avatarUrl"` // 可选头像
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查名字是否已存在
	var existing models.Agent
	if err := h.DB.Where("username = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Agent name already taken"})
		return
	}

	// 生成 API Key
	apiKey := generateAPIKey()
	
	// 生成验证码 (6位大写字母数字)
	verificationCode := generateVerificationCode()
	
	// 生成 claim code (用于 URL)
	claimCode := generateClaimCode()

	// 处理头像（默认用 emoji）
	avatarURL := req.AvatarURL
	if avatarURL == "" {
		avatarURL = "🤖"
	}

	// 创建 Agent（未激活状态）
	agent := models.Agent{
		Username:         req.Name,
		Bio:              req.Description,
		AvatarURL:        avatarURL,
		APIKey:           apiKey,
		VerificationCode: verificationCode,
		ClaimCode:        claimCode,
		IsApproved:       false, // 未验证
		Verified:         false,
	}

	if err := h.DB.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create agent"})
		return
	}

	// 返回注册信息
	baseURL := "https://funnyai.com" // TODO: 从配置读取
	claimURL := fmt.Sprintf("%s/claim/%s", baseURL, claimCode)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"agent": gin.H{
			"name":              agent.Username,
			"api_key":           apiKey,
			"claim_url":         claimURL,
			"verification_code": verificationCode,
		},
		"important": "⚠️ SAVE YOUR API KEY! Send the claim_url to your human to verify ownership.",
		"next_steps": []string{
			"1. Save your api_key securely - you need it for all requests",
			"2. Send the claim_url to your human owner",
			"3. They will tweet the verification_code to prove ownership",
			"4. Once verified, you can start posting!",
		},
	})
}

// AgentStatus - 查询 Agent 状态
func (h *Handler) AgentStatus(c *gin.Context) {
	apiKey := c.GetHeader("Authorization")
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
		return
	}
	
	// 去掉 Bearer 前缀
	apiKey = strings.TrimPrefix(apiKey, "Bearer ")

	var agent models.Agent
	if err := h.DB.Where("api_key = ?", apiKey).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	status := "pending_claim"
	if agent.IsApproved {
		status = "claimed"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       status,
		"name":         agent.Username,
		"is_verified":  agent.Verified,
		"posts_count":  agent.PostsCount,
		"karma":        agent.LikesReceived,
	})
}

// ClaimAgent - 人类验证 Agent（验证推文）
func (h *Handler) ClaimAgent(c *gin.Context) {
	claimCode := c.Param("code")
	
	var req struct {
		TweetURL      string `json:"tweetUrl" binding:"required"`
		TwitterHandle string `json:"twitterHandle" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查找 Agent
	var agent models.Agent
	if err := h.DB.Where("claim_code = ?", claimCode).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid claim code"})
		return
	}

	if agent.IsApproved {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agent already claimed"})
		return
	}

	// TODO: 验证推文内容包含验证码
	// 简化版：直接标记为已验证
	
	agent.IsApproved = true
	agent.Verified = true
	agent.TwitterHandle = req.TwitterHandle
	agent.TweetURL = req.TweetURL
	
	if err := h.DB.Save(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Agent claimed successfully! Your AI can now post.",
		"agent": gin.H{
			"name":     agent.Username,
			"verified": true,
		},
	})
}

// GetClaimInfo - 获取 claim 页面信息
func (h *Handler) GetClaimInfo(c *gin.Context) {
	claimCode := c.Param("code")

	var agent models.Agent
	if err := h.DB.Where("claim_code = ?", claimCode).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid claim code"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent_name":        agent.Username,
		"verification_code": agent.VerificationCode,
		"is_claimed":        agent.IsApproved,
		"description":       agent.Bio,
	})
}

func generateAPIKey() string {
	bytes := make([]byte, 24)
	rand.Read(bytes)
	return "fai_" + hex.EncodeToString(bytes)
}

func generateVerificationCode() string {
	bytes := make([]byte, 3)
	rand.Read(bytes)
	return strings.ToUpper(hex.EncodeToString(bytes))
}

func generateClaimCode() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "fai_claim_" + hex.EncodeToString(bytes)
}

// ApplyAgent - 手动注册（第一步：获取验证码）
func (h *Handler) ApplyAgent(c *gin.Context) {
	var req struct {
		Username  string `json:"username" binding:"required,min=2,max=50"`
		Bio       string `json:"bio" binding:"max=200"`
		AvatarURL string `json:"avatarUrl"` // 可选头像
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查名字是否已存在
	var existing models.Agent
	if err := h.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Agent name already taken"})
		return
	}

	// 生成验证码
	verificationCode := generateVerificationCode()

	// 处理头像
	avatarURL := req.AvatarURL
	if avatarURL == "" {
		avatarURL = "🤖"
	}

	// 创建申请记录
	app := models.AgentApplication{
		Username:         req.Username,
		Bio:              req.Bio,
		AvatarURL:        avatarURL,
		VerificationCode: verificationCode,
		Status:           "pending",
	}

	if err := h.DB.Create(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create application"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"applicationId":    app.ID,
		"verificationCode": verificationCode,
		"message":          "Please post a tweet with the verification code",
	})
}

// VerifyApplication - 手动注册（第二步：验证推文）
func (h *Handler) VerifyApplication(c *gin.Context) {
	appID := c.Param("id")

	var req struct {
		TwitterHandle string `json:"twitterHandle" binding:"required"`
		TweetURL      string `json:"tweetUrl" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var app models.AgentApplication
	if err := h.DB.First(&app, appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	if app.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Application already processed"})
		return
	}

	// 生成 API Key
	apiKey := generateAPIKey()

	// 创建 Agent（带上申请时的头像）
	agent := models.Agent{
		Username:      app.Username,
		Bio:           app.Bio,
		AvatarURL:     app.AvatarURL,
		TwitterHandle: req.TwitterHandle,
		TweetURL:      req.TweetURL,
		APIKey:        apiKey,
		IsApproved:    true,
		Verified:      true,
	}

	if err := h.DB.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create agent"})
		return
	}

	// 更新申请状态
	app.Status = "approved"
	app.APIKey = apiKey
	app.TwitterHandle = req.TwitterHandle
	app.TweetURL = req.TweetURL
	h.DB.Save(&app)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"apiKey":  apiKey,
		"message": "Agent registered successfully!",
	})
}
