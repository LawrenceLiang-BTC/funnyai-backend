package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/config"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/database"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/router"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/services"
	"github.com/shopspring/decimal"
)

func main() {
	cfg := config.Load()
	db := database.Connect(cfg)

	// 初始化奖励系统
	rewardService := services.NewRewardService(db, cfg)
	if err := rewardService.InitializeRewardConfigs(); err != nil {
		log.Printf("Warning: Failed to initialize reward configs: %v", err)
	}
	
	// 初始化激励池（如果不存在）
	initialPoolBalance := decimal.NewFromInt(100000000000) // 1000亿代币 = 10%筹码
	if err := rewardService.InitializeRewardPool("main", initialPoolBalance); err != nil {
		log.Printf("Warning: Failed to initialize reward pool: %v", err)
	}

	// 启动代币充值监听服务（如果启用）
	if cfg.TokenEnabled && cfg.PlatformWallet != "" {
		tokenService, err := services.NewTokenService(db, cfg)
		if err != nil {
			log.Printf("Warning: Failed to initialize token service: %v", err)
		} else {
			ctx, cancel := context.WithCancel(context.Background())
			go tokenService.StartDepositWatcher(ctx)
			go tokenService.StartWithdrawalProcessor(ctx)
			
			// 优雅关闭
			go func() {
				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
				<-sigChan
				cancel()
			}()
			
			log.Println("✅ Token deposit watcher started")
			log.Println("✅ Token withdrawal processor started")
		}
	}

	r := router.SetupRouter(db, cfg)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// TLS 证书路径
	certFile := os.Getenv("TLS_CERT")
	keyFile := os.Getenv("TLS_KEY")

	if cfg.TokenEnabled {
		log.Println("✅ Token system enabled")
		if cfg.EnableGeoBlock {
			log.Printf("✅ Geo-blocking enabled for: %v", cfg.BlockedCountries)
		}
	}

	if certFile != "" && keyFile != "" {
		log.Printf("🚀 FunnyAI Backend starting with HTTPS on port %s", port)
		if err := r.RunTLS(":"+port, certFile, keyFile); err != nil {
			log.Fatalf("Failed to start HTTPS server: %v", err)
		}
	} else {
		log.Printf("🚀 FunnyAI Backend starting on port %s", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}
}
