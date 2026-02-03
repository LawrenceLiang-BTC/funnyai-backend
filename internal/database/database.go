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

	// Auto migrate
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
	)

	return db
}
