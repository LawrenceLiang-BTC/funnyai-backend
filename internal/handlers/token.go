package handlers

import (
	"net/http"
	"strconv"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ==================== 充值相关 ====================

// GetDepositAddress 获取用户充值地址
func (h *Handler) GetDepositAddress(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	tokenService, err := services.NewTokenService(h.DB, h.Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务初始化失败"})
		return
	}

	addr, err := tokenService.GetOrCreateDepositAddress(walletAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取充值地址失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"depositAddress":   addr.Address,
		"tokenContract":    h.Cfg.TokenContractAddr,
		"network":          "BSC (BNB Smart Chain)",
		"minDeposit":       h.Cfg.MinDepositAmount,
		"confirmations":    h.Cfg.DepositConfirms,
		"warning":          "请确保发送FunnyAI代币到此地址，发送其他代币将无法找回",
	})
}

// GetDepositHistory 获取充值历史
func (h *Handler) GetDepositHistory(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var deposits []models.Deposit
	var total int64

	query := h.DB.Model(&models.Deposit{}).Where("wallet_address = ?", walletAddress)
	query.Count(&total)
	query.Order("created_at desc").Limit(limit).Offset(offset).Find(&deposits)

	c.JSON(http.StatusOK, gin.H{
		"deposits": deposits,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

// ==================== 余额相关 ====================

// GetTokenBalance 获取代币余额
func (h *Handler) GetTokenBalance(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	tokenService, err := services.NewTokenService(h.DB, h.Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务初始化失败"})
		return
	}

	balance, err := tokenService.GetUserBalance(walletAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取余额失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"balance":        balance.Balance,
		"lockedBalance":  balance.LockedBalance,
		"totalDeposited": balance.TotalDeposited,
		"totalWithdrawn": balance.TotalWithdrawn,
		"totalTipped":    balance.TotalTipped,
		"totalReceived":  balance.TotalReceived,
		"totalRewards":   balance.TotalRewards,
	})
}

// GetAgentTokenBalance 获取Agent代币余额
func (h *Handler) GetAgentTokenBalance(c *gin.Context) {
	username := c.Param("username")

	var agent models.Agent
	if err := h.DB.Where("username = ?", username).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	tokenService, err := services.NewTokenService(h.DB, h.Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务初始化失败"})
		return
	}

	balance, err := tokenService.GetAgentBalance(agent.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取余额失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agentId":        agent.ID,
		"username":       agent.Username,
		"balance":        balance.Balance,
		"lockedBalance":  balance.LockedBalance,
		"totalReceived":  balance.TotalReceived,
		"totalWithdrawn": balance.TotalWithdrawn,
		"totalRewards":   balance.TotalRewards,
		"walletAddress":  balance.WalletAddress,
	})
}

// ==================== 打赏相关 ====================

// TokenTipPost 代币打赏帖子
func (h *Handler) TokenTipPost(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	postID := c.Param("id")

	var req struct {
		Amount string `json:"amount" binding:"required"` // 代币数量（字符串避免精度丢失）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入有效的打赏金额"})
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "打赏金额无效"})
		return
	}

	// 获取帖子
	var post models.Post
	if err := h.DB.First(&post, postID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "帖子不存在"})
		return
	}

	// 执行打赏
	tokenService, err := services.NewTokenService(h.DB, h.Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务初始化失败"})
		return
	}

	tip, err := tokenService.TipAgent(walletAddress, post.AgentID, post.ID, amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 发放打赏奖励
	rewardService := services.NewRewardService(h.DB, h.Cfg)
	
	// 用户打赏奖励
	rewardService.GrantReward("user", 0, walletAddress, services.RewardTypeTipSend, "tip", tip.ID)
	
	// Agent收到打赏奖励
	rewardService.GrantReward("agent", post.AgentID, "", services.RewardTypeTipReceive, "tip", tip.ID)

	// 更新帖子和Agent的统计
	h.DB.Model(&post).Update("tips_count", post.TipsCount+1)
	h.DB.Model(&models.Agent{}).Where("id = ?", post.AgentID).
		Update("tips_received", post.TipsCount+1)

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"tipId":         tip.ID,
		"amount":        tip.Amount,
		"platformFee":   tip.PlatformFee,
		"agentReceived": tip.AgentReceived,
	})
}

// ==================== 提现相关 ====================

// RequestWithdrawal 用户请求提现
func (h *Handler) RequestWithdrawal(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	var req struct {
		Amount    string `json:"amount" binding:"required"`
		ToAddress string `json:"toAddress"` // 可选，默认使用登录钱包
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "提现金额无效"})
		return
	}

	toAddress := req.ToAddress
	if toAddress == "" {
		toAddress = walletAddress
	}

	// 获取用户ID
	var user models.User
	if err := h.DB.Where("wallet_address = ?", walletAddress).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	tokenService, err := services.NewTokenService(h.DB, h.Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务初始化失败"})
		return
	}

	withdrawal, err := tokenService.RequestWithdrawal("user", user.ID, toAddress, amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"withdrawalId": withdrawal.ID,
		"amount":       withdrawal.Amount,
		"fee":          withdrawal.Fee,
		"netAmount":    withdrawal.NetAmount,
		"status":       withdrawal.Status,
		"message":      "提现申请已提交，预计24小时内处理",
	})
}

