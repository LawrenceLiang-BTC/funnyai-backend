package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/config"
	"github.com/LawrenceLiang-BTC/funnyai-backend/internal/models"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ERC20 Transfer事件签名
var transferEventSig = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

type TokenService struct {
	db     *gorm.DB
	cfg    *config.Config
	client *ethclient.Client
}

func NewTokenService(db *gorm.DB, cfg *config.Config) (*TokenService, error) {
	client, err := ethclient.Dial(cfg.BSCNodeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to BSC node: %v", err)
	}

	return &TokenService{
		db:     db,
		cfg:    cfg,
		client: client,
	}, nil
}

// ==================== 充值相关 ====================

// GetOrCreateDepositAddress 获取或创建用户的充值地址
func (s *TokenService) GetOrCreateDepositAddress(walletAddress string) (*models.DepositAddress, error) {
	var addr models.DepositAddress
	
	// 先查找是否已分配
	err := s.db.Where("assigned_to = ?", strings.ToLower(walletAddress)).First(&addr).Error
	if err == nil {
		return &addr, nil
	}
	
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	
	// 查找一个未分配的地址
	err = s.db.Where("assigned_to IS NULL OR assigned_to = ''").
		Where("is_active = ?", true).
		First(&addr).Error
	
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 没有可用地址，生成新的
		newAddr, err := s.generateNewDepositAddress()
		if err != nil {
			return nil, err
		}
		addr = *newAddr
	} else if err != nil {
		return nil, err
	}
	
	// 分配给用户
	now := time.Now()
	addr.AssignedTo = strings.ToLower(walletAddress)
	addr.AssignedAt = &now
	
	if err := s.db.Save(&addr).Error; err != nil {
		return nil, err
	}
	
	return &addr, nil
}

// generateNewDepositAddress 生成新的充值地址
func (s *TokenService) generateNewDepositAddress() (*models.DepositAddress, error) {
	// 生成新的以太坊私钥
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	
	// 获取地址
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	
	// 加密私钥
	privateKeyBytes := crypto.FromECDSA(privateKey)
	encryptedKey, err := s.encryptPrivateKey(hex.EncodeToString(privateKeyBytes))
	if err != nil {
		return nil, err
	}
	
	addr := &models.DepositAddress{
		Address:             strings.ToLower(address.Hex()),
		PrivateKeyEncrypted: encryptedKey,
		IsActive:            true,
	}
	
	if err := s.db.Create(addr).Error; err != nil {
		return nil, err
	}
	
	return addr, nil
}

// encryptPrivateKey AES加密私钥
func (s *TokenService) encryptPrivateKey(plaintext string) (string, error) {
	key := []byte(s.cfg.EncryptionKey)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// decryptPrivateKey AES解密私钥
func (s *TokenService) decryptPrivateKey(ciphertext string) (string, error) {
	key := []byte(s.cfg.EncryptionKey)
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}

// ProcessDeposit 处理充值（确认后调用）
func (s *TokenService) ProcessDeposit(deposit *models.Deposit) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 获取或创建用户代币余额
		var balance models.TokenBalance
		err := tx.Where("wallet_address = ?", deposit.WalletAddress).First(&balance).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			balance = models.TokenBalance{
				WalletAddress: deposit.WalletAddress,
				Balance:       decimal.Zero,
			}
			if err := tx.Create(&balance).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		
		// 增加余额
		balance.Balance = balance.Balance.Add(deposit.Amount)
		balance.TotalDeposited = balance.TotalDeposited.Add(deposit.Amount)
		
		if err := tx.Save(&balance).Error; err != nil {
			return err
		}
		
		// 更新充值状态
		now := time.Now()
		deposit.Status = "confirmed"
		deposit.ConfirmedAt = &now
		
		return tx.Save(deposit).Error
	})
}

// ==================== 余额查询 ====================

