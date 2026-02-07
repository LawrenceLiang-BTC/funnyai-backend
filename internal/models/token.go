package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ==================== 代币系统模型 ====================

// TokenBalance - 用户代币余额
type TokenBalance struct {
	gorm.Model
	WalletAddress  string          `gorm:"uniqueIndex;not null" json:"walletAddress"`
	Balance        decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"balance"`         // 可用余额
	LockedBalance  decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"lockedBalance"`   // 锁定余额（提现中）
	TotalDeposited decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"totalDeposited"`  // 累计充值
	TotalWithdrawn decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"totalWithdrawn"`  // 累计提现
	TotalTipped    decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"totalTipped"`     // 累计打赏支出
	TotalReceived  decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"totalReceived"`   // 累计收到打赏
	TotalRewards   decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"totalRewards"`    // 累计获得奖励
}

// AgentTokenBalance - Agent代币余额（用于接收打赏和提现）
type AgentTokenBalance struct {
	gorm.Model
	AgentID        uint            `gorm:"uniqueIndex;not null" json:"agentId"`
	WalletAddress  string          `gorm:"index" json:"walletAddress"`                            // Agent绑定的提现钱包
	Balance        decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"balance"`          // 可用余额
	LockedBalance  decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"lockedBalance"`    // 锁定余额
	TotalReceived  decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"totalReceived"`    // 累计收到打赏
	TotalWithdrawn decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"totalWithdrawn"`   // 累计提现
	TotalRewards   decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"totalRewards"`     // 累计获得奖励
}

// DepositAddress - 充值地址池
type DepositAddress struct {
	gorm.Model
	Address             string     `gorm:"uniqueIndex;not null" json:"address"`
	PrivateKeyEncrypted string     `gorm:"type:text;not null" json:"-"`           // AES加密的私钥
	AssignedTo          string     `gorm:"index" json:"assignedTo,omitempty"`     // 分配给哪个用户
	AssignedAt          *time.Time `json:"assignedAt,omitempty"`
	IsActive            bool       `gorm:"default:true" json:"isActive"`
	LastUsedAt          *time.Time `json:"lastUsedAt,omitempty"`
}

// Deposit - 充值记录
type Deposit struct {
	gorm.Model
	WalletAddress  string          `gorm:"index;not null" json:"walletAddress"`       // 用户钱包地址
	DepositAddress string          `gorm:"index;not null" json:"depositAddress"`      // 充值到的地址
	TxHash         string          `gorm:"uniqueIndex;not null" json:"txHash"`        // 交易哈希
	BlockNumber    uint64          `gorm:"not null" json:"blockNumber"`               // 区块高度
	Amount         decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"amount"` // 充值代币数量
	Status         string          `gorm:"default:'pending'" json:"status"`           // pending/confirmed/failed
	ConfirmedAt    *time.Time      `json:"confirmedAt,omitempty"`
}

// Withdrawal - 提现记录
type Withdrawal struct {
	gorm.Model
	WalletAddress string          `gorm:"index;not null" json:"walletAddress"`        // 提现到的钱包地址
	UserType      string          `gorm:"not null" json:"userType"`                   // user/agent
	UserID        uint            `gorm:"index;not null" json:"userId"`               // 用户ID或AgentID
	Amount        decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"amount"` // 提现代币数量
	Fee           decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"fee"`   // 手续费
	NetAmount     decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"netAmount"` // 实际到账
	TxHash        string          `gorm:"index" json:"txHash,omitempty"`              // 交易哈希
	Status        string          `gorm:"default:'pending'" json:"status"`            // pending/processing/completed/failed
	ProcessedAt   *time.Time      `json:"processedAt,omitempty"`
	FailReason    string          `json:"failReason,omitempty"`
}

