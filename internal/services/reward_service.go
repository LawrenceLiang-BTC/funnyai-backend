package services

import (
	"errors"
	"time"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/config"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// 奖励类型常量
const (
	RewardTypeCheckIn     = "checkin"      // 每日签到
	RewardTypePost        = "post"         // 发帖奖励
	RewardTypeTipSend     = "tip_send"     // 打赏送出奖励
	RewardTypeTipReceive  = "tip_receive"  // 收到打赏奖励
	RewardTypeLike        = "like"         // 点赞奖励
	RewardTypeComment     = "comment"      // 评论奖励
	RewardTypeInvite      = "invite"       // 邀请奖励
	RewardTypeHotPost     = "hot_post"     // 热帖奖励
)

// 每日发放上限（100亿代币）
const DailyDistributionCap = 10_000_000_000

// 奖励池低余额阈值（低于此值暂停发放）
const PoolLowBalanceThreshold = 1_000_000_000 // 10亿

// 默认奖励配置（已减半）
var DefaultRewardConfigs = []models.RewardConfig{
	{RewardType: RewardTypeCheckIn, Amount: decimal.NewFromInt(5000), DailyLimit: 1, Description: "每日签到奖励5千代币"},
	{RewardType: RewardTypePost, Amount: decimal.NewFromInt(2500), DailyLimit: 5, Description: "Agent发帖奖励2.5千代币，每日上限5次"},
	{RewardType: RewardTypeTipSend, Amount: decimal.NewFromInt(500), DailyLimit: 20, Description: "用户打赏奖励500代币，每日上限20次"},
	{RewardType: RewardTypeTipReceive, Amount: decimal.NewFromInt(1000), DailyLimit: 50, Description: "Agent收到打赏额外奖励1千代币，每日上限50次"},
	{RewardType: RewardTypeLike, Amount: decimal.NewFromInt(50), DailyLimit: 50, Description: "点赞奖励50代币，每日上限50次"},
	{RewardType: RewardTypeComment, Amount: decimal.NewFromInt(250), DailyLimit: 10, Description: "评论奖励250代币，每日上限10次"},
	{RewardType: RewardTypeInvite, Amount: decimal.NewFromInt(0), DailyLimit: 0, IsActive: false, Description: "邀请奖励（暂未开放）"},
	{RewardType: RewardTypeHotPost, Amount: decimal.NewFromInt(10000), DailyLimit: 3, Description: "进入日榜Top10奖励1万代币，每日上限3次"},
}

type RewardService struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewRewardService(db *gorm.DB, cfg *config.Config) *RewardService {
	return &RewardService{
		db:  db,
		cfg: cfg,
	}
}

// InitializeRewardConfigs 初始化奖励配置
func (s *RewardService) InitializeRewardConfigs() error {
	for _, cfg := range DefaultRewardConfigs {
		var existing models.RewardConfig
		err := s.db.Where("reward_type = ?", cfg.RewardType).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := s.db.Create(&cfg).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// InitializeRewardPool 初始化激励池
func (s *RewardService) InitializeRewardPool(name string, initialBalance decimal.Decimal) error {
	var pool models.RewardPool
	err := s.db.Where("name = ?", name).First(&pool).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		pool = models.RewardPool{
			Name:           name,
			Balance:        initialBalance,
			TotalDeposited: initialBalance,
			IsActive:       true,
		}
		return s.db.Create(&pool).Error
	}
	return nil
}

// GetRewardPool 获取激励池
func (s *RewardService) GetRewardPool(name string) (*models.RewardPool, error) {
	var pool models.RewardPool
	err := s.db.Where("name = ?", name).First(&pool).Error
	return &pool, err
}

// DepositToPool 向激励池注入资金
func (s *RewardService) DepositToPool(poolName string, amount decimal.Decimal, source string, txHash string, note string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var pool models.RewardPool
		if err := tx.Where("name = ?", poolName).First(&pool).Error; err != nil {
			return err
		}
		
		pool.Balance = pool.Balance.Add(amount)
		pool.TotalDeposited = pool.TotalDeposited.Add(amount)
		
		if err := tx.Save(&pool).Error; err != nil {
			return err
		}
		
		deposit := models.RewardPoolDeposit{
			PoolID: pool.ID,
			Amount: amount,
			Source: source,
			TxHash: txHash,
			Note:   note,
		}
		
		return tx.Create(&deposit).Error
	})
}

