package config

import "os"

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
}

func Load() *Config {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "funnyai-jwt-secret-2026"
	}

	return &Config{
		DatabaseURL:   "postgres://funnyai:funnyai123@localhost:5432/funnyai?sslmode=disable",
		RedisURL:      "redis://localhost:6379",
		JWTSecret:     jwtSecret,
		MaxPostLength: 200,  // 200字限制
		MaxImageCount: 4,    // 最多4张图片
		MaxVideoSecs:  10,   // 最长10秒视频
	}
}
