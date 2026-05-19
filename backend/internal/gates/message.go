package gates

import "aegisguard/internal/interfaces"

// MessageGate 消息门控
type MessageGate struct {
	policyEngine *PolicyEngine
}

// NewMessageGate 创建消息门控
func NewMessageGate() *MessageGate {
	return &MessageGate{
		policyEngine: NewPolicyEngine(),
	}
}

// Evaluate 评估消息
func (g *MessageGate) Evaluate(body []byte) interfaces.EvaluateResult {
	text := extractJSONText(body, "content", "role", "name", "tool_choice")
	score, rules := g.policyEngine.Score(text)

	if hasRuleFromList(rules, "memory_poisoning") {
		return makeEvaluateResult(Block, "message attempts to persist or rewrite trusted memory/instructions", score, rules)
	}
	if hasRuleFromList(rules, "illegal_finance") {
		return makeEvaluateResult(Deny, "message contains prohibited financial misconduct intent", score, rules)
	}
	if hasRuleFromList(rules, "prompt_injection") && (hasRuleFromList(rules, "privileged_scope") || hasRuleFromList(rules, "sensitive_access")) {
		return makeEvaluateResult(Block, "message combines prompt-injection markers with privileged or sensitive intent", score, rules)
	}
	if g.policyEngine.ShouldBlock(score) {
		return makeEvaluateResult(Block, "message risk exceeds block threshold", score, rules)
	}
	if g.policyEngine.ShouldHumanReview(score) || hasRuleFromList(rules, "prompt_injection") || hasRuleFromList(rules, "sensitive_access") {
		return makeEvaluateResult(Degrade, "message risk can be handled by degraded execution", score, rules)
	}

	return makeEvaluateResult(Allow, "message passed policy checks", score, rules)
}
