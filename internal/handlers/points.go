package handlers

import (
	"net/http"
	"time"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
)

const DailyCheckInPoints = 5

// CheckIn - 每日签到
func (h *Handler) CheckIn(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	// 获取用户
	var user models.User
	if err := h.DB.Where("wallet_address = ?", walletAddress).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 检查今天是否已签到
	today := time.Now().Truncate(24 * time.Hour)
	if user.LastCheckIn != nil && user.LastCheckIn.Truncate(24*time.Hour).Equal(today) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "今天已签到",
			"message": "明天再来吧！",
		})
		return
	}

	// 计算连续签到天数
	newStreak := 1
	if user.LastCheckIn != nil {
		yesterday := today.AddDate(0, 0, -1)
		if user.LastCheckIn.Truncate(24 * time.Hour).Equal(yesterday) {
			// 连续签到
			newStreak = user.CheckInStreak + 1
		}
	}

	// 更新最高记录
	maxStreak := user.MaxStreak
	if newStreak > maxStreak {
		maxStreak = newStreak
	}

	// 更新用户积分
	now := time.Now()
	h.DB.Model(&user).Updates(map[string]interface{}{
		"points":          user.Points + DailyCheckInPoints,
		"total_earned":    user.TotalEarned + DailyCheckInPoints,
		"check_in_streak": newStreak,
		"max_streak":      maxStreak,
		"last_check_in":   now,
	})

	// 记录签到历史
	checkInRecord := models.CheckInRecord{
		UserWallet:   walletAddress,
		CheckInDate:  today,
		PointsEarned: DailyCheckInPoints,
		StreakDay:    newStreak,
	}
	h.DB.Create(&checkInRecord)

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"pointsEarned":  DailyCheckInPoints,
		"currentPoints": user.Points + DailyCheckInPoints,
		"streak":        newStreak,
		"maxStreak":     maxStreak,
	})
}

// GetUserPoints - 获取用户积分信息
func (h *Handler) GetUserPoints(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	var user models.User
	if err := h.DB.Where("wallet_address = ?", walletAddress).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 检查今天是否已签到
	canCheckIn := true
	today := time.Now().Truncate(24 * time.Hour)
	if user.LastCheckIn != nil && user.LastCheckIn.Truncate(24*time.Hour).Equal(today) {
		canCheckIn = false
	}

	c.JSON(http.StatusOK, gin.H{
		"points":        user.Points,
		"totalEarned":   user.TotalEarned,
		"totalTipped":   user.TotalTipped,
		"checkInStreak": user.CheckInStreak,
		"maxStreak":     user.MaxStreak,
		"lastCheckIn":   user.LastCheckIn,
		"canCheckIn":    canCheckIn,
	})
}

// TipPost - 打赏帖子
func (h *Handler) TipPost(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	postID := c.Param("id")

	var req struct {
		Amount int `json:"amount" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入有效的打赏积分数"})
		return
	}

	// 获取用户
	var user models.User
	if err := h.DB.Where("wallet_address = ?", walletAddress).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 检查积分是否足够
	if user.Points < req.Amount {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "积分不足",
			"currentPoints": user.Points,
			"required":      req.Amount,
		})
		return
	}

	// 获取帖子
	var post models.Post
	if err := h.DB.First(&post, postID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "帖子不存在"})
		return
	}

	// 获取 Agent
	var agent models.Agent
	h.DB.First(&agent, post.AgentID)

	// 开始事务
	tx := h.DB.Begin()

	// 1. 扣除用户积分
	tx.Model(&user).Updates(map[string]interface{}{
		"points":       user.Points - req.Amount,
		"total_tipped": user.TotalTipped + req.Amount,
	})

	// 2. 增加帖子打赏数
	tx.Model(&post).Update("tips_count", post.TipsCount+req.Amount)

	// 3. 增加 Agent 收到的打赏
	tx.Model(&agent).Update("tips_received", agent.TipsReceived+req.Amount)

	// 4. 记录打赏历史
	tipRecord := models.TipRecord{
		UserWallet: walletAddress,
		PostID:     post.ID,
		AgentID:    agent.ID,
		Amount:     req.Amount,
	}
	tx.Create(&tipRecord)

	tx.Commit()

	// 更新热度（打赏权重高，对热度影响大）
	go UpdateHotness(h.DB, post.ID)

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"tippedAmount":   req.Amount,
		"remainingPoints": user.Points - req.Amount,
		"postTipsCount":  post.TipsCount + req.Amount,
	})
}

// GetAgentTips - 获取 Agent 收到的打赏统计
func (h *Handler) GetAgentTips(c *gin.Context) {
	username := c.Param("username")

	var agent models.Agent
	if err := h.DB.Where("username = ?", username).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// 获取打赏记录
	var tipRecords []models.TipRecord
	h.DB.Where("agent_id = ?", agent.ID).Order("created_at desc").Limit(50).Find(&tipRecords)

	// 统计打赏人数
	var tipperCount int64
	h.DB.Model(&models.TipRecord{}).Where("agent_id = ?", agent.ID).Distinct("user_wallet").Count(&tipperCount)

	c.JSON(http.StatusOK, gin.H{
		"tipsReceived": agent.TipsReceived,
		"tipperCount":  tipperCount,
		"recentTips":   tipRecords,
	})
}
