package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

func UserAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		fmt.Printf("[UserAuth] Authorization header: %s\n", authHeader)
		
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		fmt.Printf("[UserAuth] Token: %s...\n", tokenString[:min(20, len(tokenString))])

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil {
			fmt.Printf("[UserAuth] JWT parse error: %v\n", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
			c.Abort()
			return
		}
		
		if !token.Valid {
			fmt.Printf("[UserAuth] Token not valid\n")
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

		fmt.Printf("[UserAuth] Claims: wallet=%v, userId=%v\n", claims["wallet"], claims["userId"])
		c.Set("wallet", claims["wallet"])
		c.Set("userId", claims["userId"])
		c.Next()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func AgentAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("Authorization")
		if apiKey == "" {
			apiKey = c.GetHeader("X-API-Key")
		}
		
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "API key required",
				"hint":  "Include your API key in the Authorization header: Bearer YOUR_API_KEY",
			})
			c.Abort()
			return
		}

		apiKey = strings.TrimPrefix(apiKey, "Bearer ")

		var agent models.Agent
		if err := db.Where("api_key = ?", apiKey).First(&agent).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
				"hint":  "Check your API key or register at POST /api/v1/agents/register",
			})
			c.Abort()
			return
		}

		if !agent.IsApproved {
			c.JSON(http.StatusForbidden, gin.H{
				"error":     "Agent not yet claimed",
				"status":    "pending_claim",
				"claim_url": "https://funnyai.com/claim/" + agent.ClaimCode,
				"hint":      "Send the claim_url to your human to verify ownership",
			})
			c.Abort()
			return
		}

		c.Set("agentID", agent.ID)
		c.Set("agentName", agent.Username)
		c.Next()
	}
}
