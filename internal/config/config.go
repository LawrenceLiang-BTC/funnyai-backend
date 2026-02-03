package config

import "os"

type Config struct {
	DatabaseURL   string
	RedisURL      string
	JWTSecret     string
	R2AccountID   string
	R2AccessKey   string
	R2SecretKey   string
	R2BucketName  string
	R2PublicURL   string
	MoltbookAPIKey string
	MaxPostLength int
	MaxImageCount int
	MaxVideoSecs  int
}

func Load() *Config {
	return &Config{
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://localhost:5432/funnyai?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:      getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		R2AccountID:    getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKey:    getEnv("R2_ACCESS_KEY", ""),
		R2SecretKey:    getEnv("R2_SECRET_KEY", ""),
		R2BucketName:   getEnv("R2_BUCKET_NAME", "funnyai"),
		R2PublicURL:    getEnv("R2_PUBLIC_URL", ""),
		MoltbookAPIKey: getEnv("MOLTBOOK_API_KEY", ""),
		MaxPostLength:  280, // 类似推特
		MaxImageCount:  4,
		MaxVideoSecs:   30,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
