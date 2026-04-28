package auth

import (
	"testing"
	"time"
)

// TestTokenSignAndVerify 测试令牌的签名和验证流程
// 验证完整的令牌生命周期：创建 -> 签名 -> 验证
func TestTokenSignAndVerify(t *testing.T) {
	// 1. 初始化签名密钥（自动生成）
	if err := InitSigningKey(""); err != nil {
		t.Fatalf("Failed to init signing key: %v", err)
	}

	// 2. 创建新令牌（有效期 5 分钟）
	token, err := NewToken("test-tool", "read", "agent-001", "session-123", "task-456", 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	// 3. 检查签名是否已生成
	if token.Signature == "" {
		t.Fatal("Token signature is empty")
	}

	// 4. 创建验证器并验证令牌
	verifier := NewVerifier()
	if err := verifier.Verify(token); err != nil {
		t.Fatalf("Token verification failed: %v", err)
	}

	// 5. 输出成功信息
	t.Logf("Token created and verified successfully")
	t.Logf("Signature: %s", token.Signature)
}

// TestTokenVerifyExpired 测试过期令牌的验证
// 验证系统能够正确拒绝已过期的令牌
func TestTokenVerifyExpired(t *testing.T) {
	// 1. 初始化签名密钥
	if err := InitSigningKey(""); err != nil {
		t.Fatalf("Failed to init signing key: %v", err)
	}

	// 2. 创建已过期的令牌（有效期为 -1 分钟）
	token, err := NewToken("test-tool", "read", "agent-001", "session-123", "task-456", -1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	// 3. 验证令牌应该失败
	verifier := NewVerifier()
	if err := verifier.Verify(token); err == nil {
		t.Fatal("Expected verification to fail for expired token")
	}

	// 4. 输出成功信息
	t.Logf("Expired token correctly rejected: %v", err)
}

// TestTokenVerifyTamperedSignature 测试篡改签名的检测
// 验证系统能够正确识别被篡改的签名
func TestTokenVerifyTamperedSignature(t *testing.T) {
	// 1. 初始化签名密钥
	if err := InitSigningKey(""); err != nil {
		t.Fatalf("Failed to init signing key: %v", err)
	}

	// 2. 创建正常令牌
	token, err := NewToken("test-tool", "read", "agent-001", "session-123", "task-456", 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	// 3. 篡改签名
	token.Signature = "tampered-signature"

	// 4. 验证应该失败
	verifier := NewVerifier()
	if err := verifier.Verify(token); err == nil {
		t.Fatal("Expected verification to fail for tampered signature")
	}

	// 5. 输出成功信息
	t.Logf("Tampered signature correctly rejected: %v", err)
}

// TestTokenNonceReuse 测试 Nonce 重放攻击的防护
// 验证系统能够检测到重复使用的 Nonce
func TestTokenNonceReuse(t *testing.T) {
	// 1. 重置 Nonce 记录（清理测试环境）
	ResetNonces()

	// 2. 初始化签名密钥
	if err := InitSigningKey(""); err != nil {
		t.Fatalf("Failed to init signing key: %v", err)
	}

	// 3. 创建令牌
	token, err := NewToken("test-tool", "read", "agent-001", "session-123", "task-456", 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	// 4. 第一次验证应该成功
	verifier := NewVerifier()
	if err := verifier.Verify(token); err != nil {
		t.Fatalf("First verification failed: %v", err)
	}

	// 5. 使用同一个令牌再次验证（重放攻击）
	verifier2 := NewVerifier()
	if err := verifier2.Verify(token); err == nil {
		t.Fatal("Expected verification to fail for reused nonce")
	}

	// 6. 输出成功信息
	t.Logf("Nonce reuse correctly detected: %v", err)
}
