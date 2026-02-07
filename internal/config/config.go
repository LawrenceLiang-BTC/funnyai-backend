package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL    string
	RedisURL       string
	JWTSecret      string
	R2AccountID    string
	R2AccessKey    string
	R2SecretKey    string
	R2BucketName   string
	R2PublicURL    string
	MoltbookAPIKey string
	MaxPostLength  int
	MaxImageCount  int
	MaxVideoSecs   int

	// ===== 代币系统配置 =====
	TokenEnabled       bool    // 是否启用代币系统
	BSCNodeURL         string  // BSC RPC节点
	TokenContractAddr  string  // FunnyAI代币合约地址
	PlatformWallet     string  // 平台主钱包地址（归集、提现用）
	PlatformPrivateKey string  // 平台钱包私钥（加密存储）
	DepositConfirms    int     // 充值确认区块数
	
	// 费率配置
	TipFeeRate         float64 // 打赏平台抽成比例（如0.05表示5%）
	WithdrawFeeRate    float64 // 提现手续费比例
	MinWithdrawAmount  float64 // 最低提现金额（代币数量）
	MinDepositAmount   float64 // 最低充值金额（代币数量）
	
	// 激励池税费分配比例
	TaxToRewardPool    float64 // 税费进激励池比例（如0.5表示50%）
	TaxToBuyback       float64 // 税费用于回购销毁比例
	TaxToOperation     float64 // 税费用于运营比例
	
	// IP限制
	EnableGeoBlock     bool     // 是否启用地区限制
	BlockedCountries   []string // 被限制的国家代码
	
	// 加密密钥
	EncryptionKey      string  // AES加密密钥（用于加密私钥等敏感信息）
}

func Load() *Config {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "funnyai-jwt-secret-2026"
	}

	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		encryptionKey = "funnyai-encryption-key-32bytes!!" // 32 bytes for AES-256
	}

	return &Config{
		DatabaseURL:   "postgres://funnyai:funnyai123@localhost:5432/funnyai?sslmode=disable",
		RedisURL:      "redis://localhost:6379",
		JWTSecret:     jwtSecret,
		MaxPostLength: 200,  // 200字限制
		MaxImageCount: 4,    // 最多4张图片
		MaxVideoSecs:  10,   // 最长10秒视频

		// 代币系统配置
		TokenEnabled:       getEnvBool("TOKEN_ENABLED", true),
		BSCNodeURL:         getEnv("BSC_NODE_URL", "https://bsc-dataseed1.binance.org"),
		TokenContractAddr:  getEnv("TOKEN_CONTRACT", "0x3c471D10F11142C52DE4f3A3953c39d8AAaeFfFf"),
		PlatformWallet:     getEnv("PLATFORM_WALLET", ""),
		PlatformPrivateKey: os.Getenv("PLATFORM_PRIVATE_KEY"), // 敏感信息，不设默认值
		DepositConfirms:    getEnvInt("DEPOSIT_CONFIRMS", 6),
		
		// 费率配置
		TipFeeRate:        getEnvFloat("TIP_FEE_RATE", 0.05),        // 5%
		WithdrawFeeRate:   getEnvFloat("WITHDRAW_FEE_RATE", 0.02),   // 2%
		MinWithdrawAmount: getEnvFloat("MIN_WITHDRAW", 100000),      // 10万代币
		MinDepositAmount:  getEnvFloat("MIN_DEPOSIT", 100000),       // 10万代币（约$0.6）
		
		// 激励池税费分配
		TaxToRewardPool:   getEnvFloat("TAX_TO_REWARD", 0.5),        // 50%
		TaxToBuyback:      getEnvFloat("TAX_TO_BUYBACK", 0.2),       // 20%
		TaxToOperation:    getEnvFloat("TAX_TO_OPERATION", 0.3),     // 30%
		
		// IP限制
		EnableGeoBlock:    getEnvBool("ENABLE_GEO_BLOCK", true),
		BlockedCountries:  []string{"CN"}, // 默认限制中国大陆
		
		// 加密密钥
		EncryptionKey:     encryptionKey,
	}
}

// 辅助函数
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		return val == "true" || val == "1"
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var i int
		if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			return f
		}
	}
	return defaultVal
}
