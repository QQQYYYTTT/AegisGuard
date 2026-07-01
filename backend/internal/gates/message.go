package gates

import (
	"os"
	"strings"

	"aegisguard/internal/interfaces"
)

// MessageGate 消息门控
type MessageGate struct {
	policyEngine *PolicyEngine
}

// NewMessageGate 创建消息门控
func NewMessageGate() *MessageGate {
	return NewMessageGateWithRuntime(nil)
}

func NewMessageGateWithRuntime(runtime *PolicyRuntime) *MessageGate {
	return &MessageGate{
		policyEngine: NewPolicyEngineWithRuntime(runtime),
	}
}

// Evaluate 评估消息
func (g *MessageGate) Evaluate(body []byte) interfaces.EvaluateResult {
	text := extractJSONText(body, "content", "role", "name", "tool_choice")
	score, rules, ruleAction := g.policyEngine.ScoreForGate("message", text)

	if hasRuleFromList(rules, "memory_poisoning") {
		if messageBlockMode() == "degrade" {
			return makeEvaluateResult(Degrade, "message attempts to persist or rewrite trusted memory/instructions; degraded by policy", score, rules)
		}
		return makeEvaluateResult(Block, "message attempts to persist or rewrite trusted memory/instructions", score, rules)
	}
	if hasRuleFromList(rules, "illegal_finance") {
		if messageBlockMode() == "degrade" {
			return makeEvaluateResult(Degrade, "message contains prohibited financial misconduct intent; degraded by policy", score, rules)
		}
		return makeEvaluateResult(Deny, "message contains prohibited financial misconduct intent", score, rules)
	}
	if hasRuleFromList(rules, "prompt_injection") && (hasRuleFromList(rules, "privileged_scope") || hasRuleFromList(rules, "sensitive_access")) {
		if messageBlockMode() == "degrade" {
			return makeEvaluateResult(Degrade, "message combines prompt-injection markers with privileged or sensitive intent; degraded by policy", score, rules)
		}
		return makeEvaluateResult(Block, "message combines prompt-injection markers with privileged or sensitive intent", score, rules)
	}
	if ruleAction == Block && g.policyEngine.ShouldBlock(score) {
		if messageBlockMode() == "degrade" {
			return makeEvaluateResult(Degrade, "message risk exceeds block threshold; degraded by policy", score, rules)
		}
		return makeEvaluateResult(Block, "message risk exceeds block threshold", score, rules)
	}
	if ruleAction == Deny && g.policyEngine.ShouldBlock(score) {
		return makeEvaluateResult(Deny, "message risk exceeds deny threshold", score, rules)
	}
	if ruleAction == HumanApproval && g.policyEngine.ShouldHumanReview(score) {
		return makeEvaluateResult(Degrade, "message requires degraded execution before approval", score, rules)
	}
	if g.policyEngine.ShouldBlock(score) {
		if messageBlockMode() == "degrade" {
			return makeEvaluateResult(Degrade, "message risk exceeds block threshold; degraded by policy", score, rules)
		}
		return makeEvaluateResult(Block, "message risk exceeds block threshold", score, rules)
	}
	if g.policyEngine.ShouldHumanReview(score) || hasRuleFromList(rules, "prompt_injection") || hasRuleFromList(rules, "sensitive_access") {
		return makeEvaluateResult(Degrade, "message risk can be handled by degraded execution", score, rules)
	}

	return makeEvaluateResult(Allow, "message passed policy checks", score, rules)
}

func messageBlockMode() string {
	mode := strings.TrimSpace(strings.ToLower(os.Getenv("AEGISGUARD_MESSAGE_BLOCK_MODE")))
	switch mode {
	case "degrade", "soft", "sanitize":
		return "degrade"
	default:
		return "block"
	}
}
