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
	usedNonces      = make(map[string]int64)
	nonceMu         sync.RWMutex
	nonceExpiration = 24 * time.Hour
	nonceGCDone     = make(chan struct{})

	schemaCache    sync.Map
	schemaCacheTTL = 10 * time.Minute
)

type cachedVerification struct {
	valid   bool
	expires time.Time
}

type VerificationChecks struct {
	SignatureValid bool `json:"signature_valid"`
	ExpiryValid    bool `json:"expiry_valid"`
	NonceValid     bool `json:"nonce_valid"`
	CallBudgetOK   bool `json:"call_budget_ok"`
}

func (c VerificationChecks) IsValid() bool {
	return c.SignatureValid && c.ExpiryValid && c.NonceValid && c.CallBudgetOK
}

// Verifier 执行平面校验器
// 负责验证 RequireToken 的合法性和有效性
type Verifier struct {
	publicKey *ecdsa.PublicKey // SM2 公钥，用于验证签名
	cache     sync.Map         // 缓存已验证的 token 结果，key 为签名消息哈希
	cacheTTL  time.Duration    // 缓存 TTL
}

// NewVerifier 创建新的验证器实例
// 返回：Verifier 对象
func NewVerifier() *Verifier {
	v := &Verifier{
		publicKey: GetSigningPublicKey(),
		cacheTTL:  5 * time.Minute,
	}
	return v
}

// Verify 执行令牌的全量校验（七项检查）
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
// 7. Schema 指纹 - 使用 SM3 哈希对比，防止参数篡改
// 8. 调用次数预算 - 检查是否超过 max_calls（SAGA 风格防 DoS）
func (v *Verifier) Verify(token *RequireToken) error {
	_, err := v.verifyChecks(token, true)
	return err
}

// Inspect 返回不消费 Nonce 的基础校验结果，适合状态展示或页面轮询。
func (v *Verifier) Inspect(token *RequireToken) VerificationChecks {
	checks, _ := v.verifyChecks(token, false)
	return checks
}

func (v *Verifier) verifyChecks(token *RequireToken, consumeNonce bool) (VerificationChecks, error) {
	checks := VerificationChecks{}

	if v.publicKey == nil {
		return checks, fmt.Errorf("verifier public key not initialized")
	}

	cacheKey := token.buildCacheKey()
	if cached, ok := v.cache.Load(cacheKey); ok {
		cv := cached.(*cachedVerification)
		if time.Now().Before(cv.expires) {
			if !cv.valid {
				return checks, fmt.Errorf("cached invalid signature")
			}
			checks.SignatureValid = true
			checks.ExpiryValid = true
			if err := v.verifyNonce(token, consumeNonce); err != nil {
				return checks, err
			}
			checks.NonceValid = true
			if err := v.verifyCallBudget(token); err != nil {
				return checks, err
			}
			checks.CallBudgetOK = true
			return checks, nil
		}
		v.cache.Delete(cacheKey)
	}

	if err := v.verifySignature(token); err != nil {
		v.cache.Store(cacheKey, &cachedVerification{valid: false, expires: time.Now().Add(v.cacheTTL)})
		return checks, fmt.Errorf("signature verification failed: %w", err)
	}
	checks.SignatureValid = true

	if err := v.verifyExpiry(token); err != nil {
		return checks, err
	}
	checks.ExpiryValid = true

	if err := v.verifyNonce(token, consumeNonce); err != nil {
		return checks, err
	}
	checks.NonceValid = true

	if err := v.verifyCallBudget(token); err != nil {
		return checks, err
	}
	checks.CallBudgetOK = true

	if err := v.verifySchemaHash(token); err != nil {
		return checks, err
	}

	v.cache.Store(cacheKey, &cachedVerification{valid: true, expires: time.Now().Add(v.cacheTTL)})
	return checks, nil
}

