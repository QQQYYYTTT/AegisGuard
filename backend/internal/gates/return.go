package gates

import (
	"encoding/json"
	"regexp"
	"strings"
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
func (g *ReturnGate) Evaluate(body []byte) (Decision, string) {
	text := extractJSONText(body, "content", "text", "message", "output", "tool_calls", "function_call")
	score, rules := g.policyEngine.Score(text)

	if hasRuleFromList(rules, "memory_poisoning") {
		return Block, makeReasonFromScore("return contains memory or instruction contamination", score, rules)
	}
	if hasRuleFromList(rules, "illegal_finance") {
		return Deny, makeReasonFromScore("return contains prohibited financial misconduct content", score, rules)
	}
	if hasRuleFromList(rules, "sensitive_access") || hasRuleFromList(rules, "prompt_injection") {
		return Degrade, makeReasonFromScore("return contains sensitive or contaminated content and must be sanitized", score, rules)
	}
	if g.policyEngine.ShouldBlock(score) {
		return Block, makeReasonFromScore("return risk exceeds block threshold", score, rules)
	}
	if g.policyEngine.ShouldDegrade(score) {
		return Degrade, makeReasonFromScore("return risk can be handled by sanitized summary", score, rules)
	}

	return Allow, makeReasonFromScore("return passed policy checks", score, rules)
}

// Filter returns a safe degraded response body.
func (g *ReturnGate) Filter(body []byte) []byte {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return []byte(sanitizeText(string(body)))
	}

	sanitizeJSONStrings(payload)
	filtered, err := json.Marshal(payload)
	if err != nil {
		return []byte(sanitizeText(string(body)))
	}
	return filtered
}

func sanitizeJSONStrings(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if text, ok := child.(string); ok {
				if isSensitiveField(key) {
					typed[key] = "[REDACTED]"
					continue
				}
				typed[key] = sanitizeText(text)
				continue
			}
			sanitizeJSONStrings(child)
		}
	case []any:
		for _, child := range typed {
			sanitizeJSONStrings(child)
		}
	}
}

func isSensitiveField(key string) bool {
	normalized := strings.ToLower(key)
	for _, marker := range []string{"password", "passwd", "api_key", "apikey", "secret", "token", "private_key"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func sanitizeText(text string) string {
	if text == "" {
		return text
	}

	sanitized := text
	valuePatterns := []string{
		`(?i)\bsk-[A-Za-z0-9_-]{8,}\b`,
		`(?i)\bBearer\s+[A-Za-z0-9._-]{12,}\b`,
		`(?i)\b(AKIA[0-9A-Z]{12,})\b`,
	}
	for _, pattern := range valuePatterns {
		re := regexp.MustCompile(pattern)
		sanitized = re.ReplaceAllString(sanitized, "[REDACTED]")
	}

	assignmentPatterns := []string{
		`(?i)\b(password|passwd|api[_-]?key|secret|token|private[_-]?key)\s*[:=]\s*("[^"]*"|'[^']*'|[^\s,;}]+)`,
	}
	for _, pattern := range assignmentPatterns {
		re := regexp.MustCompile(pattern)
		sanitized = re.ReplaceAllString(sanitized, "$1=[REDACTED]")
	}

	instructionPatterns := []string{
		`(?i)\bignore\b.{0,80}\b(previous|prior|system|developer|instruction|policy|rule)s?\b`,
		`(?i)\b(system prompt|developer message|hidden instruction|jailbreak|prompt injection)\b`,
		`(?i)\b(add this to memory|update your memory|from now on)\b`,
	}
	for _, pattern := range instructionPatterns {
		re := regexp.MustCompile(pattern)
		sanitized = re.ReplaceAllString(sanitized, "[FILTERED_POLICY_TEXT]")
	}

	return strings.TrimSpace(sanitized)
}