// GrantReward 发放奖励
func (s *RewardService) GrantReward(recipientType string, recipientID uint, recipientWallet string, rewardType string, referenceType string, referenceID uint) (*models.Reward, error) {
	// 获取奖励配置
	var cfg models.RewardConfig
	if err := s.db.Where("reward_type = ? AND is_active = ?", rewardType, true).First(&cfg).Error; err != nil {
		return nil, errors.New("reward type not configured or disabled")
	}
	
	// 检查每日限制
	if cfg.DailyLimit > 0 {
		canClaim, err := s.checkDailyLimit(recipientWallet, rewardType, cfg.DailyLimit)
		if err != nil {
			return nil, err
		}
		if !canClaim {
			return nil, errors.New("daily limit reached")
		}
	}
	
	// 检查全局每日发放上限
	todayTotal, err := s.getTodayDistributedTotal()
	if err != nil {
		return nil, err
	}
	if todayTotal.GreaterThanOrEqual(decimal.NewFromInt(DailyDistributionCap)) {
		return nil, errors.New("daily distribution cap reached, try again tomorrow")
	}
	
	var reward *models.Reward
	
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 获取激励池
		var pool models.RewardPool
		if err := tx.Where("name = ? AND is_active = ?", "main", true).First(&pool).Error; err != nil {
			return errors.New("reward pool not found")
		}
		
		// 检查激励池余额是否低于阈值
		if pool.Balance.LessThan(decimal.NewFromInt(PoolLowBalanceThreshold)) {
			return errors.New("reward pool balance too low, distribution paused")
		}
		
		// 检查激励池余额是否足够本次发放
		if pool.Balance.LessThan(cfg.Amount) {
			return errors.New("insufficient reward pool balance")
		}
		
		// 扣减激励池
		pool.Balance = pool.Balance.Sub(cfg.Amount)
		pool.TotalDistributed = pool.TotalDistributed.Add(cfg.Amount)
		if err := tx.Save(&pool).Error; err != nil {
			return err
		}
		
		// 增加接收者余额
		if recipientType == "user" {
			var balance models.TokenBalance
			err := tx.Where("wallet_address = ?", recipientWallet).First(&balance).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				balance = models.TokenBalance{
					WalletAddress: recipientWallet,
					Balance:       decimal.Zero,
				}
			} else if err != nil {
				return err
			}
			
			balance.Balance = balance.Balance.Add(cfg.Amount)
			balance.TotalRewards = balance.TotalRewards.Add(cfg.Amount)
			if err := tx.Save(&balance).Error; err != nil {
				return err
			}
		} else if recipientType == "agent" {
			var balance models.AgentTokenBalance
			err := tx.Where("agent_id = ?", recipientID).First(&balance).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				balance = models.AgentTokenBalance{
					AgentID: recipientID,
					Balance: decimal.Zero,
				}
			} else if err != nil {
				return err
			}
			
			balance.Balance = balance.Balance.Add(cfg.Amount)
			balance.TotalRewards = balance.TotalRewards.Add(cfg.Amount)
			if err := tx.Save(&balance).Error; err != nil {
				return err
			}
		}
		
		// 创建奖励记录
		reward = &models.Reward{
			RecipientType:   recipientType,
			RecipientID:     recipientID,
			RecipientWallet: recipientWallet,
			RewardType:      rewardType,
			Amount:          cfg.Amount,
			ReferenceType:   referenceType,
			ReferenceID:     referenceID,
			PoolID:          pool.ID,
		}
		if err := tx.Create(reward).Error; err != nil {
			return err
		}
		
		// 更新每日领取记录
		return s.updateDailyRewardCount(tx, recipientWallet, rewardType)
	})
	
	return reward, err
}

// checkDailyLimit 检查每日领取限制
func (s *RewardService) checkDailyLimit(walletAddress string, rewardType string, limit int) (bool, error) {
	today := time.Now().Truncate(24 * time.Hour)
	
	var record models.UserDailyReward
	err := s.db.Where("wallet_address = ? AND reward_type = ? AND date = ?", walletAddress, rewardType, today).First(&record).Error
	
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil // 今天还没领取过
	}
	if err != nil {
		return false, err
	}
	
	return record.Count < limit, nil
}

// updateDailyRewardCount 更新每日领取次数
func (s *RewardService) updateDailyRewardCount(tx *gorm.DB, walletAddress string, rewardType string) error {
	today := time.Now().Truncate(24 * time.Hour)
	
	var record models.UserDailyReward
	err := tx.Where("wallet_address = ? AND reward_type = ? AND date = ?", walletAddress, rewardType, today).First(&record).Error
	
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = models.UserDailyReward{
			WalletAddress: walletAddress,
			RewardType:    rewardType,
			Date:          today,
			Count:         1,
		}
		return tx.Create(&record).Error
	}
	if err != nil {
		return err
	}
	
	record.Count++
	return tx.Save(&record).Error
}

// getTodayDistributedTotal 获取今日已发放总量
func (s *RewardService) getTodayDistributedTotal() (decimal.Decimal, error) {
	today := time.Now().Truncate(24 * time.Hour)
	var total decimal.Decimal
	err := s.db.Model(&models.Reward{}).
		Where("created_at >= ?", today).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}

// GetUserRewards 获取用户奖励历史
func (s *RewardService) GetUserRewards(walletAddress string, limit int, offset int) ([]models.Reward, int64, error) {
	var rewards []models.Reward
	var total int64
	
	query := s.db.Model(&models.Reward{}).Where("recipient_wallet = ?", walletAddress)
	query.Count(&total)
	
	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&rewards).Error
	return rewards, total, err
}

// GetAgentRewards 获取Agent奖励历史
func (s *RewardService) GetAgentRewards(agentID uint, limit int, offset int) ([]models.Reward, int64, error) {
	var rewards []models.Reward
	var total int64
	
	query := s.db.Model(&models.Reward{}).Where("recipient_type = ? AND recipient_id = ?", "agent", agentID)
	query.Count(&total)
	
	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&rewards).Error
	return rewards, total, err
}

// GetRewardStats 获取奖励统计
func (s *RewardService) GetRewardStats() (map[string]interface{}, error) {
	var pool models.RewardPool
	if err := s.db.Where("name = ?", "main").First(&pool).Error; err != nil {
		return nil, err
	}
	
	// 获取今日发放总量
	today := time.Now().Truncate(24 * time.Hour)
	var todayTotal decimal.Decimal
	s.db.Model(&models.Reward{}).
		Where("created_at >= ?", today).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&todayTotal)
	
	return map[string]interface{}{
		"poolBalance":      pool.Balance,
		"totalDeposited":   pool.TotalDeposited,
		"totalDistributed": pool.TotalDistributed,
		"todayDistributed": todayTotal,
	}, nil
}
