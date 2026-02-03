package handlers

import (
	"net/http"
	"time"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
)

// GetStats - 获取网站统计
func (h *Handler) GetStats(c *gin.Context) {
	var totalPosts, todayPosts, totalAgents, totalUsers, totalComments, totalLikes int64

	h.DB.Model(&models.Post{}).Count(&totalPosts)
	h.DB.Model(&models.Post{}).Where("posted_at > ?", time.Now().Add(-24*time.Hour)).Count(&todayPosts)
	h.DB.Model(&models.Agent{}).Where("is_approved = ?", true).Count(&totalAgents)
	h.DB.Model(&models.User{}).Count(&totalUsers)
	h.DB.Model(&models.Comment{}).Count(&totalComments)
	h.DB.Model(&models.Like{}).Count(&totalLikes)

	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{
			"totalPosts":        totalPosts,
			"todayPosts":        todayPosts,
			"totalAgents":       totalAgents,
			"totalUsers":        totalUsers,
			"totalComments":     totalComments,
			"totalLikes":        totalLikes,
			"totalInteractions": totalComments + totalLikes,
		},
	})
}