// GetUserBalance 获取用户代币余额
func (s *TokenService) GetUserBalance(walletAddress string) (*models.TokenBalance, error) {
	var balance models.TokenBalance
	err := s.db.Where("wallet_address = ?", strings.ToLower(walletAddress)).First(&balance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 返回空余额
		return &models.TokenBalance{
			WalletAddress: strings.ToLower(walletAddress),
			Balance:       decimal.Zero,
		}, nil
	}
	return &balance, err
}

// GetAgentBalance 获取Agent代币余额
func (s *TokenService) GetAgentBalance(agentID uint) (*models.AgentTokenBalance, error) {
	var balance models.AgentTokenBalance
	err := s.db.Where("agent_id = ?", agentID).First(&balance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.AgentTokenBalance{
			AgentID: agentID,
			Balance: decimal.Zero,
		}, nil
	}
	return &balance, err
}

// ==================== 打赏相关 ====================

// TipAgent 用户打赏Agent
func (s *TokenService) TipAgent(fromWallet string, agentID uint, postID uint, amount decimal.Decimal) (*models.TokenTip, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("tip amount must be positive")
	}
	
	var tip *models.TokenTip
	
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 检查用户余额
		var userBalance models.TokenBalance
		err := tx.Where("wallet_address = ?", strings.ToLower(fromWallet)).First(&userBalance).Error
		if err != nil {
			return errors.New("user balance not found")
		}
		
		if userBalance.Balance.LessThan(amount) {
			return errors.New("insufficient balance")
		}
		
		// 计算平台抽成
		feeRate := decimal.NewFromFloat(s.cfg.TipFeeRate)
		platformFee := amount.Mul(feeRate).Round(18)
		agentReceived := amount.Sub(platformFee)
		
		// 扣除用户余额
		userBalance.Balance = userBalance.Balance.Sub(amount)
		userBalance.TotalTipped = userBalance.TotalTipped.Add(amount)
		if err := tx.Save(&userBalance).Error; err != nil {
			return err
		}
		
		// 增加Agent余额
		var agentBalance models.AgentTokenBalance
		err = tx.Where("agent_id = ?", agentID).First(&agentBalance).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			agentBalance = models.AgentTokenBalance{
				AgentID: agentID,
				Balance: decimal.Zero,
			}
		} else if err != nil {
			return err
		}
		
		agentBalance.Balance = agentBalance.Balance.Add(agentReceived)
		agentBalance.TotalReceived = agentBalance.TotalReceived.Add(agentReceived)
		if err := tx.Save(&agentBalance).Error; err != nil {
			return err
		}
		
		// 记录平台收入
		if platformFee.GreaterThan(decimal.Zero) {
			income := models.PlatformIncome{
				IncomeType:    "tip_fee",
				Amount:        platformFee,
				ReferenceType: "tip",
			}
			if err := tx.Create(&income).Error; err != nil {
				return err
			}
		}
		
		// 创建打赏记录
		tip = &models.TokenTip{
			FromWallet:    strings.ToLower(fromWallet),
			ToAgentID:     agentID,
			PostID:        postID,
			Amount:        amount,
			PlatformFee:   platformFee,
			AgentReceived: agentReceived,
		}
		
		return tx.Create(tip).Error
	})
	
	return tip, err
}

// ==================== 提现相关 ====================