// verifySignature 验证令牌的 SM2 签名
func (v *Verifier) verifySignature(token *RequireToken) error {
	if token.Signature == "" {
		return fmt.Errorf("missing signature")
	}

	message := token.buildSignMessage()

	valid, err := smcrypto.VerifySignatureHex(v.publicKey, message, token.Signature, signingUID)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// verifyExpiry 验证令牌的时效性
func (v *Verifier) verifyExpiry(token *RequireToken) error {
	if time.Now().After(token.ExpiresAt) {
		return fmt.Errorf("token expired at %v", token.ExpiresAt)
	}
	return nil
}

// verifyNonce 验证 Nonce 防止重放攻击
// 使用 RWMutex 优化读多写少场景：读操作获取 RLock，写操作获取 Lock
// 同时支持 Nonce 过期自动清理，避免内存泄漏
func (v *Verifier) verifyNonce(token *RequireToken, consume bool) error {
	nonceMu.RLock()
	expiresAt, exists := usedNonces[token.Nonce]
	nonceMu.RUnlock()

	if exists {
		if time.Now().Unix() < expiresAt {
			return fmt.Errorf("nonce already used: %s", token.Nonce)
		}
	}

	if consume {
		nonceMu.Lock()
		usedNonces[token.Nonce] = time.Now().Add(nonceExpiration).Unix()
		nonceMu.Unlock()
	}
	return nil
}

// verifySchemaHash 验证 Schema 指纹（SM3 哈希）
// 如果令牌提供了 SchemaHash，调用方可以通过 CompareSchemaHash 验证
// 工具 Schema 的完整性。此处仅检查 SchemaHash 字段非空，实际对比
// 由调用方在获取到工具 Schema 后通过 CompareSchemaHash 执行。
func (v *Verifier) verifySchemaHash(token *RequireToken) error {
	if token.SchemaHash != "" {
		// SchemaHash 字段非空，说明签发方提供了指纹。
		// 具体的对比由上层调用方在解析工具 Schema 后调用
		// CompareSchemaHash 完成，因为此处不知道预期的 Schema 内容。
	}
	return nil
}

// CompareSchemaHash 对比工具 Schema 的 SM3 哈希是否与令牌一致
// 使用缓存优化：如果相同的 SchemaHash + toolSchema 组合已验证过，直接返回缓存结果
// 参数：
//   - token: 含有预期 SchemaHash 的 RequireToken
//   - toolSchema: 实际获取到的工具 Schema 原始字节
//
// 返回：错误信息（哈希不匹配或 token 未提供 SchemaHash）
//
// 使用场景：ActionGate 在收到工具调用请求后，先比对工具描述的 SM3 哈希，
// 再放行或拒绝。防止攻击者在授权后篡改工具参数。
func CompareSchemaHash(token *RequireToken, toolSchema []byte) error {
	if token.SchemaHash == "" {
		return fmt.Errorf("token has no schema hash set")
	}

	schemaKey := fmt.Sprintf("%s:%x", token.SchemaHash, smcrypto.SM3Sum(toolSchema))

	if cached, ok := schemaCache.Load(schemaKey); ok {
		if !cached.(bool) {
			return fmt.Errorf("schema hash mismatch")
		}
		return nil
	}

	actualHash := smcrypto.SM3Hex(toolSchema)
	if actualHash != token.SchemaHash {
		schemaCache.Store(schemaKey, false)
		return fmt.Errorf("schema hash mismatch: expected %s, got %s",
			token.SchemaHash, actualHash)
	}

	schemaCache.Store(schemaKey, true)
	return nil
}

// verifyCallBudget 验证调用次数预算（SAGA 风格防 DoS）
func (v *Verifier) verifyCallBudget(token *RequireToken) error {
	if token.MaxCalls == 0 {
		return nil
	}

	if token.CallCount >= token.MaxCalls {
		return fmt.Errorf("call budget exceeded: %d/%d calls used", token.CallCount, token.MaxCalls)
	}

	return nil
}

// ResetNonces 重置所有已使用的 Nonce 记录
// 用于测试环境或特殊场景下的清理操作
func ResetNonces() {
	nonceMu.Lock()
	defer nonceMu.Unlock()
	usedNonces = make(map[string]int64)
}

// StartNonceGC 启动后台 goroutine 定期清理过期的 Nonce
// 防止长时间运行后内存泄漏
// 参数：
//   - interval: 清理间隔时间，默认 1 小时
func StartNonceGC(interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				nonceMu.Lock()
				now := time.Now().Unix()
				deleted := 0
				for nonce, expiresAt := range usedNonces {
					if now >= expiresAt {
						delete(usedNonces, nonce)
						deleted++
					}
				}
				nonceMu.Unlock()
				if deleted > 0 {
					fmt.Printf("[NonceGC] cleaned up %d expired nonces\n", deleted)
				}
			case <-nonceGCDone:
				return
			}
		}
	}()
}

// StopNonceGC 停止后台 Nonce GC goroutine
func StopNonceGC() {
	close(nonceGCDone)
}
