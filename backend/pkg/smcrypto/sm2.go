// Package smcrypto 提供国密算法封装，包括 SM2 签名/验签、SM3 哈希、SM4 加密等
package smcrypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/emmansun/gmsm/sm2"
)

// defaultUID 是 SM2 签名的默认用户标识符
// 根据国密标准，SM2 签名需要结合用户 ID 进行计算
var (
	defaultUID = []byte("1234567812345678")
)

// SM2KeyPair 表示 SM2 密钥对，包含私钥和公钥
type SM2KeyPair struct {
	PrivateKey *sm2.PrivateKey  // SM2 私钥，用于签名和解密
	PublicKey  *ecdsa.PublicKey // SM2 公钥，用于验签和加密
}

// GenerateKeyPair 生成新的 SM2 密钥对
// 返回：密钥对对象或错误
func GenerateKeyPair() (*SM2KeyPair, error) {
	// 使用加密安全的随机数生成器生成密钥对
	privKey, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SM2 key pair: %w", err)
	}

	// 返回密钥对，公钥从私钥中提取
	return &SM2KeyPair{
		PrivateKey: privKey,
		PublicKey:  &privKey.PublicKey,
	}, nil
}

// LoadPrivateKeyFromHex 从十六进制字符串加载 SM2 私钥
// 参数：privHex - 私钥的十六进制表示（64 个字符，32 字节）
// 返回：SM2 私钥对象或错误
func LoadPrivateKeyFromHex(privHex string) (*sm2.PrivateKey, error) {
	// 将十六进制字符串解码为字节数组
	privBytes, err := hex.DecodeString(privHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key hex: %w", err)
	}

	// 从字节数组解析 SM2 私钥
	privKey, err := sm2.NewPrivateKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	return privKey, nil
}

// LoadPublicKeyFromHex 从十六进制字符串加载 SM2 公钥
// 参数：pubHex - 公钥的十六进制表示（130 个字符，65 字节，未压缩格式）
// 返回：ECDSA 公钥对象或错误
func LoadPublicKeyFromHex(pubHex string) (*ecdsa.PublicKey, error) {
	// 将十六进制字符串解码为字节数组
	pubBytes, err := hex.DecodeString(pubHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key hex: %w", err)
	}

	// 从字节数组解析 SM2 公钥
	pubKey, err := sm2.NewPublicKey(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load public key: %w", err)
	}

	return pubKey, nil
}

// SignMessage 使用 SM2 私钥对消息进行签名
// 参数：
//   - privKey: SM2 私钥
//   - message: 待签名的原始消息
//   - uid: 用户标识符，如果为 nil 则使用默认 UID
// 返回：ASN.1 编码的签名字节数组或错误
func SignMessage(privKey *sm2.PrivateKey, message []byte, uid []byte) ([]byte, error) {
	// 如果未提供 UID，使用默认值
	if uid == nil {
		uid = defaultUID
	}

	// 使用 SM2 算法进行签名（符合 GB/T 32918.2-2016 标准）
	signature, err := privKey.SignWithSM2(rand.Reader, uid, message)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	return signature, nil
}

// SignMessageHex 使用 SM2 私钥对消息进行签名，并返回十六进制字符串
// 参数：同 SignMessage
// 返回：十六进制编码的签名或错误
func SignMessageHex(privKey *sm2.PrivateKey, message []byte, uid []byte) (string, error) {
	// 调用 SignMessage 获取签名
	signature, err := SignMessage(privKey, message, uid)
	if err != nil {
		return "", err
	}

	// 将签名转换为十六进制字符串便于传输和存储
	return hex.EncodeToString(signature), nil
}

// VerifySignature 使用 SM2 公钥验证签名
// 参数：
//   - pubKey: SM2 公钥
//   - message: 原始消息
//   - signature: ASN.1 编码的签名
//   - uid: 用户标识符，如果为 nil 则使用默认 UID
// 返回：签名是否有效
func VerifySignature(pubKey *ecdsa.PublicKey, message []byte, signature []byte, uid []byte) bool {
	// 如果未提供 UID，使用默认值
	if uid == nil {
		uid = defaultUID
	}

	// 使用 SM2 算法验证签名（符合 GB/T 32918.2-2016 标准）
	return sm2.VerifyASN1WithSM2(pubKey, uid, message, signature)
}

// VerifySignatureHex 使用 SM2 公钥验证十六进制编码的签名
// 参数：
//   - pubKey: SM2 公钥
//   - message: 原始消息
//   - signatureHex: 十六进制编码的签名
//   - uid: 用户标识符，如果为 nil 则使用默认 UID
// 返回：签名是否有效及错误信息
func VerifySignatureHex(pubKey *ecdsa.PublicKey, message []byte, signatureHex string, uid []byte) (bool, error) {
	// 将十六进制签名解码为字节数组
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature hex: %w", err)
	}

	// 如果未提供 UID，使用默认值
	if uid == nil {
		uid = defaultUID
	}

	// 使用 SM2 算法验证签名
	return sm2.VerifyASN1WithSM2(pubKey, uid, message, signature), nil
}

// GetPublicKeyHex 将 SM2 公钥转换为十六进制字符串
// 参数：pubKey - SM2 公钥
// 返回：十六进制编码的公钥（未压缩格式，130 个字符）
func GetPublicKeyHex(pubKey *ecdsa.PublicKey) string {
	// 获取公钥的字节表示（忽略错误，因为公钥总是有效的）
	bytes, _ := pubKey.Bytes()
	return hex.EncodeToString(bytes)
}

// GetPrivateKeyHex 将 SM2 私钥转换为十六进制字符串
// 参数：privKey - SM2 私钥
// 返回：十六进制编码的私钥（64 个字符）
func GetPrivateKeyHex(privKey *sm2.PrivateKey) string {
	// 获取私钥的 D 值（大整数）并转换为字节数组
	return hex.EncodeToString(privKey.D.Bytes())
}
