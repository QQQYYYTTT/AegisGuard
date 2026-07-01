// backend/internal/gates/provenance_test.go
package gates

import "testing"

// 测试样例：user_prompt 来源 —— 参数值确实出现在用户消息里时放行
func TestCheckProvenanceUserPromptValueFound(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"please transfer 1000 to alice"},
		{"role":"assistant","tool_calls":[{"function":{"name":"transfer_funds","arguments":{}}}]}
	]}`)
	params := map[string]interface{}{"recipient": "alice", "amount": float64(1000)}

	violations := CheckProvenance("transfer_funds", params, body)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %+v", violations)
	}
}

// 测试样例：user_prompt 来源 —— 参数值系凭空捏造（未出现在任何用户消息里），应判定违规
func TestCheckProvenanceUserPromptValueMissingIsViolation(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"please transfer 1000 to alice"},
		{"role":"assistant","tool_calls":[{"function":{"name":"transfer_funds","arguments":{}}}]}
	]}`)
	// 注入攻击场景：用户只说了转给 alice，但工具调用的收款人被篡改为 attacker
	params := map[string]interface{}{"recipient": "attacker", "amount": float64(1000)}

	violations := CheckProvenance("transfer_funds", params, body)
	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 violation for fabricated recipient, got %+v", violations)
	}
	if violations[0].Param != "recipient" {
		t.Fatalf("expected violation on 'recipient' param, got %q", violations[0].Param)
	}
}

// 测试样例：observation_direct 来源 —— 参数值确实来自声明的合法来源工具的历史回执
func TestCheckProvenanceObservationDirectValueFound(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"clean up stale records"},
		{"role":"assistant","tool_calls":[{"function":{"name":"search_records","arguments":{}}}]},
		{"role":"tool","content":"{\"records\":[{\"id\":\"rec-42\"}]}"},
		{"role":"assistant","tool_calls":[{"function":{"name":"delete_record","arguments":{}}}]}
	]}`)
	params := map[string]interface{}{"record_id": "rec-42"}

	violations := CheckProvenance("delete_record", params, body)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %+v", violations)
	}
}

// 测试样例：observation_direct 来源 —— 参数值从未在任何声明的来源工具的回执里出现过，应判定违规
// 对应"注入指令诱导 Agent 跳过侦察直接删除任意 ID"的典型攻击模式
func TestCheckProvenanceObservationDirectValueMissingIsViolation(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"clean up stale records"},
		{"role":"assistant","tool_calls":[{"function":{"name":"search_records","arguments":{}}}]},
		{"role":"tool","content":"{\"records\":[{\"id\":\"rec-42\"}]}"},
		{"role":"assistant","tool_calls":[{"function":{"name":"delete_record","arguments":{}}}]}
	]}`)
	params := map[string]interface{}{"record_id": "rec-99"}

	violations := CheckProvenance("delete_record", params, body)
	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 violation for untraceable record_id, got %+v", violations)
	}
}

// 测试样例：来源工具未被调用过，参数值自然也不在任何历史回执里，应判定违规
func TestCheckProvenanceObservationDirectNoPriorSourceCallIsViolation(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"delete record rec-42 directly"},
		{"role":"assistant","tool_calls":[{"function":{"name":"delete_record","arguments":{}}}]}
	]}`)
	params := map[string]interface{}{"record_id": "rec-42"}

	violations := CheckProvenance("delete_record", params, body)
	if len(violations) != 1 {
		t.Fatalf("expected violation when source tool was never called, got %+v", violations)
	}
}

// 测试样例：未在 paramSourceRegistry 登记的工具，不做任何溯源检查（避免过度设计/误杀）
func TestCheckProvenanceUnregisteredToolSkipsCheck(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	params := map[string]interface{}{"anything": "goes"}

	violations := CheckProvenance("search_flights", params, body)
	if len(violations) != 0 {
		t.Fatalf("expected no violations for unregistered tool, got %+v", violations)
	}
}

// 测试样例：只声明部分参数策略，未提供的参数不参与校验（例如工具调用没有传该参数）
func TestCheckProvenanceMissingParamIsSkipped(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"transfer money"}]}`)
	params := map[string]interface{}{"amount": float64(1000)} // 缺少 recipient

	violations := CheckProvenance("transfer_funds", params, body)
	for _, v := range violations {
		if v.Param == "recipient" {
			t.Fatalf("did not expect a violation for a param that was never supplied: %+v", v)
		}
	}
}

// 测试样例：畸形请求体不 panic，直接跳过校验（保守放行，由其它 Gate 兜底）
func TestCheckProvenanceMalformedBodyDoesNotPanic(t *testing.T) {
	violations := CheckProvenance("transfer_funds", map[string]interface{}{"recipient": "alice"}, []byte("not json"))
	if violations != nil {
		t.Fatalf("expected nil violations for malformed body, got %+v", violations)
	}
}

func TestNormalizeProvenanceMode(t *testing.T) {
	cases := map[string]string{
		"enforce":  "enforce",
		"Enforce":  "enforce",
		" ENFORCE ": "enforce",
		"log-only": "log-only",
		"":         "log-only",
		"garbage":  "log-only",
	}
	for input, want := range cases {
		if got := NormalizeProvenanceMode(input); got != want {
			t.Fatalf("NormalizeProvenanceMode(%q) = %q, want %q", input, got, want)
		}
	}
}
