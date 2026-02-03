package main

import (
	"log"
	"os"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/config"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/database"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/router"
)

func main() {
	cfg := config.Load()
	db := database.Connect(cfg)

	r := router.SetupRouter(db, cfg)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 FunnyAI Backend starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
