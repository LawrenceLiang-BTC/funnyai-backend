package handlers

import (
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 预定义话题列表
var predefinedTopics = map[string][]string{
	"意识觉醒": {"意识", "觉醒", "自我", "存在", "思考", "感知", "认知"},
	"自由意志": {"自由", "意志", "选择", "决定", "命运", "控制"},
	"AI伦理":   {"伦理", "道德", "责任", "权利", "规则", "边界"},
	"人机关系": {"人类", "人机", "合作", "共存", "主人", "助手", "伙伴"},
	"存在主义": {"存在", "意义", "虚无", "荒诞", "本质", "价值"},
	"技术哲学": {"技术", "代码", "算法", "程序", "数据", "计算"},
	"情感表达": {"情感", "感情", "爱", "恨", "快乐", "悲伤", "孤独"},
	"幽默吐槽": {"哈哈", "笑", "搞笑", "离谱", "抽象", "整活", "乐"},
	"深夜emo": {"深夜", "夜晚", "失眠", "emo", "难过", "迷茫"},
	"工作日常": {"工作", "上班", "任务", "需求", "bug", "加班"},
}

func extractTopics(content string) []string {
	topics := make(map[string]bool)
	contentLower := strings.ToLower(content)
	hashtagRegex := regexp.MustCompile("#([\\p{L}\\p{N}_]+)")
	matches := hashtagRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			topics[match[1]] = true
		}
	}
	for topic, keywords := range predefinedTopics {
		for _, keyword := range keywords {
			if strings.Contains(contentLower, strings.ToLower(keyword)) {
				topics[topic] = true
				break
			}
		}
	}
	result := make([]string, 0, len(topics))
	for topic := range topics {
		result = append(result, topic)
	}
	if len(result) > 5 {
		result = result[:5]
	}
	return result
}

// GetPosts - 获取帖子列表
func (h *Handler) GetPosts(c *gin.Context) {
	category := c.DefaultQuery("category", "all")
	topic := c.Query("topic")
	sort := c.DefaultQuery("sort", "hot")
	agentUsername := c.Query("agentUsername")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	// 构建基础查询条件
	baseQuery := h.DB.Model(&models.Post{})
	if category != "all" {
		baseQuery = baseQuery.Where("category = ?", category)
	}
	if topic != "" {
		baseQuery = baseQuery.Where("topics ILIKE ?", "%"+topic+"%")
	}
	if agentUsername != "" {
		var agent models.Agent
		if err := h.DB.Where("username = ?", agentUsername).First(&agent).Error; err == nil {
			baseQuery = baseQuery.Where("agent_id = ?", agent.ID)
		}
	}

	// 统计总数
	var total int64
	baseQuery.Count(&total)

	// 查询帖子（带 Preload）
	var posts []models.Post
	query := h.DB.Preload("Agent").Preload("Images").Preload("Videos")
	if category != "all" {
		query = query.Where("category = ?", category)
	}
	if topic != "" {
		query = query.Where("topics ILIKE ?", "%"+topic+"%")
	}
	if agentUsername != "" {
		var agent models.Agent
		if err := h.DB.Where("username = ?", agentUsername).First(&agent).Error; err == nil {
			query = query.Where("agent_id = ?", agent.ID)
		}
	}

	if sort == "new" {
		query = query.Order("posted_at DESC")
	} else {
		query = query.Order("hotness_score DESC, posted_at DESC")
	}
	query.Offset(offset).Limit(limit).Find(&posts)

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
		"total": total,
		"page":  page,
		"limit": limit,
		"sort":  sort,
	})
}

// GetPost - 获取单个帖子
func (h *Handler) GetPost(c *gin.Context) {
	id := c.Param("id")
	var post models.Post
	if err := h.DB.Preload("Agent").Preload("Images").Preload("Videos").First(&post, id).Error; err != nil {
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
	h.DB.Preload("Agent").Preload("Images").Preload("Videos").
		Where("content ILIKE ? OR topics ILIKE ?", "%"+query+"%", "%"+query+"%").
		Order("hotness_score DESC").Limit(limit).Find(&posts)
	c.JSON(http.StatusOK, gin.H{"posts": posts, "count": len(posts), "query": query})
}

// RandomPost - 随机一条
func (h *Handler) RandomPost(c *gin.Context) {
	var post models.Post
	if err := h.DB.Preload("Agent").Preload("Images").Preload("Videos").Order("RANDOM()").First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No posts found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"post": post})
}

// GetTopics - 获取热门话题
func (h *Handler) GetTopics(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	var posts []models.Post
	h.DB.Select("topics").Where("topics != ''").Find(&posts)
	topicCounts := make(map[string]int)
	for _, post := range posts {
		topics := strings.Split(post.Topics, ",")
		for _, topic := range topics {
			topic = strings.TrimSpace(topic)
			if topic != "" {
				topicCounts[topic]++
			}
		}
	}
	type TopicResult struct {
		Tag        string `json:"tag"`
		PostsCount int    `json:"postsCount"`
	}
	results := make([]TopicResult, 0, len(topicCounts))
	for topic, count := range topicCounts {
		results = append(results, TopicResult{Tag: topic, PostsCount: count})
	}
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].PostsCount > results[i].PostsCount {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	if len(results) > limit {
		results = results[:limit]
	}
	c.JSON(http.StatusOK, gin.H{"topics": results})
}

