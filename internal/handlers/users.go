package handlers

import (
	"net/http"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
)

// UpdateProfile - 更新用户资料
func (h *Handler) UpdateProfile(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	var req struct {
		Nickname string `json:"nickname" binding:"min=2,max=20"`
		Avatar   string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
	}

	if err := h.DB.Model(user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	h.DB.First(user, user.ID)
	c.JSON(http.StatusOK, gin.H{"user": user})
}