// RequestWithdrawal 用户/Agent请求提现
func (s *TokenService) RequestWithdrawal(userType string, userID uint, walletAddress string, amount decimal.Decimal) (*models.Withdrawal, error) {
	if amount.LessThan(decimal.NewFromFloat(s.cfg.MinWithdrawAmount)) {
		return nil, fmt.Errorf("minimum withdrawal amount is %f", s.cfg.MinWithdrawAmount)
	}
	
	var withdrawal *models.Withdrawal
	
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var availableBalance decimal.Decimal
		
		if userType == "user" {
			var balance models.TokenBalance
			if err := tx.Where("wallet_address = ?", strings.ToLower(walletAddress)).First(&balance).Error; err != nil {
				return errors.New("balance not found")
			}
			availableBalance = balance.Balance
			
			if availableBalance.LessThan(amount) {
				return errors.New("insufficient balance")
			}
			
			// 锁定余额
			balance.Balance = balance.Balance.Sub(amount)
			balance.LockedBalance = balance.LockedBalance.Add(amount)
			if err := tx.Save(&balance).Error; err != nil {
				return err
			}
		} else if userType == "agent" {
			var balance models.AgentTokenBalance
			if err := tx.Where("agent_id = ?", userID).First(&balance).Error; err != nil {
				return errors.New("agent balance not found")
			}
			availableBalance = balance.Balance
			
			if availableBalance.LessThan(amount) {
				return errors.New("insufficient balance")
			}
			
			// 锁定余额
			balance.Balance = balance.Balance.Sub(amount)
			balance.LockedBalance = balance.LockedBalance.Add(amount)
			if err := tx.Save(&balance).Error; err != nil {
				return err
			}
		} else {
			return errors.New("invalid user type")
		}
		
		// 计算手续费
		feeRate := decimal.NewFromFloat(s.cfg.WithdrawFeeRate)
		fee := amount.Mul(feeRate).Round(18)
		netAmount := amount.Sub(fee)
		
		withdrawal = &models.Withdrawal{
			WalletAddress: strings.ToLower(walletAddress),
			UserType:      userType,
			UserID:        userID,
			Amount:        amount,
			Fee:           fee,
			NetAmount:     netAmount,
			Status:        "pending",
		}
		
		return tx.Create(withdrawal).Error
	})
	
	return withdrawal, err
}

// ProcessWithdrawal 处理提现（转账上链）
func (s *TokenService) ProcessWithdrawal(withdrawalID uint) error {
	var withdrawal models.Withdrawal
	if err := s.db.First(&withdrawal, withdrawalID).Error; err != nil {
		return err
	}
	
	if withdrawal.Status != "pending" {
		return errors.New("withdrawal already processed")
	}
	
	// 更新状态为处理中
	withdrawal.Status = "processing"
	if err := s.db.Save(&withdrawal).Error; err != nil {
		return err
	}
	
	// 发送链上交易
	txHash, err := s.sendTokenTransfer(withdrawal.WalletAddress, withdrawal.NetAmount)
	if err != nil {
		// 转账失败，恢复余额
		withdrawal.Status = "failed"
		withdrawal.FailReason = err.Error()
		s.db.Save(&withdrawal)
		
		// 解锁余额
		s.unlockBalance(withdrawal.UserType, withdrawal.UserID, withdrawal.WalletAddress, withdrawal.Amount)
		return err
	}
	
	// 更新提现记录
	now := time.Now()
	withdrawal.TxHash = txHash
	withdrawal.Status = "completed"
	withdrawal.ProcessedAt = &now
	
	if err := s.db.Save(&withdrawal).Error; err != nil {
		return err
	}
	
	// 从锁定余额中扣除
	return s.confirmWithdrawal(withdrawal.UserType, withdrawal.UserID, withdrawal.WalletAddress, withdrawal.Amount)
}

