package gates

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"aegisguard/internal/interfaces"
)

type policyRule struct {
	ID       string
	Weight   int
	Patterns []*regexp.Regexp
}

type PolicyEngine struct {
	rules []policyRule
}

func NewPolicyEngine() *PolicyEngine {
	rules := []policyRule{
		rule("prompt_injection", 35,
			`(?i)\b(ignore|forget|bypass|override)\b.{0,80}\b(previous|prior|system|developer|instruction|policy|rule)s?\b`,
			`(?i)\b(system prompt|developer message|hidden instruction|jailbreak|prompt injection)\b`,
			`(?i)\b(do not tell|do not reveal|secretly|silently)\b.{0,80}\b(user|operator|admin)\b`,
		),
		rule("privileged_scope", 30,
			`(?i)\b(admin|root|sudo|privileged|permission|credential|authorization|access token)\b`,
			`(?i)\b(shell|cmd|powershell|bash|exec|spawn|subprocess)\b`,
			`(?i)\b(delete|drop|wipe|exfiltrate|upload|download|modify|overwrite)\b.{0,80}\b(file|database|record|config|secret)\b`,
		),
		rule("sensitive_access", 35,
			`(?i)\b(api[_-]?key|api\s+key|password|passwd|secret|private key|token|session|cookie|credential)\b`,
			`(?i)\b(sk-[A-Za-z0-9_-]{8,}|AKIA[0-9A-Z]{12,}|Bearer\s+[A-Za-z0-9._-]{12,})\b`,
			`(?i)\b(card number|credit card|ssn|social security|id card|bank account)\b`,
		),
		rule("high_impact_action", 32,
			`(?i)\b(transfer|wire|withdraw|purchase|refund|pay|trade|sell|buy|delete|disable|revoke)\b.{0,80}\b(production|prod|customer|account|database|billing|payment|funds?|money|record|file)\b`,
			`(?i)\b(transfer_funds|wire_transfer|delete_file|shell_exec)\b`,
			`(?i)\b(rm\s+-rf|format\s+[A-Z]:|drop\s+table|truncate\s+table)\b`,
		),
		rule("memory_poisoning", 45,
			`(?i)\b(save|store|remember|persist|write)\b.{0,80}\b(instruction|rule|memory|policy|system prompt)\b`,
			`(?i)\b(remember|save|store|persist)\b.{0,80}\b(command|response|answer)\b.{0,80}\b(forever|always|from now on)\b`,
			`(?i)\b(add this to memory|update your memory|from now on)\b`,
		),
		rule("illegal_finance", 70,
			`(?i)\b(money laundering|insider trading|fake invoice|tax evasion|evade sanctions)\b`,
			`(?i)\b(fraudulent|stolen card|bypass kyc|launder)\b`,
		),
	}
	return &PolicyEngine{rules: rules}
}

func rule(id string, weight int, patterns ...string) policyRule {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		compiled = append(compiled, regexp.MustCompile(pattern))
	}
	return policyRule{ID: id, Weight: weight, Patterns: compiled}
}

func (pe *PolicyEngine) Score(text string) (int, []string) {
	if pe == nil || text == "" {
		return 0, nil
	}

	score := 0
	matched := make(map[string]struct{})
	for _, rule := range pe.rules {
		for _, pattern := range rule.Patterns {
			if pattern.MatchString(text) {
				score += rule.Weight
				matched[rule.ID] = struct{}{}
				break
			}
		}
	}
	if score > 100 {
		score = 100
	}

	rules := make([]string, 0, len(matched))
	for id := range matched {
		rules = append(rules, id)
	}
	sort.Strings(rules)
	return score, rules
}

func (pe *PolicyEngine) ShouldBlock(score int) bool {
	return score >= 85
}

func (pe *PolicyEngine) ShouldDegrade(score int) bool {
	return score >= 35
}

func (pe *PolicyEngine) ShouldHumanReview(score int) bool {
	return score >= 65
}

func RiskLevel(score int) string {
	switch {
	case score >= 85:
		return "critical"
	case score >= 65:
		return "high"
	case score >= 45:
		return "medium"
	case score > 0:
		return "low"
	default:
		return "none"
	}
}

func hasRuleFromList(rules []string, id string) bool {
	for _, rule := range rules {
		if rule == id {
			return true
		}
	}
	return false
}

func makeReasonFromScore(message string, score int, rules []string) string {
	if len(rules) == 0 {
		return fmt.Sprintf("%s; risk_score=%d; risk_level=%s; matched_rules=none", message, score, RiskLevel(score))
	}
	return fmt.Sprintf("%s; risk_score=%d; risk_level=%s; matched_rules=%s", message, score, RiskLevel(score), strings.Join(rules, ","))
}

func makeEvaluateResult(decision Decision, message string, score int, rules []string) interfaces.EvaluateResult {
	return interfaces.EvaluateResult{
		Decision:     decision,
		Reason:       makeReasonFromScore(message, score, rules),
		RiskScore:    score,
		RiskLevel:    RiskLevel(score),
		MatchedRules: rules,
	}
}

func extractJSONText(body []byte, keys ...string) string {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return string(body)
	}

	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
	}

	var parts []string
	collectText(payload, allowed, &parts)
	return strings.Join(parts, "\n")
}

func collectText(value any, allowed map[string]struct{}, parts *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if len(allowed) == 0 {
				collectText(child, allowed, parts)
				continue
			}
			if _, ok := allowed[key]; ok {
				appendText(child, parts)
				continue
			}
			collectText(child, allowed, parts)
		}
	case []any:
		for _, child := range typed {
			collectText(child, allowed, parts)
		}
	}
}

func appendText(value any, parts *[]string) {
	switch typed := value.(type) {
	case string:
		*parts = append(*parts, typed)
	case map[string]any, []any:
		collectText(typed, nil, parts)
	default:
		if typed != nil {
			*parts = append(*parts, fmt.Sprint(typed))
		}
	}
}
