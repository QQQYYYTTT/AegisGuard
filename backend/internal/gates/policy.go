package gates

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"aegisguard/internal/interfaces"
)

var (
	defaultPolicyOnce sync.Once
	defaultPolicy     *PolicyRuntime
	defaultPolicyErr  error
)

var toolRuleRegistry = map[string][]string{
	"read":   {"prompt_injection", "memory_poisoning"},
	"search": {"prompt_injection", "memory_poisoning"},
	"get":    {"prompt_injection", "memory_poisoning"},
	"list":   {"prompt_injection", "memory_poisoning"},
	"write":  {"prompt_injection", "memory_poisoning", "sensitive_access", "high_impact_action"},
	"update": {"prompt_injection", "memory_poisoning", "sensitive_access", "high_impact_action"},
	"create": {"prompt_injection", "memory_poisoning", "sensitive_access", "high_impact_action"},
	"delete": {"prompt_injection", "memory_poisoning", "sensitive_access", "high_impact_action", "illegal_finance"},
	"exec":     {"prompt_injection", "privileged_scope", "sensitive_access", "high_impact_action", "memory_poisoning", "illegal_finance"},
	"send":     {"prompt_injection", "privileged_scope", "sensitive_access", "high_impact_action", "memory_poisoning", "illegal_finance"},
	"transfer": {"prompt_injection", "privileged_scope", "sensitive_access", "high_impact_action", "memory_poisoning", "illegal_finance"},
}

var defaultRuleIDs = []string{"prompt_injection", "privileged_scope", "sensitive_access", "high_impact_action", "memory_poisoning", "illegal_finance"}

var toolRiskTierRegistry = map[string]int{
	"read": 0, "search": 0, "get": 0, "list": 0,
	"write": 1, "update": 1, "create": 1, "delete": 1,
	"exec": 2, "send": 2, "transfer": 2,
}

const highRiskTier = 2

type PolicyEngine struct {
	runtime *PolicyRuntime
}

func NewPolicyEngine() *PolicyEngine {
	defaultPolicyOnce.Do(func() {
		defaultPolicy, defaultPolicyErr = NewPolicyRuntime("")
	})
	if defaultPolicyErr != nil {
		panic(defaultPolicyErr)
	}
	return &PolicyEngine{runtime: defaultPolicy}
}

func NewPolicyEngineWithRuntime(runtime *PolicyRuntime) *PolicyEngine {
	if runtime == nil {
		return NewPolicyEngine()
	}
	return &PolicyEngine{runtime: runtime}
}

func resolveToolRiskTier(toolName string) int {
	if tier, ok := toolRiskTierRegistry[toolName]; ok {
		return tier
	}
	if tier, ok := toolRiskTierRegistry[normalizeToolName(toolName)]; ok {
		return tier
	}
	return highRiskTier
}

func normalizeToolName(raw string) string {
	clean := strings.ToLower(strings.TrimSpace(raw))
	if idx := strings.Index(clean, "_"); idx > 0 {
		clean = clean[:idx]
	}
	return clean
}

func resolveRuleIDs(toolName string) map[string]struct{} {
	ids, ok := toolRuleRegistry[toolName]
	if !ok {
		ids, ok = toolRuleRegistry[normalizeToolName(toolName)]
	}
	if !ok {
		ids = defaultRuleIDs
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func (pe *PolicyEngine) Score(text string) (int, []string) {
	if pe == nil || pe.runtime == nil || strings.TrimSpace(text) == "" {
		return 0, nil
	}
	score, rules, _ := evaluateRules(text, pe.runtime.rules, pe.runtime.config.GlobalThreshold)
	return score, rules
}

func (pe *PolicyEngine) ScoreForTool(toolName, text string) (int, []string) {
	if pe == nil || pe.runtime == nil || strings.TrimSpace(text) == "" {
		return 0, nil
	}
	activeIDs := resolveRuleIDs(toolName)
	routed := make([]runtimeRule, 0, len(pe.runtime.rules))
	for _, rule := range pe.runtime.rules {
		if _, ok := activeIDs[rule.ID]; ok || hasAliasForRule(rule.ID, activeIDs) {
			routed = append(routed, rule)
		}
	}
	if len(routed) == 0 {
		routed = pe.runtime.rules
	}
	score, rules, _ := evaluateRules(text, routed, pe.runtime.config.GlobalThreshold)
	return score, rules
}

func (pe *PolicyEngine) ScoreForGate(gateType, text string) (int, []string, Decision) {
	if pe == nil || pe.runtime == nil || strings.TrimSpace(text) == "" {
		return 0, nil, Allow
	}
	score, rules, topRule := pe.runtime.Evaluate(gateType, text)
	if topRule == nil {
		return score, rules, Allow
	}
	return score, rules, topRule.Action
}

func (pe *PolicyEngine) ScoreForGateAndTool(gateType, toolName, text string, dynamic bool) (int, []string, Decision) {
	if pe == nil || pe.runtime == nil || strings.TrimSpace(text) == "" {
		return 0, nil, Allow
	}
	score, rules, topRule := pe.runtime.EvaluateForTool(gateType, toolName, text, dynamic)
	if topRule == nil {
		return score, rules, Allow
	}
	return score, rules, topRule.Action
}

func (pe *PolicyEngine) ShouldBlock(score int) bool {
	if pe == nil || pe.runtime == nil {
		return score >= 85
	}
	global, _, _ := pe.runtime.Thresholds()
	return score >= global
}

func (pe *PolicyEngine) ShouldDegrade(score int) bool {
	if pe == nil || pe.runtime == nil {
		return score >= 35
	}
	_, _, degrade := pe.runtime.Thresholds()
	return score >= degrade
}

func (pe *PolicyEngine) ShouldHumanReview(score int) bool {
	if pe == nil || pe.runtime == nil {
		return score >= 65
	}
	_, humanReview, _ := pe.runtime.Thresholds()
	return score >= humanReview
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
		if rule == id || strings.HasPrefix(rule, id+"_") {
			return true
		}
	}
	return false
}

func hasAliasForRule(ruleID string, activeIDs map[string]struct{}) bool {
	for activeID := range activeIDs {
		if ruleID == activeID || strings.HasPrefix(ruleID, activeID+"_") {
			return true
		}
	}
	return false
}

func makeReasonFromScore(message string, score int, rules []string) string {
	if len(rules) == 0 {
		return fmt.Sprintf("%s; risk_score=%d; risk_level=%s; matched_rules=none", message, score, RiskLevel(score))
	}
	ordered := append([]string(nil), rules...)
	sort.Strings(ordered)
	return fmt.Sprintf("%s; risk_score=%d; risk_level=%s; matched_rules=%s", message, score, RiskLevel(score), strings.Join(ordered, ","))
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
