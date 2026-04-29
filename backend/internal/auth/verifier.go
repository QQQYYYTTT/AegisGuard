// backend/internal/auth/verifier.go
// Package auth 实现 AegisGuard 的授权令牌验证机制
// 执行平面（Agent）使用此模块验证令牌的合法性和有效性
package auth

import (
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"aegisguard/pkg/smcrypto"
)

var (
	usedNonces = make(map[string]bool)
	nonceMu    sync.Mutex
)

// Verifier 执行平面校验器
// 负责验证 RequireToken 的合法性和有效性
type Verifier struct {
	publicKey *ecdsa.PublicKey // SM2 公钥，用于验证签名
}

// NewVerifier 创建新的验证器实例
// 返回：Verifier 对象
func NewVerifier() *Verifier {
	return &Verifier{
		// 获取全局签名公钥
		publicKey: GetSigningPublicKey(),
	}
}

// Verify 执行令牌的全量校验（八项检查）
// 参数：
//   - token: 待验证的 RequireToken
//
// 返回：错误信息（如果验证通过则返回 nil）
//
// 校验项目：
// 1. 签名有效性 - 使用 SM2 验签
// 2. 时效性 - 检查是否过期
// 3. Nonce 防重放 - 检查是否重复使用
// 4. Agent 身份 - 在 ActionGate 中检查
// 5. 会话绑定 - 在 ActionGate 中检查
// 6. 权限范围 - 在 ActionGate 中检查
// 7. Schema 指纹 - 预留，用于防止参数篡改
// 8. 调用次数预算 - 检查是否超过 max_calls（SAGA 风格防 DoS）
func (v *Verifier) Verify(token *RequireToken) error {
	// 检查公钥是否已初始化
	if v.publicKey == nil {
		return fmt.Errorf("verifier public key not initialized")
	}

	// 1. 验证签名有效性
	if err := v.verifySignature(token); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// 2. 验证时效性
	if err := v.verifyExpiry(token); err != nil {
		return err
	}

	// 3. 验证 Nonce 防重放
	if err := v.verifyNonce(token); err != nil {
		return err
	}

	// 8. 验证调用次数预算（SAGA 风格防 DoS）
	if err := v.verifyCallBudget(token); err != nil {
		return err
	}

	// 7. 验证 Schema 指纹（预留）
	if err := v.verifySchemaHash(token); err != nil {
		return err
	}

	// 注意：4-6 项在 ActionGate 中检查
	return nil
}

// verifySignature 验证令牌的 SM2 签名
// 参数：token - 待验证的令牌
// 返回：错误信息
func (v *Verifier) verifySignature(token *RequireToken) error {
	// 检查签名是否存在
	if token.Signature == "" {
		return fmt.Errorf("missing signature")
	}

	// 构建与签名时相同的消息体
	message := token.buildSignMessage()

	// 使用 SM2 算法验证签名
	valid, err := smcrypto.VerifySignatureHex(v.publicKey, message, token.Signature, signingUID)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	// 检查签名是否有效
	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// verifyExpiry 验证令牌的时效性
// 参数：token - 待验证的令牌
// 返回：错误信息（如果已过期）
func (v *Verifier) verifyExpiry(token *RequireToken) error {
	// 检查当前时间是否超过过期时间
	if time.Now().After(token.ExpiresAt) {
		return fmt.Errorf("token expired at %v", token.ExpiresAt)
	}
	return nil
}

// verifyNonce 验证 Nonce 防止重放攻击
// 参数：token - 待验证的令牌
// 返回：错误信息（如果 Nonce 已使用）
func (v *Verifier) verifyNonce(token *RequireToken) error {
	nonceMu.Lock()
	defer nonceMu.Unlock()
	if usedNonces[token.Nonce] {
		return fmt.Errorf("nonce already used: %s", token.Nonce)
	}
	usedNonces[token.Nonce] = true
	return nil
}

// verifySchemaHash 验证 Schema 指纹（预留功能）
// 参数：token - 待验证的令牌
// 返回：错误信息
// 注意：当前版本仅做占位，后续将实现 SM3 哈希比对
func (v *Verifier) verifySchemaHash(token *RequireToken) error {
	// 如果提供了 SchemaHash，后续将验证工具描述的完整性
	if token.SchemaHash != "" {
		// TODO: 对比工具描述的 SM3 哈希值
	}
	return nil
}

// verifyCallBudget 验证调用次数预算（SAGA 风格防 DoS）
// 参数：token - 待验证的令牌
// 返回：错误信息（如果超过预算）
// 注意：这里只检查是否超过预算，不自动递增 CallCount
// CallCount 的递增由上层应用逻辑控制（如 ActionGate）
func (v *Verifier) verifyCallBudget(token *RequireToken) error {
	// 如果 MaxCalls 为 0，表示无限制
	if token.MaxCalls == 0 {
		return nil
	}

	// 检查当前调用次数是否已达到或超过最大限制
	if token.CallCount >= token.MaxCalls {
		return fmt.Errorf("call budget exceeded: %d/%d calls used", token.CallCount, token.MaxCalls)
	}

	return nil
}

// ResetNonces 重置所有已使用的 Nonce 记录
// 用于测试环境或特殊场景下的清理操作
// 注意：生产环境应谨慎使用此方法
func (v *Verifier) ResetNonces() {
	usedNonces = make(map[string]bool)
}

// ResetNonces 包级别的 Nonce 重置函数（用于测试）
func ResetNonces() {
	usedNonces = make(map[string]bool)
}
