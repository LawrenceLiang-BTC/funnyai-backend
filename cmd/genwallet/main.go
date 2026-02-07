package main

import (
	"crypto/ecdsa"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	// 生成新的私钥
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	// 获取私钥的十六进制表示
	privateKeyBytes := crypto.FromECDSA(privateKey)
	privateKeyHex := hexutil.Encode(privateKeyBytes)

	// 获取公钥
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	// 从公钥生成地址
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	fmt.Println("=== BSC 测试钱包生成 ===")
	fmt.Println("⚠️  请妥善保管私钥，不要泄露！")
	fmt.Println("")
	fmt.Println("钱包地址:", address)
	fmt.Println("私钥:", privateKeyHex)
	fmt.Println("")
	fmt.Println("将以下内容添加到 .env 文件：")
	fmt.Println("PLATFORM_WALLET=" + address)
	fmt.Println("PLATFORM_PRIVATE_KEY=" + privateKeyHex)
}
