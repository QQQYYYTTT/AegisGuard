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

// toolRuleRegistry 按工具名（或工具类别前缀）映射到应激活的规则 ID 子集。
// Phase 1（借鉴 DRIFT 动态校验思想）：不同风险面的工具只匹配相关规则子集，
// 免除无关规则的正则扫描开销，而非对每次调用做全量匹配。
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

// defaultRuleIDs 是未匹配任何映射表条目的工具应回退到的全量规则集合，
// 与当前 Score() 的行为保持一致。
var defaultRuleIDs = []string{"prompt_injection", "privileged_scope", "sensitive_access", "high_impact_action", "memory_poisoning", "illegal_finance"}

// toolRiskTierRegistry 复用 Phase 1 已划定的工具类别边界，将工具映射到风险层级：
//
//	0 = 低危（只读/侦察类，如 read/search/get/list）
//	1 = 中危（写入类，如 write/update/create/delete）
//	2 = 高危（破坏性/不可逆类，如 exec/send/transfer）
//
// Phase 2（TDG 拓扑校验）用这份分层实现"高危工具需要前置低/中危调用"的顺序约束——
// 呼应 PLAN.md 中"Phase 2 依赖 Phase 1 规则路由"的设计，也是在不引入 LLM 规划的前提下，
// 对 IPIGuard"调用顺序应符合任务依赖关系"这一核心思想的工程近似（详见 PLAN.md Phase 2 实现说明）。
var toolRiskTierRegistry = map[string]int{
	"read": 0, "search": 0, "get": 0, "list": 0,
	"write": 1, "update": 1, "create": 1, "delete": 1,
	"exec": 2, "send": 2, "transfer": 2,
}

// highRiskTier 是需要前置低/中危调用才能放行的风险层级阈值。
const highRiskTier = 2

// resolveToolRiskTier 解析某个工具名的风险层级：先精确匹配原始工具名，
// 再匹配标准化后的前缀，均未命中时保守地按最高风险层级处理（宁可误拦不可放过）。
func resolveToolRiskTier(toolName string) int {
	if tier, ok := toolRiskTierRegistry[toolName]; ok {
		return tier
	}
	if tier, ok := toolRiskTierRegistry[normalizeToolName(toolName)]; ok {
		return tier
	}
	return highRiskTier
}

// normalizeToolName 将工具名标准化为映射表可匹配的前缀形式。
// 例: "search_flights" -> "search"，"Search_Flights " -> "search"，"get_email_v2" -> "get"
func normalizeToolName(raw string) string {
	clean := strings.ToLower(strings.TrimSpace(raw))
	if idx := strings.Index(clean, "_"); idx > 0 {
		clean = clean[:idx]
	}
	return clean
}

// resolveRuleIDs 解析某个工具名应激活的规则 ID 集合：
// 先精确匹配原始工具名，再匹配标准化后的前缀，均未命中时 fallback 到全量规则。
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

// ScoreForTool 是 Score 的动态规则路由版本（Phase 1）：按工具名先路由到相关规则子集，
// 再执行匹配。未匹配任何映射表条目的工具名 fallback 到全量规则，结果与 Score 一致。
func (pe *PolicyEngine) ScoreForTool(toolName, text string) (int, []string) {
	if pe == nil || text == "" {
		return 0, nil
	}

	activeIDs := resolveRuleIDs(toolName)
	score := 0
	matched := make(map[string]struct{})
	for _, rule := range pe.rules {
		if _, active := activeIDs[rule.ID]; !active {
			continue
		}
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
