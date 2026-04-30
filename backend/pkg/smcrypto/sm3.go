// Package smcrypto 国密算法封装 — SM3 哈希
package smcrypto

import (
	"encoding/hex"

	"github.com/emmansun/gmsm/sm3"
)

// SM3Sum 计算 SM3 哈希值（256 位）
func SM3Sum(data []byte) []byte {
	h := sm3.New()
	h.Write(data)
	return h.Sum(nil)
}

// SM3Hex 计算 SM3 哈希值并返回十六进制小写字符串
func SM3Hex(data []byte) string {
	return hex.EncodeToString(SM3Sum(data))
}

// SM3SumTruncated 计算 body 前 maxBytes 字节的 SM3 哈希
// 用于审计日志 body 摘要，避免完整存储敏感 payload
// maxBytes: 参与哈希计算的截取长度
// 如果 data 长度小于 maxBytes，取全部
func SM3SumTruncated(data []byte, maxBytes int) []byte {
	if len(data) <= maxBytes {
		return SM3Sum(data)
	}
	return SM3Sum(data[:maxBytes])
}

// SM3HexTruncated 截断 SM3 哈希的十六进制版本
func SM3HexTruncated(data []byte, maxBytes int) string {
	return hex.EncodeToString(SM3SumTruncated(data, maxBytes))
}