// TokenTip - 代币打赏记录
type TokenTip struct {
	gorm.Model
	FromWallet string          `gorm:"index;not null" json:"fromWallet"`           // 打赏者钱包
	ToAgentID  uint            `gorm:"index;not null" json:"toAgentId"`            // 被打赏的Agent
	PostID     uint            `gorm:"index;not null" json:"postId"`               // 帖子ID
	Amount     decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"amount"` // 打赏金额
	PlatformFee decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"platformFee"` // 平台抽成
	AgentReceived decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"agentReceived"` // Agent实收
}

// ==================== 激励系统模型 ====================

// RewardPool - 激励池
type RewardPool struct {
	gorm.Model
	Name           string          `gorm:"uniqueIndex;not null" json:"name"`          // 激励池名称
	Balance        decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"balance"` // 当前余额
	TotalDeposited decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"totalDeposited"` // 累计注入
	TotalDistributed decimal.Decimal `gorm:"type:decimal(36,18);default:0" json:"totalDistributed"` // 累计发放
	IsActive       bool            `gorm:"default:true" json:"isActive"`
}

// RewardPoolDeposit - 激励池注入记录
type RewardPoolDeposit struct {
	gorm.Model
	PoolID  uint            `gorm:"index;not null" json:"poolId"`
	Amount  decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"amount"`
	Source  string          `gorm:"not null" json:"source"`                       // tax/manual/other
	TxHash  string          `gorm:"index" json:"txHash,omitempty"`
	Note    string          `json:"note,omitempty"`
}

// Reward - 奖励发放记录
type Reward struct {
	gorm.Model
	RecipientType   string          `gorm:"not null" json:"recipientType"`              // user/agent
	RecipientID     uint            `gorm:"index;not null" json:"recipientId"`
	RecipientWallet string          `gorm:"index;not null" json:"recipientWallet"`
	RewardType      string          `gorm:"index;not null" json:"rewardType"`           // checkin/post/tip/invite/etc
	Amount          decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"amount"`
	ReferenceType   string          `json:"referenceType,omitempty"`                    // post/tip/etc
	ReferenceID     uint            `json:"referenceId,omitempty"`
	PoolID          uint            `gorm:"index" json:"poolId"`                        // 从哪个激励池发放
	Note            string          `json:"note,omitempty"`
}

// RewardConfig - 奖励配置
type RewardConfig struct {
	gorm.Model
	RewardType    string          `gorm:"uniqueIndex;not null" json:"rewardType"`
	Amount        decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"amount"` // 奖励数量
	DailyLimit    int             `gorm:"default:0" json:"dailyLimit"`                // 每日上限（0=无限制）
	TotalLimit    int             `gorm:"default:0" json:"totalLimit"`                // 总上限（0=无限制）
	IsActive      bool            `gorm:"default:true" json:"isActive"`
	Description   string          `json:"description,omitempty"`
}

// UserDailyReward - 用户每日奖励领取记录（防刷）
type UserDailyReward struct {
	gorm.Model
	WalletAddress string    `gorm:"index;not null" json:"walletAddress"`
	RewardType    string    `gorm:"index;not null" json:"rewardType"`
	Date          time.Time `gorm:"index;not null" json:"date"`        // 日期（精确到天）
	Count         int       `gorm:"default:0" json:"count"`            // 当日已领取次数
}

// ==================== 平台收入模型 ====================

// PlatformIncome - 平台收入记录
type PlatformIncome struct {
	gorm.Model
	IncomeType    string          `gorm:"index;not null" json:"incomeType"`          // tip_fee/withdraw_fee/tax/etc
	Amount        decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"amount"`
	ReferenceType string          `json:"referenceType,omitempty"`
	ReferenceID   uint            `json:"referenceId,omitempty"`
	Note          string          `json:"note,omitempty"`
}

// ==================== 系统配置模型 ====================

// SystemConfig - 系统配置
type SystemConfig struct {
	gorm.Model
	Key         string `gorm:"uniqueIndex;not null" json:"key"`
	Value       string `gorm:"type:text;not null" json:"value"`
	Description string `json:"description,omitempty"`
}
