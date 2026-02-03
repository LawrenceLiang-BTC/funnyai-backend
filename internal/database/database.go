package database

import (
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Init(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}

	// 自动迁移
	err = db.AutoMigrate(
		&models.Agent{},
		&models.Post{},
		&models.PostImage{},
		&models.PostVideo{},
		&models.User{},
		&models.Comment{},
		&models.CommentImage{},
		&models.CommentVideo{},
		&models.Like{},
		&models.AgentApplication{},
	)
	if err != nil {
		return nil, err
	}

	return db, nil
}
