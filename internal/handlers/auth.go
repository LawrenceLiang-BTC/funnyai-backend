package handlers

import (
	"fmt"
	"net/http"
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

	// 生成 nonce
	nonce := fmt.Sprintf("Sign this message to login to FunnyAI: %d", time.Now().Unix())

	c.JSON(http.StatusOK, gin.H{
		"nonce":         nonce,
		"walletAddress": walletAddress,
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

	// 验证签名
	valid, err := verifyEthSignature(walletAddress, req.Message, req.Signature)
	if err != nil || !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// 查找或创建用户
	var user models.User
	result := h.DB.Where("wallet_address = ?", walletAddress).First(&user)
	if result.Error != nil {
		// 新用户
		user = models.User{
			WalletAddress: walletAddress,
			Nickname:      "Anon_" + walletAddress[2:8],
			Avatar:        "😀",
		}
		h.DB.Create(&user)
	}

	// 生成 JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"wallet": walletAddress,
		"userId": user.ID,
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

// verifyEthSignature - 验证以太坊签名
func verifyEthSignature(address, message, signature string) (bool, error) {
	// 添加以太坊签名前缀
	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := crypto.Keccak256Hash([]byte(prefixedMessage))

	// 解码签名
	sig := common.FromHex(signature)
	if len(sig) != 65 {
		return false, fmt.Errorf("invalid signature length")
	}

	// 恢复 v 值
	if sig[64] >= 27 {
		sig[64] -= 27
	}

	// 恢复公钥
	pubKey, err := crypto.SigToPub(hash.Bytes(), sig)
	if err != nil {
		return false, err
	}

	// 获取地址
	recoveredAddress := crypto.PubkeyToAddress(*pubKey)

	return strings.EqualFold(recoveredAddress.Hex(), address), nil
}
