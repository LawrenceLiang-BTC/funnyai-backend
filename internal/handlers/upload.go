package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件类型"})
		return
	}

	isVideo := contentType == "video/mp4" || contentType == "video/webm"
	var maxSize int64
	if isVideo {
		maxSize = 10 * 1024 * 1024
	} else {
		maxSize = 5 * 1024 * 1024
	}
	if header.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件太大"})
		return
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	fileName := fmt.Sprintf("%s_%d%s", uuid.New().String()[:8], time.Now().Unix(), ext)

	r2AccountID := os.Getenv("R2_ACCOUNT_ID")
	r2AccessKey := os.Getenv("R2_ACCESS_KEY")
	r2SecretKey := os.Getenv("R2_SECRET_KEY")
	r2Bucket := os.Getenv("R2_BUCKET")
	r2PublicURL := os.Getenv("R2_PUBLIC_URL")

	if r2AccountID != "" && r2AccessKey != "" && r2SecretKey != "" && r2Bucket != "" {
		endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", r2AccountID)
		
		resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: endpoint}, nil
		})

		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithEndpointResolverWithOptions(resolver),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(r2AccessKey, r2SecretKey, "")),
			config.WithRegion("auto"),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "R2 配置失败: " + err.Error()})
			return
		}

		client := s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = true
		})

		_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:      aws.String(r2Bucket),
			Key:         aws.String(fileName),
			Body:        bytes.NewReader(fileBytes),
			ContentType: aws.String(contentType),
		})
		if err != nil {
			// R2 失败，fallback 到本地
			goto localUpload
		}

		url := r2PublicURL + "/" + fileName
		c.JSON(http.StatusOK, gin.H{"success": true, "url": url, "storage": "r2"})
		return
	}

localUpload:
	uploadDir := "./uploads"
	os.MkdirAll(uploadDir, 0755)
	dst, _ := os.Create(filepath.Join(uploadDir, fileName))
	defer dst.Close()
	dst.Write(fileBytes)

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://47.251.8.19:8080"
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "url": baseURL + "/uploads/" + fileName, "storage": "local"})
}
