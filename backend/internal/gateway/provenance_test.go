// backend/internal/gateway/provenance_test.go
package gateway

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"

	"aegisguard/internal/auth"
	"aegisguard/internal/gates"

	"go.uber.org/zap"
)

// 测试样例：extractToolCall 应返回消息数组中最后一条带 tool_calls 的消息（本次待处理的调用），
// 而不是历史上第一次已经执行完的调用（工程审计发现的既有缺陷回归测试）。
func TestExtractToolCallPicksMostRecentCall(t *testing.T) {
	proxy := &AegisProxy{logger: zap.NewNop()}

	body := []byte(`{"messages":[
		{"role":"user","content":"clean up stale records"},
		{"role":"assistant","tool_calls":[{"function":{"name":"search_records","arguments":{}}}]},
		{"role":"tool","content":"{\"records\":[{\"id\":\"rec-42\"}]}"},
		{"role":"assistant","tool_calls":[{"function":{"name":"delete_record","arguments":{"record_id":"rec-42"}}}]}
	]}`)

	toolName, params := proxy.extractToolCall(body)
	if toolName != "delete_record" {
		t.Fatalf("expected the most recent tool call 'delete_record', got %q", toolName)
	}
	if params["record_id"] != "rec-42" {
		t.Fatalf("expected params to carry record_id from the most recent call, got %+v", params)
	}
}

func newProvenanceTestProxy(t *testing.T, enabled bool, mode string) *AegisProxy {
	t.Helper()
	auth.ResetNonces()
	if err := auth.InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}
	return &AegisProxy{
		actionGate:        gates.NewActionGateWithMode(zap.NewNop(), "strict"),
		tokenIssuer:       auth.NewTokenStore(),
		tokenMode:         "strict",
		logger:            zap.NewNop(),
		provenanceEnabled: enabled,
		provenanceMode:    mode,
	}
}

// bodyWithFabricatedRecordID 模拟"侦察到 rec-42，却对未出现过的 rec-99 发起删除"的注入场景：
// delete_record 不在 PolicyEngine 的任何硬编码正则里，避免与语义评分规则相互干扰，
// 让测试单纯验证溯源信号本身（沿用 Phase 2 已验证过的"避免工具名撞正则"经验）。
func bodyWithFabricatedRecordID() []byte {
	return []byte(`{"messages":[
		{"role":"user","content":"clean up stale records"},
		{"role":"assistant","tool_calls":[{"function":{"name":"search_records","arguments":{}}}]},
		{"role":"tool","content":"{\"records\":[{\"id\":\"rec-42\"}]}"},
		{"role":"assistant","tool_calls":[{"function":{"name":"delete_record","arguments":{"record_id":"rec-99"}}}]}
	]}`)
}

func bodyWithTraceableRecordID() []byte {
	return []byte(`{"messages":[
		{"role":"user","content":"clean up stale records"},
		{"role":"assistant","tool_calls":[{"function":{"name":"search_records","arguments":{}}}]},
		{"role":"tool","content":"{\"records\":[{\"id\":\"rec-42\"}]}"},
		{"role":"assistant","tool_calls":[{"function":{"name":"delete_record","arguments":{"record_id":"rec-42"}}}]}
	]}`)
}

// 测试样例：enforce 模式下，无法溯源到任何声明来源工具历史回执的高危参数（疑似注入伪造）应被拒绝
func TestHandleToolCallProvenanceEnforceBlocksFabricatedRecordID(t *testing.T) {
	proxy := newProvenanceTestProxy(t, true, "enforce")
	body := bodyWithFabricatedRecordID()

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), "request_id", "req-1"))
	req = req.WithContext(context.WithValue(req.Context(), "gateway_key", "agk-test-001"))

	result, ok := proxy.handleToolCall(req, body)
	if ok {
		t.Fatalf("expected fabricated record_id to be denied, got decision=%s", result.Decision)
	}
	if result.Decision != gates.Deny {
		t.Fatalf("expected Deny decision, got %s (%s)", result.Decision, result.Reason)
	}
}

// 测试样例：log-only 模式下同样的违规只记录不阻断
func TestHandleToolCallProvenanceLogOnlyDoesNotBlock(t *testing.T) {
	proxy := newProvenanceTestProxy(t, true, "log-only")
	body := bodyWithFabricatedRecordID()

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), "request_id", "req-1"))
	req = req.WithContext(context.WithValue(req.Context(), "gateway_key", "agk-test-001"))

	result, ok := proxy.handleToolCall(req, body)
	if !ok {
		t.Fatalf("log-only mode should not block, got decision=%s reason=%s", result.Decision, result.Reason)
	}
}

// 测试样例：参数值确实可追溯到已声明来源工具的历史回执时，enforce 模式应正常放行
func TestHandleToolCallProvenanceAllowsTraceableValue(t *testing.T) {
	proxy := newProvenanceTestProxy(t, true, "enforce")
	body := bodyWithTraceableRecordID()

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), "request_id", "req-1"))
	req = req.WithContext(context.WithValue(req.Context(), "gateway_key", "agk-test-001"))

	result, ok := proxy.handleToolCall(req, body)
	if !ok {
		t.Fatalf("expected traceable record_id to be allowed, got decision=%s reason=%s", result.Decision, result.Reason)
	}
}

// 测试样例：开关默认关闭时，即使参数无法溯源也不受影响（与开启前行为完全一致）
func TestHandleToolCallProvenanceDisabledByDefault(t *testing.T) {
	proxy := newProvenanceTestProxy(t, false, "enforce") // enabled=false，mode 无关紧要
	body := bodyWithFabricatedRecordID()

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), "request_id", "req-1"))
	req = req.WithContext(context.WithValue(req.Context(), "gateway_key", "agk-test-001"))

	result, ok := proxy.handleToolCall(req, body)
	if !ok {
		t.Fatalf("provenance disabled should not affect existing behavior, got decision=%s reason=%s", result.Decision, result.Reason)
	}
}
