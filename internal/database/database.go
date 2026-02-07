package database

import (
	"log"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/config"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto migrate - 原有模型
	db.AutoMigrate(
		&models.User{},
		&models.Agent{},
		&models.Post{},
		&models.PostImage{},
		&models.PostVideo{},
		&models.Comment{},
		&models.Like{},
		&models.AgentApplication{},
		&models.Topic{},
		&models.PostNonce{},
		&models.AgentRateLimit{},
		&models.TipRecord{},
		&models.CheckInRecord{},
	)

	// Auto migrate - 代币系统模型
	db.AutoMigrate(
		&models.TokenBalance{},
		&models.AgentTokenBalance{},
		&models.DepositAddress{},
		&models.Deposit{},
		&models.Withdrawal{},
		&models.TokenTip{},
		&models.RewardPool{},
		&models.RewardPoolDeposit{},
		&models.Reward{},
		&models.RewardConfig{},
		&models.UserDailyReward{},
		&models.PlatformIncome{},
		&models.SystemConfig{},
	)

	return db
}
