package smcrypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/emmansun/gmsm/sm9"
)

// SM9HidSign SM9 签名功能标识符（国标定义）
const SM9HidSign byte = 1

// SM9GenerateSignMasterKey 生成 SM9 签名主密钥对
// 返回：主私钥、主公钥、错误
func SM9GenerateSignMasterKey() (*sm9.SignMasterPrivateKey, *sm9.SignMasterPublicKey, error) {
	masterPriv, err := sm9.GenerateSignMasterKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("sm9: generate master key failed: %w", err)
	}
	return masterPriv, masterPriv.PublicKey(), nil
}

// SM9GenerateSignKey 由主私钥和用户标识派生用户签名私钥
// uid: 用户标识（如 agent_id），建议格式 "agent_id@domain"
func SM9GenerateSignKey(masterPriv *sm9.SignMasterPrivateKey, uid []byte) (*sm9.SignPrivateKey, error) {
	if masterPriv == nil {
		return nil, fmt.Errorf("sm9: master private key is nil")
	}
	userKey, err := masterPriv.GenerateUserKey(uid, SM9HidSign)
	if err != nil {
		return nil, fmt.Errorf("sm9: generate sign key failed: %w", err)
	}
	return userKey, nil
}

// SM9Sign 使用 SM9 用户私钥对消息哈希值进行签名
// hash: 待签名消息的哈希值（建议使用 SM3）
// 返回：ASN.1 DER 编码的签名字节数组
func SM9Sign(priv *sm9.SignPrivateKey, hash []byte) ([]byte, error) {
	if priv == nil {
		return nil, fmt.Errorf("sm9: private key is nil")
	}
	sig, err := sm9.SignASN1(rand.Reader, priv, hash)
	if err != nil {
		return nil, fmt.Errorf("sm9: sign failed: %w", err)
	}
	return sig, nil
}

// SM9SignHex 签名并返回十六进制字符串
func SM9SignHex(priv *sm9.SignPrivateKey, hash []byte) (string, error) {
	sig, err := SM9Sign(priv, hash)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sig), nil
}

// SM9Verify 使用 SM9 主公钥和用户标识验证签名
// uid: 用户标识（必须与生成用户私钥时使用的标识一致）
// hash: 原始消息的哈希值
// sig: ASN.1 DER 编码的签名
func SM9Verify(pub *sm9.SignMasterPublicKey, uid []byte, hash, sig []byte) bool {
	if pub == nil {
		return false
	}
	return sm9.VerifyASN1(pub, uid, SM9HidSign, hash, sig)
}

// SM9VerifyHex 验证十六进制编码的签名
func SM9VerifyHex(pub *sm9.SignMasterPublicKey, uid []byte, hash []byte, sigHex string) bool {
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	return SM9Verify(pub, uid, hash, sig)
}