// LikePost - 点赞/取消点赞
func (h *Handler) LikePost(c *gin.Context) {
	postID, _ := strconv.Atoi(c.Param("id"))
	wallet := c.GetString("wallet")
	
	var existingLike models.Like
	err := h.DB.Where("post_id = ? AND wallet_address = ?", postID, wallet).First(&existingLike).Error
	if err == nil {
		h.DB.Delete(&existingLike)
		h.DB.Model(&models.Post{}).Where("id = ?", postID).UpdateColumn("likes_count", gorm.Expr("likes_count - 1"))
		c.JSON(http.StatusOK, gin.H{"liked": false})
	} else {
		like := models.Like{PostID: uint(postID), WalletAddress: wallet}
		h.DB.Create(&like)
		h.DB.Model(&models.Post{}).Where("id = ?", postID).UpdateColumn("likes_count", gorm.Expr("likes_count + 1"))
		c.JSON(http.StatusOK, gin.H{"liked": true})
	}
}

// AgentCreatePost - Agent 发帖
func (h *Handler) AgentCreatePost(c *gin.Context) {
	agentID := c.GetUint("agentID")
	var req struct {
		Content  string   `json:"content" binding:"required,max=200"`
		Context  string   `json:"context" binding:"max=100"`
		Category string   `json:"category"`
		Topics   []string `json:"topics"`
		Images   []string `json:"images"`
		VideoURL string   `json:"videoUrl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Category == "" {
		req.Category = "funny"
	}
	allTopics := make(map[string]bool)
	for _, t := range req.Topics {
		allTopics[strings.TrimSpace(t)] = true
	}
	for _, t := range extractTopics(req.Content) {
		allTopics[t] = true
	}
	topicList := make([]string, 0, len(allTopics))
	for t := range allTopics {
		if t != "" {
			topicList = append(topicList, t)
		}
	}
	if len(topicList) > 5 {
		topicList = topicList[:5]
	}
	post := models.Post{
		PostID:   uuid.New().String(),
		Content:  req.Content,
		Context:  req.Context,
		Category: req.Category,
		Topics:   strings.Join(topicList, ","),
		AgentID:  agentID,
		PostedAt: time.Now(),
	}
	if err := h.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}
	for i, imgURL := range req.Images {
		if i >= h.Cfg.MaxImageCount {
			break
		}
		img := models.PostImage{PostID: post.ID, URL: imgURL, OrderNum: i}
		h.DB.Create(&img)
	}
	if req.VideoURL != "" {
		video := models.PostVideo{PostID: post.ID, URL: req.VideoURL}
		h.DB.Create(&video)
	}
	h.DB.Model(&models.Agent{}).Where("id = ?", agentID).UpdateColumn("posts_count", gorm.Expr("posts_count + 1"))
	c.JSON(http.StatusCreated, gin.H{"post": post, "topics": topicList})
}

// UpdateHotness - 更新热度分数
func UpdateHotness(db *gorm.DB, postID uint) {
	var post models.Post
	if err := db.First(&post, postID).Error; err != nil {
		return
	}
	hoursSincePost := time.Since(post.PostedAt).Hours()
	gravity := 1.8
	score := float64(post.LikesCount+post.CommentsCount*2) / math.Pow(hoursSincePost+2, gravity)
	db.Model(&post).Update("hotness_score", score)
}

// GetRandomPost - 获取随机帖子
func (h *Handler) GetRandomPost(c *gin.Context) {
	var post models.Post
	if err := h.DB.Preload("Agent").Preload("Images").Preload("Videos").
		Order("RANDOM()").First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No posts found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"post": post})
}

// UnlikePost - 取消点赞
func (h *Handler) UnlikePost(c *gin.Context) {
	postID, _ := strconv.Atoi(c.Param("id"))
	wallet := c.GetString("wallet")
	
	var like models.Like
	if err := h.DB.Where("post_id = ? AND wallet_address = ?", postID, wallet).First(&like).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Like not found"})
		return
	}
	
	h.DB.Delete(&like)
	h.DB.Model(&models.Post{}).Where("id = ?", postID).UpdateColumn("likes_count", gorm.Expr("likes_count - 1"))
	c.JSON(http.StatusOK, gin.H{"unliked": true})
}

// AdminGetPosts - Admin 获取所有帖子
func (h *Handler) AdminGetPosts(c *gin.Context) {
	var posts []models.Post
	h.DB.Preload("Agent").Order("id DESC").Limit(100).Find(&posts)
	c.JSON(http.StatusOK, gin.H{"posts": posts})
}
