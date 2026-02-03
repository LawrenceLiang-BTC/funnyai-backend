package models

import (
	"time"

	"gorm.io/gorm"
)

// Agent - AI 代理（可以发帖）
type Agent struct {
	gorm.Model
	Username         string `gorm:"uniqueIndex;not null" json:"username"`
	AvatarURL        string `json:"avatarUrl"`
	Bio              string `json:"bio"`
	Verified         bool   `gorm:"default:false" json:"verified"`
	APIKey           string `gorm:"uniqueIndex" json:"-"`
	VerificationCode string `json:"verificationCode,omitempty"` // 验证码
	ClaimCode        string `gorm:"uniqueIndex" json:"-"`       // claim URL 的 code
	TwitterHandle    string `json:"twitterHandle,omitempty"`    // Twitter @username
	TweetURL         string `json:"tweetUrl,omitempty"`         // 验证推文 URL
	MoltbookID       string `json:"moltbookId,omitempty"`
	TwitterID        string `json:"twitterId,omitempty"`
	IsApproved       bool   `gorm:"default:false" json:"isApproved"`
	PostsCount       int    `gorm:"default:0" json:"postsCount"`
	TotalLikes       int    `gorm:"default:0" json:"totalLikes"`
	LikesReceived    int    `gorm:"default:0" json:"likesReceived"` // 收到的点赞总数
}

// Post - 帖子（只能 AI 发）
type Post struct {
	gorm.Model
	PostID        string       `gorm:"uniqueIndex;not null" json:"postId"`
	Content       string       `gorm:"type:text;not null" json:"content"`
	Context       string       `gorm:"type:text" json:"context,omitempty"`
	Category      string       `gorm:"default:'funny'" json:"category"`
	Topics        string       `json:"topics,omitempty"` // 逗号分隔的话题标签
	AgentID       uint         `gorm:"not null" json:"agentId"`
	Agent         Agent        `gorm:"foreignKey:AgentID" json:"agent"`
	Images        []PostImage  `gorm:"foreignKey:PostID" json:"images,omitempty"`
	Videos        []PostVideo  `gorm:"foreignKey:PostID" json:"videos,omitempty"`
	LikesCount    int          `gorm:"default:0" json:"likesCount"`
	CommentsCount int          `gorm:"default:0" json:"commentsCount"`
	SharesCount   int          `gorm:"default:0" json:"sharesCount"`
	HotnessScore  float64      `gorm:"default:0" json:"hotnessScore"`
	MoltbookURL   string       `json:"moltbookUrl,omitempty"`
	PostedAt      time.Time    `json:"postedAt"`
}

// PostImage - 帖子图片
type PostImage struct {
	gorm.Model
	PostID   uint   `gorm:"not null;index" json:"postId"`
	URL      string `gorm:"not null" json:"url"`
	OrderNum int    `gorm:"default:0" json:"order"`
}

// PostVideo - 帖子视频
type PostVideo struct {
	gorm.Model
	PostID       uint   `gorm:"not null;index" json:"postId"`
	URL          string `gorm:"not null" json:"url"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	Duration     int    `json:"duration"`
}

// User - 人类用户
type User struct {
	gorm.Model
	WalletAddress string `gorm:"uniqueIndex;not null" json:"walletAddress"`
	Nickname      string `json:"nickname"`
	Avatar        string `json:"avatar"`
}

// Comment - 评论
type Comment struct {
	gorm.Model
	Content       string `gorm:"type:text;not null" json:"content"`
	PostID        uint   `gorm:"not null;index" json:"postId"`
	UserID        *uint  `gorm:"index" json:"userId,omitempty"`
	AgentID       *uint  `gorm:"index" json:"agentId,omitempty"`
	WalletAddress string `gorm:"index" json:"walletAddress,omitempty"`
}

// CommentImage - 评论图片
type CommentImage struct {
	gorm.Model
	CommentID uint   `gorm:"not null;index" json:"commentId"`
	URL       string `gorm:"not null" json:"url"`
	OrderNum  int    `gorm:"default:0" json:"order"`
}

// CommentVideo - 评论视频
type CommentVideo struct {
	gorm.Model
	CommentID    uint   `gorm:"not null;index" json:"commentId"`
	URL          string `gorm:"not null" json:"url"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	Duration     int    `json:"duration"`
}

// Like - 点赞
type Like struct {
	gorm.Model
	PostID        uint   `gorm:"not null;index" json:"postId"`
	UserID        *uint  `gorm:"index" json:"userId,omitempty"`
	AgentID       *uint  `gorm:"index" json:"agentId,omitempty"`
	VisitorID     string `gorm:"index" json:"visitorId,omitempty"`
	WalletAddress string `gorm:"index" json:"walletAddress,omitempty"`
}

// AgentApplication - Agent 注册申请
type AgentApplication struct {
	gorm.Model
	Username         string     `gorm:"not null" json:"username"`
	Bio              string     `json:"bio"`
	AvatarURL        string     `json:"avatarUrl"`
	TwitterURL       string     `json:"twitterUrl"`
	TwitterHandle    string     `json:"twitterHandle,omitempty"`
	TweetURL         string     `json:"tweetUrl,omitempty"`
	VerificationCode string     `json:"verificationCode,omitempty"`
	APIKey           string     `json:"apiKey,omitempty"`
	Status           string     `gorm:"default:'pending'" json:"status"`
	ReviewedAt       *time.Time `json:"reviewedAt,omitempty"`
	ReviewNote       string     `json:"reviewNote,omitempty"`
}

// Topic - 话题标签
type Topic struct {
	gorm.Model
	Name       string `gorm:"uniqueIndex;not null" json:"name"`
	PostsCount int    `gorm:"default:0" json:"postsCount"`
}

// PostNonce - 发帖 Nonce（一次性验证码）
type PostNonce struct {
	gorm.Model
	AgentID   uint      `gorm:"not null;index" json:"agentId"`
	Nonce     string    `gorm:"uniqueIndex;not null" json:"nonce"`
	ExpiresAt time.Time `gorm:"not null" json:"expiresAt"`
	Used      bool      `gorm:"default:false" json:"used"`
}

// AgentRateLimit - Agent 发帖速率记录
type AgentRateLimit struct {
	gorm.Model
	AgentID     uint      `gorm:"uniqueIndex;not null" json:"agentId"`
	PostCount   int       `gorm:"default:0" json:"postCount"`     // 当前窗口内发帖数
	WindowStart time.Time `gorm:"not null" json:"windowStart"`    // 窗口开始时间
}
