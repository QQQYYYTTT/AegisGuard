// backend/internal/gateway/trace_test.go
package gateway

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"aegisguard/internal/auth"
	"aegisguard/internal/gates"

	"go.uber.org/zap"
)

// 测试样例：computeTraceID - 同一对话的多轮请求应产生稳定的 Trace ID
func TestComputeTraceIDStableAcrossConversationTurns(t *testing.T) {
	turn1 := []byte(`{"messages":[{"role":"user","content":"help me book a flight"}]}`)
	turn2 := []byte(`{"messages":[
		{"role":"user","content":"help me book a flight"},
		{"role":"assistant","tool_calls":[{"function":{"name":"search_flights","arguments":"{}"}}]},
		{"role":"tool","content":"[]"}
	]}`)

	id1 := computeTraceID(turn1)
	id2 := computeTraceID(turn2)

	if id1 == "" || id2 == "" {
		t.Fatalf("expected non-empty trace ids, got %q and %q", id1, id2)
	}
	if id1 != id2 {
		t.Fatalf("trace id should stay stable across turns of the same conversation: %q != %q", id1, id2)
	}
}

// 测试样例：computeTraceID - 不同对话不应产生相同 Trace ID
func TestComputeTraceIDDiffersAcrossConversations(t *testing.T) {
	a := computeTraceID([]byte(`{"messages":[{"role":"user","content":"book a flight"}]}`))
	b := computeTraceID([]byte(`{"messages":[{"role":"user","content":"cancel my order"}]}`))

	if a == b {
		t.Fatalf("different conversations should not collide: %q", a)
	}
}

// 测试样例：computeTraceID - 异常/空请求体应返回空字符串而非 panic
func TestComputeTraceIDEmptyForMalformedBody(t *testing.T) {
	if id := computeTraceID([]byte("not json")); id != "" {
		t.Fatalf("expected empty trace id for malformed body, got %q", id)
	}
	if id := computeTraceID([]byte(`{"messages":[]}`)); id != "" {
		t.Fatalf("expected empty trace id for empty messages, got %q", id)
	}
}

// 测试样例：handleToolCall - 注入 Trace ID Header，且 TDG enforce 模式阻断同一对话内的
// 连续重复工具调用（跨越两次独立的 HTTP 请求，模拟真实的多轮 Agent 交互）
func TestHandleToolCallInjectsTraceIDAndEnforcesTDG(t *testing.T) {
	auth.ResetNonces()
	if err := auth.InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}

	actionGate := gates.NewActionGateWithMode(zap.NewNop(), "strict")
	actionGate.SetTDG(gates.TDGSettings{Enabled: true, Mode: "enforce", MaxNodes: 10, MaxRepeat: 1, TTL: time.Minute})

	proxy := &AegisProxy{
		actionGate:  actionGate,
		tokenIssuer: auth.NewTokenStore(),
		tokenMode:   "strict",
		logger:      zap.NewNop(),
	}

	body := []byte(`{
		"messages": [
			{"role": "user", "content": "read my file"},
			{"tool_calls": [{"function": {"name": "read_file", "arguments": {"path": "D:/workspace/demo.txt"}}}]}
		]
	}`)

	req1 := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req1 = req1.WithContext(context.WithValue(req1.Context(), "request_id", "req-1"))
	req1 = req1.WithContext(context.WithValue(req1.Context(), "gateway_key", "agk-test-001"))

	result1, ok1 := proxy.handleToolCall(req1, body)
	if !ok1 {
		t.Fatalf("first call should be allowed, got decision=%s reason=%s", result1.Decision, result1.Reason)
	}
	traceID := req1.Header.Get("X-Aegis-Trace-ID")
	if traceID == "" {
		t.Fatalf("expected trace id header to be injected")
	}

	// 模拟同一对话的第二轮请求：新的 HTTP request_id，但重发了相同的首条用户消息
	req2 := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req2 = req2.WithContext(context.WithValue(req2.Context(), "request_id", "req-2"))
	req2 = req2.WithContext(context.WithValue(req2.Context(), "gateway_key", "agk-test-001"))

	result2, ok2 := proxy.handleToolCall(req2, body)
	if ok2 {
		t.Fatalf("second identical consecutive call should be denied by tdg enforce mode, got decision=%s", result2.Decision)
	}
	if result2.Decision != gates.Deny {
		t.Fatalf("expected Deny decision from tdg enforcement, got %s (%s)", result2.Decision, result2.Reason)
	}
	if got := req2.Header.Get("X-Aegis-Trace-ID"); got != traceID {
		t.Fatalf("expected same trace id across conversation turns, got %q vs %q", got, traceID)
	}
}
