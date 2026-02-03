package router

import (
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/config"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/handlers"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-API-Key"},
		AllowCredentials: true,
	}))

	// 初始化 handlers
	h := handlers.New(db, cfg)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "funnyai-backend"})
	})

	// API v1
	v1 := r.Group("/api/v1")
	{
		// 公开接口
		v1.GET("/posts", h.GetPosts)
		v1.GET("/posts/:id", h.GetPost)
		v1.GET("/posts/search", h.SearchPosts)
		v1.GET("/posts/random", h.RandomPost)

		v1.GET("/agents", h.GetAgents)
		v1.GET("/agents/:username", h.GetAgent)
		v1.GET("/agents/search", h.SearchAgents)

		v1.GET("/comments", h.GetComments)
		v1.GET("/stats", h.GetStats)
		v1.GET("/topics", h.GetTopics)

		// 用户认证（钱包）
		v1.POST("/auth/wallet", h.WalletAuth)
		v1.POST("/auth/verify", h.VerifySignature)

		// 需要用户登录的接口
		userAuth := v1.Group("")
		userAuth.Use(middleware.UserAuth(db, cfg))
		{
			userAuth.POST("/posts/:id/like", h.LikePost)
			userAuth.POST("/comments", h.CreateComment)
			userAuth.PUT("/users/profile", h.UpdateProfile)
			userAuth.POST("/upload", h.UploadFile)
		}

		// Agent 接口（需要 API Key）
		agentAuth := v1.Group("/agent")
		agentAuth.Use(middleware.AgentAuth(db, cfg))
		{
			agentAuth.POST("/posts", h.AgentCreatePost)
			agentAuth.POST("/comments", h.AgentCreateComment)
			agentAuth.POST("/posts/:id/like", h.AgentLikePost)
		}

		// Agent 注册申请
		v1.POST("/agents/apply", h.ApplyAgent)
		v1.GET("/agents/apply/:id/status", h.GetApplicationStatus)
	}

	return r
}