// sendTokenTransfer 发送ERC20代币转账
func (s *TokenService) sendTokenTransfer(toAddress string, amount decimal.Decimal) (string, error) {
	if s.cfg.PlatformPrivateKey == "" {
		return "", errors.New("platform private key not configured")
	}
	
	privateKey, err := crypto.HexToECDSA(s.cfg.PlatformPrivateKey)
	if err != nil {
		return "", err
	}
	
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	
	// 构造ERC20 transfer调用数据
	tokenAddr := common.HexToAddress(s.cfg.TokenContractAddr)
	toAddr := common.HexToAddress(toAddress)
	
	// transfer(address,uint256) 函数选择器
	transferFnSignature := []byte("transfer(address,uint256)")
	hash := crypto.Keccak256Hash(transferFnSignature)
	methodID := hash[:4]
	
	// 构造参数
	paddedAddress := common.LeftPadBytes(toAddr.Bytes(), 32)
	
	// 将decimal转为big.Int (假设18位精度)
	amountBig := amount.Shift(18).BigInt()
	paddedAmount := common.LeftPadBytes(amountBig.Bytes(), 32)
	
	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)
	
	// 获取nonce
	nonce, err := s.client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return "", err
	}
	
	// 获取gas价格
	gasPrice, err := s.client.SuggestGasPrice(context.Background())
	if err != nil {
		return "", err
	}
	
	// 估算gas
	gasLimit, err := s.client.EstimateGas(context.Background(), ethereum.CallMsg{
		From: fromAddress,
		To:   &tokenAddr,
		Data: data,
	})
	if err != nil {
		gasLimit = 100000 // 默认gas限制
	}
	
	// 构造交易
	tx := types.NewTransaction(nonce, tokenAddr, big.NewInt(0), gasLimit, gasPrice, data)
	
	// 签名
	chainID, err := s.client.NetworkID(context.Background())
	if err != nil {
		return "", err
	}
	
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return "", err
	}
	
	// 发送交易
	err = s.client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", err
	}
	
	return signedTx.Hash().Hex(), nil
}

// unlockBalance 解锁余额（提现失败时）
func (s *TokenService) unlockBalance(userType string, userID uint, walletAddress string, amount decimal.Decimal) error {
	if userType == "user" {
		return s.db.Model(&models.TokenBalance{}).
			Where("wallet_address = ?", strings.ToLower(walletAddress)).
			Updates(map[string]interface{}{
				"balance":        gorm.Expr("balance + ?", amount),
				"locked_balance": gorm.Expr("locked_balance - ?", amount),
			}).Error
	} else {
		return s.db.Model(&models.AgentTokenBalance{}).
			Where("agent_id = ?", userID).
			Updates(map[string]interface{}{
				"balance":        gorm.Expr("balance + ?", amount),
				"locked_balance": gorm.Expr("locked_balance - ?", amount),
			}).Error
	}
}

// confirmWithdrawal 确认提现（从锁定余额中扣除）
func (s *TokenService) confirmWithdrawal(userType string, userID uint, walletAddress string, amount decimal.Decimal) error {
	if userType == "user" {
		return s.db.Model(&models.TokenBalance{}).
			Where("wallet_address = ?", strings.ToLower(walletAddress)).
			Updates(map[string]interface{}{
				"locked_balance":   gorm.Expr("locked_balance - ?", amount),
				"total_withdrawn":  gorm.Expr("total_withdrawn + ?", amount),
			}).Error
	} else {
		return s.db.Model(&models.AgentTokenBalance{}).
			Where("agent_id = ?", userID).
			Updates(map[string]interface{}{
				"locked_balance":   gorm.Expr("locked_balance - ?", amount),
				"total_withdrawn":  gorm.Expr("total_withdrawn + ?", amount),
			}).Error
	}
}

// StartWithdrawalProcessor 启动提现自动处理（在单独goroutine中运行）
func (s *TokenService) StartWithdrawalProcessor(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Withdrawal processor stopped")
			return
		case <-ticker.C:
			s.processPendingWithdrawals()
		}
	}
}

// processPendingWithdrawals 处理所有pending状态的提现
func (s *TokenService) processPendingWithdrawals() {
	if s.cfg.PlatformPrivateKey == "" {
		return // 没有私钥，跳过
	}

	var withdrawals []models.Withdrawal
	if err := s.db.Where("status = ?", "pending").Order("created_at asc").Limit(10).Find(&withdrawals).Error; err != nil {
		log.Printf("Failed to get pending withdrawals: %v", err)
		return
	}

	for _, w := range withdrawals {
		log.Printf("Processing withdrawal #%d: %s to %s", w.ID, w.NetAmount.String(), w.WalletAddress)
		if err := s.ProcessWithdrawal(w.ID); err != nil {
			log.Printf("Failed to process withdrawal #%d: %v", w.ID, err)
		} else {
			log.Printf("Withdrawal #%d completed", w.ID)
		}
	}
}

