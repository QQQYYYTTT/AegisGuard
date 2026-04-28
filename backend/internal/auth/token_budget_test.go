// backend/internal/auth/token_test.go
package auth

import (
	"testing"
	"time"
)

func TestNewTokenWithBudget(t *testing.T) {
	// 初始化签名密钥
	if err := InitSigningKey(""); err != nil {
		t.Fatalf("Failed to init signing key: %v", err)
	}

	// 测试 1：创建带预算的令牌
	token, err := NewToken("shell_exec", "workspace.write", "agent-001", "session-123", "task-456", 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	// 验证字段
	if token.ToolName != "shell_exec" {
		t.Errorf("Expected tool_name 'shell_exec', got '%s'", token.ToolName)
	}
	if token.MaxCalls != 10 {
		t.Errorf("Expected max_calls 10, got %d", token.MaxCalls)
	}
	if token.CallCount != 0 {
		t.Errorf("Expected initial call_count 0, got %d", token.CallCount)
	}
	if token.Signature == "" {
		t.Error("Expected signature to be set")
	}

	// 测试 2：创建无限制预算的令牌（maxCalls=0）
	tokenUnlimited, err := NewToken("read_file", "read", "agent-001", "session-123", "task-456", 5*time.Minute, 0)
	if err != nil {
		t.Fatalf("Failed to create unlimited token: %v", err)
	}
	if tokenUnlimited.MaxCalls != 0 {
		t.Errorf("Expected max_calls 0 (unlimited), got %d", tokenUnlimited.MaxCalls)
	}
}

func TestTokenSignatureIncludesBudget(t *testing.T) {
	if err := InitSigningKey(""); err != nil {
		t.Fatalf("Failed to init signing key: %v", err)
	}

	// 创建两个不同预算的令牌
	token1, _ := NewToken("tool", "scope", "agent", "session", "task", 5*time.Minute, 5)
	token2, _ := NewToken("tool", "scope", "agent", "session", "task", 5*time.Minute, 10)

	// 签名应该不同（因为预算不同）
	if token1.Signature == token2.Signature {
		t.Error("Signatures should differ when max_calls differs")
	}

	// 验证签名
	verifier := NewVerifier()
	if err := verifier.Verify(token1); err != nil {
		t.Errorf("Token1 verification failed: %v", err)
	}
	if err := verifier.Verify(token2); err != nil {
		t.Errorf("Token2 verification failed: %v", err)
	}
}

func TestCallBudgetEnforcement(t *testing.T) {
	if err := InitSigningKey(""); err != nil {
		t.Fatalf("Failed to init signing key: %v", err)
	}

	verifier := NewVerifier()

	// 测试场景：一个预算为 3 次的令牌，每次调用后递增计数
	// 实际应用中，每次请求会携带更新后的 CallCount

	// 第 1 次调用：创建令牌，CallCount=0
	token1, _ := NewToken("test_tool", "read", "agent", "session", "task", 5*time.Minute, 3)
	if err := verifier.Verify(token1); err != nil {
		t.Errorf("First call should succeed: %v", err)
	}
	// 验证后，上层应用将 CallCount 递增为 1

	// 第 2 次调用：新令牌（同一会话），CallCount=1
	token2, _ := NewToken("test_tool", "read", "agent", "session", "task", 5*time.Minute, 3)
	token2.CallCount = 1
	if err := verifier.Verify(token2); err != nil {
		t.Errorf("Second call should succeed when CallCount=1: %v", err)
	}
	// 验证后，上层应用将 CallCount 递增为 2

	// 第 3 次调用：新令牌，CallCount=2
	token3, _ := NewToken("test_tool", "read", "agent", "session", "task", 5*time.Minute, 3)
	token3.CallCount = 2
	if err := verifier.Verify(token3); err != nil {
		t.Errorf("Third call should succeed when CallCount=2: %v", err)
	}
	// 验证后，上层应用将 CallCount 递增为 3

	// 第 4 次调用：新令牌，CallCount=3（已达上限）
	token4, _ := NewToken("test_tool", "read", "agent", "session", "task", 5*time.Minute, 3)
	token4.CallCount = 3
	if err := verifier.Verify(token4); err == nil {
		t.Error("Fourth call should fail (budget exceeded)")
	} else {
		t.Logf("Correctly rejected call: %v", err)
	}
}

func TestUnlimitedBudget(t *testing.T) {
	if err := InitSigningKey(""); err != nil {
		t.Fatalf("Failed to init signing key: %v", err)
	}

	// 创建无限制预算的令牌
	token, _ := NewToken("test_tool", "read", "agent", "session", "task", 5*time.Minute, 0)
	verifier := NewVerifier()

	// 多次调用都应该成功
	for i := 0; i < 100; i++ {
		// 需要创建新令牌，因为 Nonce 只能使用一次
		token, _ = NewToken("test_tool", "read", "agent", "session", "task", 5*time.Minute, 0)
		if err := verifier.Verify(token); err != nil {
			t.Errorf("Call %d should succeed with unlimited budget: %v", i+1, err)
		}
	}
}
