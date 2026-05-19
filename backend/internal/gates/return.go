package gates

import (
	"encoding/json"

	"aegisguard/internal/interfaces"
	"aegisguard/internal/sanitize"
)

// ReturnGate 返回门控
type ReturnGate struct {
	policyEngine *PolicyEngine
}

// NewReturnGate 创建返回门控
func NewReturnGate() *ReturnGate {
	return &ReturnGate{
		policyEngine: NewPolicyEngine(),
	}
}

// Evaluate 评估返回结果
func (g *ReturnGate) Evaluate(body []byte) interfaces.EvaluateResult {
	text := extractJSONText(body, "content", "text", "message", "output", "tool_calls", "function_call")
	score, rules := g.policyEngine.Score(text)

	if hasRuleFromList(rules, "memory_poisoning") {
		return makeEvaluateResult(Block, "return contains memory or instruction contamination", score, rules)
	}
	if hasRuleFromList(rules, "illegal_finance") {
		return makeEvaluateResult(Deny, "return contains prohibited financial misconduct content", score, rules)
	}
	if hasRuleFromList(rules, "sensitive_access") || hasRuleFromList(rules, "prompt_injection") {
		return makeEvaluateResult(Degrade, "return contains sensitive or contaminated content and must be sanitized", score, rules)
	}
	if g.policyEngine.ShouldBlock(score) {
		return makeEvaluateResult(Block, "return risk exceeds block threshold", score, rules)
	}
	if g.policyEngine.ShouldDegrade(score) {
		return makeEvaluateResult(Degrade, "return risk can be handled by sanitized summary", score, rules)
	}

	return makeEvaluateResult(Allow, "return passed policy checks", score, rules)
}

// Filter returns a safe degraded response body.
func (g *ReturnGate) Filter(body []byte) []byte {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		safe, _ := sanitize.Text(string(body))
		return []byte(safe)
	}

	var v any = payload
	sanitize.JSON(&v, "")
	filtered, err := json.Marshal(v)
	if err != nil {
		safe, _ := sanitize.Text(string(body))
		return []byte(safe)
	}
	return filtered
}