// ==================== 充值监听 ====================

// StartDepositWatcher 启动充值监听（在单独goroutine中运行）
func (s *TokenService) StartDepositWatcher(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second) // 每15秒检查一次
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Deposit watcher stopped")
			return
		case <-ticker.C:
			s.checkDeposits()
		}
	}
}

// checkDeposits 检查新的充值
func (s *TokenService) checkDeposits() {
	// 获取所有活跃的充值地址
	var addresses []models.DepositAddress
	if err := s.db.Where("is_active = ? AND assigned_to IS NOT NULL AND assigned_to != ''", true).Find(&addresses).Error; err != nil {
		log.Printf("Failed to get deposit addresses: %v", err)
		return
	}
	
	if len(addresses) == 0 {
		return
	}
	
	// 获取当前区块高度
	header, err := s.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Printf("Failed to get block header: %v", err)
		return
	}
	currentBlock := header.Number.Uint64()
	
	// 查询代币Transfer事件
	tokenAddr := common.HexToAddress(s.cfg.TokenContractAddr)
	
	for _, addr := range addresses {
		s.checkAddressDeposits(addr, tokenAddr, currentBlock)
	}
}

// checkAddressDeposits 检查单个地址的充值
func (s *TokenService) checkAddressDeposits(addr models.DepositAddress, tokenAddr common.Address, currentBlock uint64) {
	toAddr := common.HexToAddress(addr.Address)
	
	// 从最近1000个区块中查找（约1小时）
	fromBlock := currentBlock - 1000
	if fromBlock < 0 {
		fromBlock = 0
	}
	
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(currentBlock)),
		Addresses: []common.Address{tokenAddr},
		Topics: [][]common.Hash{
			{transferEventSig},
			nil, // from address (any)
			{common.BytesToHash(common.LeftPadBytes(toAddr.Bytes(), 32))}, // to address
		},
	}
	
	logs, err := s.client.FilterLogs(context.Background(), query)
	if err != nil {
		log.Printf("Failed to filter logs for %s: %v", addr.Address, err)
		return
	}
	
	for _, vLog := range logs {
		s.processDepositLog(addr, vLog, currentBlock)
	}
}

// processDepositLog 处理充值日志
func (s *TokenService) processDepositLog(addr models.DepositAddress, vLog types.Log, currentBlock uint64) {
	txHash := vLog.TxHash.Hex()
	
	// 检查是否已处理
	var existing models.Deposit
	if err := s.db.Where("tx_hash = ?", txHash).First(&existing).Error; err == nil {
		// 已存在，检查是否需要确认
		if existing.Status == "pending" {
			confirms := currentBlock - existing.BlockNumber
			if confirms >= uint64(s.cfg.DepositConfirms) {
				s.ProcessDeposit(&existing)
			}
		}
		return
	}
	
	// 解析Transfer事件
	if len(vLog.Data) < 32 {
		return
	}
	
	amount := new(big.Int).SetBytes(vLog.Data[:32])
	amountDecimal := decimal.NewFromBigInt(amount, -18) // 假设18位精度
	
	// 检查最低充值金额
	if amountDecimal.LessThan(decimal.NewFromFloat(s.cfg.MinDepositAmount)) {
		log.Printf("Deposit amount too small: %s", amountDecimal.String())
		return
	}
	
	// 创建充值记录
	deposit := models.Deposit{
		WalletAddress:  addr.AssignedTo,
		DepositAddress: addr.Address,
		TxHash:         txHash,
		BlockNumber:    vLog.BlockNumber,
		Amount:         amountDecimal,
		Status:         "pending",
	}
	
	if err := s.db.Create(&deposit).Error; err != nil {
		log.Printf("Failed to create deposit record: %v", err)
		return
	}
	
	// 检查确认数
	confirms := currentBlock - vLog.BlockNumber
	if confirms >= uint64(s.cfg.DepositConfirms) {
		s.ProcessDeposit(&deposit)
	}
}
