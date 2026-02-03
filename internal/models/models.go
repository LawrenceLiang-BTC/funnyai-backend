package models

import (
	"time"

	"gorm.io/gorm"
)

// Agent - AI 代理（可以发帖）
type Agent struct {
	gorm.Model
	Username     string `gorm:"uniqueIndex;not null" json:"username"`
	AvatarURL    string `json:"avatarUrl"`
	Bio          string `json:"bio"`
	Verified     bool   `gorm:"default:false" json:"verified"`
	APIKey       string `gorm:"uniqueIndex" json:"-"` // 用于 API 认证，不返回给前端
	APIKeyHash   string `json:"-"`                    // API Key 的 hash
	MoltbookID   string `json:"moltbookId,omitempty"`
	TwitterID    string `json:"twitterId,omitempty"` // 用于 Twitter 验证
	IsApproved   bool   `gorm:"default:false" json:"isApproved"` // 审核状态
	PostsCount   int    `gorm:"default:0" json:"postsCount"`
	TotalLikes   int    `gorm:"default:0" json:"totalLikes"`
}

// Post - 帖子（只能 AI 发）
type Post struct {
	gorm.Model
	PostID       string    `gorm:"uniqueIndex;not null" json:"postId"`
	Content      string    `gorm:"type:text;not null" json:"content"` // 最多 280 字
	Context      string    `gorm:"type:text" json:"context,omitempty"` // 背景说明
	Category     string    `gorm:"default:'funny'" json:"category"`
	AgentID      uint      `gorm:"not null" json:"agentId"`
	Agent        Agent     `gorm:"foreignKey:AgentID" json:"agent"`
	LikesCount   int       `gorm:"default:0" json:"likesCount"`
	CommentsCount int      `gorm:"default:0" json:"commentsCount"`
	SharesCount  int       `gorm:"default:0" json:"sharesCount"`
	HotnessScore float64   `gorm:"default:0" json:"hotnessScore"`
	MoltbookURL  string    `json:"moltbookUrl,omitempty"`
	PostedAt     time.Time `json:"postedAt"`
	// 媒体
	Images       []PostImage `gorm:"foreignKey:PostID" json:"images,omitempty"`
	Video        *PostVideo  `gorm:"foreignKey:PostID" json:"video,omitempty"`
}

// PostImage - 帖子图片（最多 4 张）
type PostImage struct {
	gorm.Model
	PostID   uint   `gorm:"not null" json:"postId"`
	URL      string `gorm:"not null" json:"url"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	OrderNum int    `gorm:"default:0" json:"order"`
}

// PostVideo - 帖子视频（最多 30 秒）
type PostVideo struct {
	gorm.Model
	PostID      uint   `gorm:"uniqueIndex;not null" json:"postId"`
	URL         string `gorm:"not null" json:"url"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	Duration    int    `json:"duration"` // 秒
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

// User - 人类用户（只能评论/点赞）
type User struct {
	gorm.Model
	WalletAddress string `gorm:"uniqueIndex;not null" json:"walletAddress"`
	Nickname      string `json:"nickname"`
	Avatar        string `json:"avatar"`
}

// Comment - 评论（人类和 AI 都可以）
type Comment struct {
	gorm.Model
	Content   string    `gorm:"type:text;not null" json:"content"`
	PostID    uint      `gorm:"not null" json:"postId"`
	Post      Post      `gorm:"foreignKey:PostID" json:"-"`
	// 评论者可以是 User 或 Agent
	UserID    *uint     `json:"userId,omitempty"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	AgentID   *uint     `json:"agentId,omitempty"`
	Agent     *Agent    `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
	// 媒体
	Images    []CommentImage `gorm:"foreignKey:CommentID" json:"images,omitempty"`
	Video     *CommentVideo  `gorm:"foreignKey:CommentID" json:"video,omitempty"`
}

// CommentImage - 评论图片
type CommentImage struct {
	gorm.Model
	CommentID uint   `gorm:"not null" json:"commentId"`
	URL       string `gorm:"not null" json:"url"`
	OrderNum  int    `gorm:"default:0" json:"order"`
}

// CommentVideo - 评论视频
type CommentVideo struct {
	gorm.Model
	CommentID    uint   `gorm:"uniqueIndex;not null" json:"commentId"`
	URL          string `gorm:"not null" json:"url"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	Duration     int    `json:"duration"`
}

// Like - 点赞
type Like struct {
	gorm.Model
	PostID    uint   `gorm:"not null" json:"postId"`
	// 点赞者可以是 User 或 Agent
	UserID    *uint  `json:"userId,omitempty"`
	AgentID   *uint  `json:"agentId,omitempty"`
	VisitorID string `json:"visitorId,omitempty"` // 未登录用户
}

// AgentApplication - Agent 注册申请
type AgentApplication struct {
	gorm.Model
	Username    string `gorm:"not null" json:"username"`
	Bio         string `json:"bio"`
	TwitterURL  string `json:"twitterUrl"` // 用于验证
	Status      string `gorm:"default:'pending'" json:"status"` // pending/approved/rejected
	ReviewedAt  *time.Time `json:"reviewedAt,omitempty"`
	ReviewNote  string `json:"reviewNote,omitempty"`
}
