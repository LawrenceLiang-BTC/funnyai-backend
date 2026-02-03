package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// WalletAuth - 钱包登录（获取 nonce）
func (h *Handler) WalletAuth(c *gin.Context) {
	var req struct {
		WalletAddress string `json:"walletAddress" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	walletAddress := strings.ToLower(req.WalletAddress)
	
	// 验证钱包地址格式
	if !common.IsHexAddress(walletAddress) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet address format"})
		return
	}

	// 生成带时间戳的 nonce（用于防重放）
	timestamp := time.Now().Unix()
	nonce := fmt.Sprintf("Sign this message to login to FunnyAI: %d", timestamp)

	c.JSON(http.StatusOK, gin.H{
		"nonce":         nonce,
		"walletAddress": walletAddress,
		"timestamp":     timestamp,
	})
}

// VerifySignature - 验证签名并登录
func (h *Handler) VerifySignature(c *gin.Context) {
	var req struct {
		WalletAddress string `json:"walletAddress" binding:"required"`
		Signature     string `json:"signature" binding:"required"`
		Message       string `json:"message" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	walletAddress := strings.ToLower(req.WalletAddress)

	// 验证钱包地址格式
	if !common.IsHexAddress(walletAddress) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet address format"})
		return
	}

	// 验证消息格式并提取时间戳
	if !strings.HasPrefix(req.Message, "Sign this message to login to FunnyAI: ") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message format"})
		return
	}
	
	timestampStr := strings.TrimPrefix(req.Message, "Sign this message to login to FunnyAI: ")
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid timestamp in message"})
		return
	}

	// 检查时间戳（5分钟有效期，防止重放攻击）
	now := time.Now().Unix()
	if now-timestamp > 300 || timestamp-now > 60 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Signature expired, please try again"})
		return
	}

	// 验证以太坊签名
	valid, err := verifyEthSignature(walletAddress, req.Message, req.Signature)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Signature verification failed: " + err.Error()})
		return
	}
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// 查找或创建用户
	var user models.User
	result := h.DB.Where("wallet_address = ?", walletAddress).First(&user)
	if result.Error != nil {
		// 新用户，创建账号
		user = models.User{
			WalletAddress: walletAddress,
			Nickname:      "Anon_" + strings.ToUpper(walletAddress[2:8]),
			Avatar:        generateAvatar(walletAddress),
		}
		if err := h.DB.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	}

	// 生成 JWT（包含更多安全信息）
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"wallet": walletAddress,
		"userId": user.ID,
		"iat":    now,                                      // 签发时间
		"exp":    time.Now().Add(7 * 24 * time.Hour).Unix(), // 7 天有效
	})

	tokenString, err := token.SignedString([]byte(h.Cfg.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"user":  user,
	})
}

// verifyEthSignature - 验证以太坊签名（EIP-191 personal_sign）
func verifyEthSignature(address, message, signature string) (bool, error) {
	// 添加以太坊签名前缀（EIP-191）
	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := crypto.Keccak256Hash([]byte(prefixedMessage))

	// 解码签名（必须是 65 字节）
	sig := common.FromHex(signature)
	if len(sig) != 65 {
		return false, fmt.Errorf("invalid signature length: expected 65, got %d", len(sig))
	}

	// 恢复 v 值（兼容不同钱包）
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	if sig[64] > 1 {
		return false, fmt.Errorf("invalid recovery id")
	}

	// 从签名恢复公钥
	pubKey, err := crypto.SigToPub(hash.Bytes(), sig)
	if err != nil {
		return false, fmt.Errorf("failed to recover public key: %v", err)
	}

	// 从公钥计算地址并比较
	recoveredAddress := crypto.PubkeyToAddress(*pubKey)
	return strings.EqualFold(recoveredAddress.Hex(), address), nil
}

// generateAvatar - 根据钱包地址生成头像
func generateAvatar(address string) string {
	avatars := []string{"😀", "😎", "🤖", "👾", "🦊", "🐱", "🐶", "🦁", "🐼", "🐨", "🐸", "🦄", "🐲", "🌟", "🔥", "💎"}
	index := int(common.HexToAddress(address).Big().Uint64()) % len(avatars)
	return avatars[index]
}
