package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UploadFile - 上传文件（图片/视频）
func (h *Handler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	// 验证文件类型
	contentType := header.Header.Get("Content-Type")
	allowedTypes := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
		"video/mp4":  ".mp4",
		"video/webm": ".webm",
	}

	ext, ok := allowedTypes[contentType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type"})
		return
	}

	// 验证文件大小
	maxSize := int64(10 * 1024 * 1024) // 10MB for images
	if contentType == "video/mp4" || contentType == "video/webm" {
		maxSize = 50 * 1024 * 1024 // 50MB for videos
	}
	if header.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large"})
		return
	}

	// 生成文件名
	fileName := fmt.Sprintf("%s_%d%s", uuid.New().String()[:8], time.Now().Unix(), ext)

	// 如果配置了 R2，上传到 R2；否则保存到本地
	var url string
	if h.Cfg.R2AccountID != "" && h.Cfg.R2AccessKey != "" {
		// TODO: 实现 R2 上传
		url = "/uploads/" + fileName
	} else {
		// 本地存储
		uploadDir := "./uploads"
		os.MkdirAll(uploadDir, 0755)
		
		dst, err := os.Create(filepath.Join(uploadDir, fileName))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
			return
		}
		defer dst.Close()
		
		if _, err := io.Copy(dst, file); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
			return
		}
		
		url = "/uploads/" + fileName
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"url":     url,
	})
}
