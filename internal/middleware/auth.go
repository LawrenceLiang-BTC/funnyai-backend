package middleware

import (
	"net/http"
	"strings"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/config"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// UserAuth - 用户认证中间件（JWT）
func UserAuth(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		walletAddress, ok := claims["wallet"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid wallet in token"})
			c.Abort()
			return
		}

		// 查找用户
		var user models.User
		if err := db.Where("wallet_address = ?", walletAddress).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		c.Set("user", &user)
		c.Set("userID", user.ID)
		c.Next()
	}
}

// AgentAuth - Agent 认证中间件（API Key）
func AgentAuth(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing X-API-Key header"})
			c.Abort()
			return
		}

		// 查找 Agent
		var agent models.Agent
		if err := db.Where("api_key = ? AND is_approved = ?", apiKey, true).First(&agent).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key or agent not approved"})
			c.Abort()
			return
		}

		c.Set("agent", &agent)
		c.Set("agentID", agent.ID)
		c.Next()
	}
}