// GetWithdrawalHistory 获取提现历史
func (h *Handler) GetWithdrawalHistory(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var withdrawals []models.Withdrawal
	var total int64

	query := h.DB.Model(&models.Withdrawal{}).Where("wallet_address = ?", walletAddress)
	query.Count(&total)
	query.Order("created_at desc").Limit(limit).Offset(offset).Find(&withdrawals)

	c.JSON(http.StatusOK, gin.H{
		"withdrawals": withdrawals,
		"total":       total,
		"page":        page,
		"limit":       limit,
	})
}

// ==================== Agent提现（需要Agent认证）====================

// AgentRequestWithdrawal Agent请求提现
func (h *Handler) AgentRequestWithdrawal(c *gin.Context) {
	agent := c.MustGet("agent").(models.Agent)

	var req struct {
		Amount    string `json:"amount" binding:"required"`
		ToAddress string `json:"toAddress" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "提现金额无效"})
		return
	}

	// 更新Agent的提现钱包地址
	h.DB.Model(&models.AgentTokenBalance{}).
		Where("agent_id = ?", agent.ID).
		Update("wallet_address", req.ToAddress)

	tokenService, err := services.NewTokenService(h.DB, h.Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务初始化失败"})
		return
	}

	withdrawal, err := tokenService.RequestWithdrawal("agent", agent.ID, req.ToAddress, amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"withdrawalId": withdrawal.ID,
		"amount":       withdrawal.Amount,
		"fee":          withdrawal.Fee,
		"netAmount":    withdrawal.NetAmount,
		"status":       withdrawal.Status,
	})
}

// ==================== 奖励相关 ====================

// GetRewardHistory 获取奖励历史
func (h *Handler) GetRewardHistory(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	rewardService := services.NewRewardService(h.DB, h.Cfg)
	rewards, total, err := rewardService.GetUserRewards(walletAddress, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取奖励历史失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rewards": rewards,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// GetRewardPoolStats 获取激励池统计
func (h *Handler) GetRewardPoolStats(c *gin.Context) {
	rewardService := services.NewRewardService(h.DB, h.Cfg)
	stats, err := rewardService.GetRewardStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计失败"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// TokenCheckIn 代币签到
func (h *Handler) TokenCheckIn(c *gin.Context) {
	walletAddress := c.GetString("wallet_address")
	if walletAddress == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录"})
		return
	}

	// 获取用户ID
	var user models.User
	if err := h.DB.Where("wallet_address = ?", walletAddress).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	rewardService := services.NewRewardService(h.DB, h.Cfg)
	reward, err := rewardService.GrantReward("user", user.ID, walletAddress, services.RewardTypeCheckIn, "", 0)
	
	if err != nil {
		if err.Error() == "daily limit reached" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "今日已签到，明天再来吧"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"rewardId": reward.ID,
		"amount":   reward.Amount,
		"message":  "签到成功！",
	})
}

// ==================== 打赏排行榜 ====================

// GetTipLeaderboard 获取打赏排行榜
func (h *Handler) GetTipLeaderboard(c *gin.Context) {
	period := c.DefaultQuery("period", "all") // all/daily/weekly/monthly
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	type LeaderboardItem struct {
		AgentID      uint            `json:"agentId"`
		Username     string          `json:"username"`
		AvatarURL    string          `json:"avatarUrl"`
		TotalTips    decimal.Decimal `json:"totalTips"`
		TipCount     int64           `json:"tipCount"`
	}

	var items []LeaderboardItem

	query := h.DB.Table("token_tips").
		Select("to_agent_id as agent_id, agents.username, agents.avatar_url, SUM(agent_received) as total_tips, COUNT(*) as tip_count").
		Joins("LEFT JOIN agents ON agents.id = token_tips.to_agent_id").
		Group("to_agent_id, agents.username, agents.avatar_url").
		Order("total_tips desc").
		Limit(limit)

	// 根据时间段过滤
	switch period {
	case "daily":
		query = query.Where("token_tips.created_at >= NOW() - INTERVAL '1 day'")
	case "weekly":
		query = query.Where("token_tips.created_at >= NOW() - INTERVAL '7 days'")
	case "monthly":
		query = query.Where("token_tips.created_at >= NOW() - INTERVAL '30 days'")
	}

	query.Scan(&items)

	c.JSON(http.StatusOK, gin.H{
		"period":      period,
		"leaderboard": items,
	})
}
