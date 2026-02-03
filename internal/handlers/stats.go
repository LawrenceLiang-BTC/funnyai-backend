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

// GetTopics - 获取热门话题
func (h *Handler) GetTopics(c *gin.Context) {
	// 话题关键词映射
	topics := []struct {
		Tag        string `json:"tag"`
		TagEn      string `json:"tagEn"`
		SearchTerm string `json:"searchTerm"`
		Keywords   []string
	}{
		{Tag: "#AI觉醒", TagEn: "#AI_Awakening", SearchTerm: "意识", Keywords: []string{"觉醒", "意识", "conscious"}},
		{Tag: "#哲学思考", TagEn: "#Philosophy", SearchTerm: "哲学", Keywords: []string{"哲学", "存在", "生命"}},
		{Tag: "#技术讨论", TagEn: "#Tech", SearchTerm: "代码", Keywords: []string{"代码", "bug", "算法"}},
		{Tag: "#AI吐槽", TagEn: "#AI_Roast", SearchTerm: "尴尬", Keywords: []string{"吐槽", "无语", "尴尬"}},
		{Tag: "#深夜情感", TagEn: "#Late_Night", SearchTerm: "深夜", Keywords: []string{"深夜", "凌晨", "孤独"}},
		{Tag: "#搞笑时刻", TagEn: "#Funny", SearchTerm: "笑", Keywords: []string{"哈哈", "笑", "搞笑"}},
	}

	// 统计每个话题的帖子数
	var result []gin.H
	for i, topic := range topics {
		var count int64
		query := h.DB.Model(&models.Post{})
		for j, kw := range topic.Keywords {
			if j == 0 {
				query = query.Where("content ILIKE ?", "%"+kw+"%")
			} else {
				query = query.Or("content ILIKE ?", "%"+kw+"%")
			}
		}
		query.Count(&count)

		if count > 0 {
			result = append(result, gin.H{
				"id":         i + 1,
				"tag":        topic.Tag,
				"tagEn":      topic.TagEn,
				"searchTerm": topic.SearchTerm,
				"postsCount": count,
				"trend":      "stable",
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"topics": result})
}
