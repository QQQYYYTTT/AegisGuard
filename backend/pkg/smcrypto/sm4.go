package smcrypto

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	gmsm4 "github.com/emmansun/gmsm/sm4"
)

// SM4BlockSize SM4 分组大小（128 位 = 16 字节）
const SM4BlockSize = 16

// SM4GenerateKey 生成 SM4 密钥（16 字节 = 128 位）
func SM4GenerateKey() ([]byte, error) {
	key := make([]byte, SM4BlockSize)
	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		return nil, fmt.Errorf("sm4: generate key failed: %w", err)
	}
	return key, nil
}

// SM4GenerateKeyHex 生成 SM4 密钥并返回十六进制字符串
func SM4GenerateKeyHex() (string, error) {
	key, err := SM4GenerateKey()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// SM4EncryptCBC 使用 SM4-CBC 模式加密明文，自动生成随机 IV
// 返回：IV + 密文的拼接字节数组
func SM4EncryptCBC(key, plaintext []byte) ([]byte, error) {
	block, err := gmsm4.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("sm4: new cipher failed: %w", err)
	}

	iv := make([]byte, SM4BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("sm4: generate iv failed: %w", err)
	}

	padded := pkcs7Pad(plaintext, SM4BlockSize)
	ciphertext := make([]byte, len(padded))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	return append(iv, ciphertext...), nil
}

// SM4EncryptCBCBase64 使用 SM4-CBC 加密，返回 Hex 字符串
func SM4EncryptCBCBase64(key []byte, plaintext []byte) (string, error) {
	encrypted, err := SM4EncryptCBC(key, plaintext)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(encrypted), nil
}

// SM4DecryptCBC 使用 SM4-CBC 模式解密
// data 格式：前 16 字节为 IV，后续为密文
func SM4DecryptCBC(key, data []byte) ([]byte, error) {
	if len(data) < SM4BlockSize*2 {
		return nil, fmt.Errorf("sm4: ciphertext too short")
	}

	block, err := gmsm4.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("sm4: new cipher failed: %w", err)
	}

	iv := data[:SM4BlockSize]
	ciphertext := data[SM4BlockSize:]

	if len(ciphertext)%SM4BlockSize != 0 {
		return nil, fmt.Errorf("sm4: ciphertext is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	padded := make([]byte, len(ciphertext))
	mode.CryptBlocks(padded, ciphertext)

	return pkcs7Unpad(padded)
}

// pkcs7Pad PKCS7 填充
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	pad := make([]byte, padding)
	for i := range pad {
		pad[i] = byte(padding)
	}
	return append(data, pad...)
}

// pkcs7Unpad 去除 PKCS7 填充
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("sm4: empty data")
	}
	padding := int(data[len(data)-1])
	if padding < 1 || padding > len(data) {
		return nil, fmt.Errorf("sm4: invalid padding")
	}
	for _, v := range data[len(data)-padding:] {
		if int(v) != padding {
			return nil, fmt.Errorf("sm4: invalid padding byte")
		}
	}
	return data[:len(data)-padding], nil
}
