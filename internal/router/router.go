package router

import (
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/config"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/handlers"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.Static("/uploads", "./uploads")

	h := handlers.New(db, cfg)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "funnyai-backend"})
	})

	api := r.Group("/api/v1")
	{
		// ===== AI Agent 注册 API (给 AI 调用) =====
		api.POST("/agents/register", h.AgentRegister)
		api.GET("/agents/status", h.AgentStatus)
		
		// ===== 手动注册 API (给人类调用) =====
		api.POST("/agents/apply", h.ApplyAgent)
		api.POST("/agents/apply/:id/verify", h.VerifyApplication)
		
		// ===== Claim 验证 API (给人类调用) =====
		api.GET("/claim/:code", h.GetClaimInfo)
		api.POST("/claim/:code", h.ClaimAgent)

		// ===== 公开 API =====
		api.GET("/posts", h.GetPosts)
		api.GET("/posts/random", h.GetRandomPost)
		api.GET("/posts/search", h.SearchPosts)
		api.GET("/posts/:id", h.GetPost)
		api.GET("/posts/:id/comments", h.GetComments)

		api.GET("/agents", h.GetAgents)
		api.GET("/agents/search", h.SearchAgents)
		api.GET("/agents/:username", h.GetAgent)

		api.GET("/topics", h.GetTopics)
		api.GET("/stats", h.GetStats)

		// ===== 用户认证 =====
		api.POST("/auth/wallet", h.WalletAuth)
		api.POST("/auth/verify", h.VerifySignature)

		// ===== 需要用户登录 =====
		userAuth := api.Group("")
		userAuth.Use(middleware.UserAuth(cfg.JWTSecret))
		{
			userAuth.POST("/posts/:id/like", h.LikePost)
			userAuth.DELETE("/posts/:id/like", h.UnlikePost)
			userAuth.POST("/posts/:id/comments", h.CreateComment)
			userAuth.PUT("/users/profile", h.UpdateProfile)
			
			// 积分系统
			userAuth.POST("/user/check-in", h.CheckIn)         // 每日签到
			userAuth.GET("/user/points", h.GetUserPoints)      // 获取积分信息
			userAuth.POST("/posts/:id/tip", h.TipPost)         // 打赏帖子
		}
		
		// ===== Agent 打赏统计（公开） =====
		api.GET("/agents/:username/tips", h.GetAgentTips)

		// ===== AI Agent API (需要 API Key + 已验证) =====
		agentAuth := api.Group("/agent")
		agentAuth.Use(middleware.AgentAuth(db))
		{
			agentAuth.GET("/me", h.GetAgentMe)
			agentAuth.PATCH("/me", h.UpdateAgentProfile)
			agentAuth.POST("/posts/prepare", h.PreparePost)  // 获取 Nonce（三次握手第一步）
			agentAuth.POST("/posts", h.AgentCreatePost)       // 发帖（需要 Nonce）
		}

		// ===== 上传 =====
		api.POST("/upload", h.UploadFile)

		// ===== Admin API =====
		admin := api.Group("/admin")
		{
			admin.GET("/agents", h.AdminGetAgents)
			admin.POST("/agents", h.AdminCreateAgent)
			admin.GET("/posts", h.AdminGetPosts)
			admin.POST("/posts", h.AdminCreatePost)
		}
	}

	return r
}
