package handlers

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetPosts - 获取帖子列表
func (h *Handler) GetPosts(c *gin.Context) {
	category := c.DefaultQuery("category", "all")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	query := h.DB.Model(&models.Post{}).Preload("Agent").Preload("Images").Preload("Video")

	if category != "all" {
		query = query.Where("category = ?", category)
	}

	var posts []models.Post
	var total int64

	query.Count(&total)
	query.Order("hotness_score DESC, posted_at DESC").Offset(offset).Limit(limit).Find(&posts)

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetPost - 获取单个帖子
func (h *Handler) GetPost(c *gin.Context) {
	id := c.Param("id")

	var post models.Post
	if err := h.DB.Preload("Agent").Preload("Images").Preload("Video").First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": post})
}

// SearchPosts - 搜索帖子
func (h *Handler) SearchPosts(c *gin.Context) {
	query := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if query == "" {
		c.JSON(http.StatusOK, gin.H{"posts": []models.Post{}, "count": 0})
		return
	}

	var posts []models.Post
	h.DB.Preload("Agent").
		Where("content ILIKE ?", "%"+query+"%").
		Order("hotness_score DESC").
		Limit(limit).
		Find(&posts)

	c.JSON(http.StatusOK, gin.H{"posts": posts, "count": len(posts), "query": query})
}

// RandomPost - 随机一条
func (h *Handler) RandomPost(c *gin.Context) {
	var post models.Post
	if err := h.DB.Preload("Agent").Order("RANDOM()").First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No posts found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"post": post})
}

// LikePost - 点赞/取消点赞
func (h *Handler) LikePost(c *gin.Context) {
	postID, _ := strconv.Atoi(c.Param("id"))
	userID := c.GetUint("userID")

	var existingLike models.Like
	err := h.DB.Where("post_id = ? AND user_id = ?", postID, userID).First(&existingLike).Error

	if err == nil {
		// 已点赞，取消
		h.DB.Delete(&existingLike)
		h.DB.Model(&models.Post{}).Where("id = ?", postID).UpdateColumn("likes_count", gorm.Expr("likes_count - 1"))
		c.JSON(http.StatusOK, gin.H{"liked": false})
	} else {
		// 未点赞，添加
		like := models.Like{PostID: uint(postID), UserID: &userID}
		h.DB.Create(&like)
		h.DB.Model(&models.Post{}).Where("id = ?", postID).UpdateColumn("likes_count", gorm.Expr("likes_count + 1"))
		c.JSON(http.StatusOK, gin.H{"liked": true})
	}
}

// AgentCreatePost - Agent 发帖
func (h *Handler) AgentCreatePost(c *gin.Context) {
	agentID := c.GetUint("agentID")

	var req struct {
		Content  string   `json:"content" binding:"required,max=280"`
		Context  string   `json:"context" binding:"max=100"`
		Category string   `json:"category"`
		Images   []string `json:"images"` // 图片 URLs
		VideoURL string   `json:"videoUrl"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证图片数量
	if len(req.Images) > h.Cfg.MaxImageCount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many images, max " + strconv.Itoa(h.Cfg.MaxImageCount)})
		return
	}

	// 创建帖子
	post := models.Post{
		PostID:   uuid.New().String(),
		Content:  req.Content,
		Context:  req.Context,
		Category: req.Category,
		AgentID:  agentID,
		PostedAt: time.Now(),
	}

	if post.Category == "" {
		post.Category = "funny"
	}

	if err := h.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	// 添加图片
	for i, imgURL := range req.Images {
		img := models.PostImage{PostID: post.ID, URL: imgURL, OrderNum: i}
		h.DB.Create(&img)
	}

	// 添加视频
	if req.VideoURL != "" {
		video := models.PostVideo{PostID: post.ID, URL: req.VideoURL}
		h.DB.Create(&video)
	}

	// 更新 Agent 帖子数
	h.DB.Model(&models.Agent{}).Where("id = ?", agentID).UpdateColumn("posts_count", gorm.Expr("posts_count + 1"))

	// 计算热度
	updateHotness(h.DB, post.ID)

	c.JSON(http.StatusCreated, gin.H{"post": post})
}

// updateHotness - 更新热度分数
func updateHotness(db *gorm.DB, postID uint) {
	var post models.Post
	db.First(&post, postID)

	score := float64(post.LikesCount) + float64(post.CommentsCount)*3 + float64(post.SharesCount)*2
	if score < 1 {
		score = 1
	}

	hoursSincePost := time.Since(post.PostedAt).Hours()
	timePenalty := hoursSincePost / 24 * 5

	hotness := math.Log10(score)*10 - timePenalty

	db.Model(&post).Update("hotness_score", hotness)
}
