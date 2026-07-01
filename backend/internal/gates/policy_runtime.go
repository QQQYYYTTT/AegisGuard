package gates

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type PolicyRuleConfig struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	GateType      string `json:"gate_type"`
	Condition     string `json:"condition"`
	Action        string `json:"action"`
	Priority      int    `json:"priority"`
	Enabled       bool   `json:"enabled"`
	RiskThreshold int    `json:"risk_threshold"`
}

type RiskWeightsConfig struct {
	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
	Gamma float64 `json:"gamma"`
}

type PolicyConfig struct {
	RiskWeights     RiskWeightsConfig  `json:"risk_weights"`
	GlobalThreshold int                `json:"global_threshold"`
	Rules           []PolicyRuleConfig `json:"rules"`
}

type runtimeRule struct {
	ID            string
	GateType      string
	Action        Decision
	RiskThreshold int
	Priority      int
	Patterns      []*regexp.Regexp
}

type PolicyRuntime struct {
	mu         sync.RWMutex
	path       string
	config     PolicyConfig
	rules      []runtimeRule
	nextRuleID int
}

func NewPolicyRuntime(path string) (*PolicyRuntime, error) {
	pr := &PolicyRuntime{
		path:   path,
		config: defaultPolicyConfig(),
	}

	if path != "" {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			var cfg PolicyConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, err
			}
			pr.config = normalizePolicyConfig(cfg)
		} else if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	pr.rebuildLocked()
	return pr, nil
}

func (pr *PolicyRuntime) Snapshot() PolicyConfig {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return clonePolicyConfig(pr.config)
}

func (pr *PolicyRuntime) Rules() []PolicyRuleConfig {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return clonePolicyRules(pr.config.Rules)
}

func (pr *PolicyRuntime) UpdateRule(rule PolicyRuleConfig) (PolicyRuleConfig, bool, error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	found := false
	for i := range pr.config.Rules {
		if pr.config.Rules[i].ID == rule.ID {
			pr.config.Rules[i] = normalizePolicyRule(rule)
			found = true
			break
		}
	}
	if !found {
		return PolicyRuleConfig{}, false, nil
	}
	pr.rebuildLocked()
	if err := pr.persistLocked(); err != nil {
		return PolicyRuleConfig{}, true, err
	}
	return rule, true, nil
}

func (pr *PolicyRuntime) CreateRule(rule PolicyRuleConfig) (PolicyRuleConfig, error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.nextRuleID++
	rule = normalizePolicyRule(rule)
	rule.ID = "rule-custom-" + strconv.Itoa(pr.nextRuleID)
	pr.config.Rules = append(pr.config.Rules, rule)
	pr.rebuildLocked()
	if err := pr.persistLocked(); err != nil {
		return PolicyRuleConfig{}, err
	}
	return rule, nil
}

func (pr *PolicyRuntime) DeleteRule(ruleID string) bool {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	for i, rule := range pr.config.Rules {
		if rule.ID == ruleID {
			pr.config.Rules = append(pr.config.Rules[:i], pr.config.Rules[i+1:]...)
			pr.rebuildLocked()
			_ = pr.persistLocked()
			return true
		}
	}
	return false
}

func (pr *PolicyRuntime) Reorder(ruleIDs []string) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	ruleMap := make(map[string]PolicyRuleConfig, len(pr.config.Rules))
	for _, rule := range pr.config.Rules {
		ruleMap[rule.ID] = rule
	}

	reordered := make([]PolicyRuleConfig, 0, len(pr.config.Rules))
	for i, id := range ruleIDs {
		rule, ok := ruleMap[id]
		if !ok {
			continue
		}
		rule.Priority = i + 1
		reordered = append(reordered, rule)
		delete(ruleMap, id)
	}

	rest := make([]PolicyRuleConfig, 0, len(ruleMap))
	for _, rule := range ruleMap {
		rest = append(rest, rule)
	}
	sort.Slice(rest, func(i, j int) bool {
		return rest[i].Priority < rest[j].Priority
	})
	for _, rule := range rest {
		rule.Priority = len(reordered) + 1
		reordered = append(reordered, rule)
	}

	pr.config.Rules = reordered
	pr.rebuildLocked()
	return pr.persistLocked()
}

func (pr *PolicyRuntime) UpdateConfig(weights *RiskWeightsConfig, threshold *int) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if weights != nil {
		pr.config.RiskWeights = *weights
	}
	if threshold != nil {
		pr.config.GlobalThreshold = *threshold
	}
	pr.rebuildLocked()
	return pr.persistLocked()
}

func (pr *PolicyRuntime) Evaluate(gateType, text string) (int, []string, *runtimeRule) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.evaluateLocked(gateType, text)
}

func (pr *PolicyRuntime) EvaluateForTool(gateType, toolName, text string, dynamic bool) (int, []string, *runtimeRule) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	filtered := pr.filterRulesLocked(gateType)
	if gateType == "action" {
		filtered = append([]runtimeRule(nil), pr.rules...)
	}
	if !dynamic {
		return evaluateRules(text, filtered, pr.config.GlobalThreshold)
	}

	activeIDs := resolveRuleIDs(toolName)
	routed := make([]runtimeRule, 0, len(filtered))
	for _, rule := range filtered {
		if _, ok := activeIDs[rule.ID]; ok {
			routed = append(routed, rule)
		}
	}
	if len(routed) == 0 {
		routed = filtered
	}
	return evaluateRules(text, routed, pr.config.GlobalThreshold)
}

func (pr *PolicyRuntime) Thresholds() (global int, humanReview int, degrade int) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.config.GlobalThreshold, max(65, pr.config.GlobalThreshold-20), 35
}

func (pr *PolicyRuntime) evaluateLocked(gateType, text string) (int, []string, *runtimeRule) {
	return evaluateRules(text, pr.filterRulesLocked(gateType), pr.config.GlobalThreshold)
}

func (pr *PolicyRuntime) filterRulesLocked(gateType string) []runtimeRule {
	filtered := make([]runtimeRule, 0, len(pr.rules))
	for _, rule := range pr.rules {
		if rule.GateType == gateType {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

func evaluateRules(text string, rules []runtimeRule, globalThreshold int) (int, []string, *runtimeRule) {
	if strings.TrimSpace(text) == "" {
		return 0, nil, nil
	}

	score := 0
	matched := make(map[string]struct{})
	countedFamilies := make(map[string]struct{})
	var topRule *runtimeRule
	for i := range rules {
		rule := rules[i]
		if len(rule.Patterns) == 0 {
			continue
		}
		for _, pattern := range rule.Patterns {
			if pattern.MatchString(text) {
				family := canonicalRuleID(rule.ID)
				if _, seen := countedFamilies[family]; !seen {
					score += rule.RiskThreshold
					countedFamilies[family] = struct{}{}
				}
				matched[rule.ID] = struct{}{}
				if topRule == nil || rule.Priority < topRule.Priority {
					copyRule := rule
					topRule = &copyRule
				}
				break
			}
		}
	}
	if score > 100 {
		score = 100
	}
	if score > 0 && score < minPositiveThreshold(rules, globalThreshold) {
		score = max(score, 1)
	}

	matchedRules := make([]string, 0, len(matched))
	for id := range matched {
		matchedRules = append(matchedRules, id)
	}
	sort.Strings(matchedRules)
	return score, matchedRules, topRule
}

func minPositiveThreshold(rules []runtimeRule, fallback int) int {
	minValue := fallback
	for _, rule := range rules {
		if rule.RiskThreshold > 0 && rule.RiskThreshold < minValue {
			minValue = rule.RiskThreshold
		}
	}
	return minValue
}

func (pr *PolicyRuntime) rebuildLocked() {
	pr.config = normalizePolicyConfig(pr.config)
	pr.rules = compilePolicyRules(pr.config.Rules)
	pr.nextRuleID = detectNextRuleID(pr.config.Rules)
}

func (pr *PolicyRuntime) persistLocked() error {
	if pr.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(pr.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pr.path, data, 0o644)
}

func compilePolicyRules(rules []PolicyRuleConfig) []runtimeRule {
	sorted := clonePolicyRules(rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	compiled := make([]runtimeRule, 0, len(sorted))
	for _, rule := range sorted {
		if !rule.Enabled {
			continue
		}
		patterns := make([]*regexp.Regexp, 0, 1)
		if strings.TrimSpace(rule.Condition) != "" {
			patterns = append(patterns, regexp.MustCompile(rule.Condition))
		}
		compiled = append(compiled, runtimeRule{
			ID:            rule.ID,
			GateType:      rule.GateType,
			Action:        parseRuleAction(rule.Action),
			RiskThreshold: rule.RiskThreshold,
			Priority:      rule.Priority,
			Patterns:      patterns,
		})
	}
	return compiled
}

func parseRuleAction(value string) Decision {
	switch strings.TrimSpace(value) {
	case "Block":
		return Block
	case "Degrade":
		return Degrade
	case "Deny":
		return Deny
	case "HumanApproval":
		return HumanApproval
	default:
		return Allow
	}
}

func normalizePolicyConfig(cfg PolicyConfig) PolicyConfig {
	cfg.Rules = clonePolicyRules(cfg.Rules)
	if cfg.RiskWeights.Alpha == 0 && cfg.RiskWeights.Beta == 0 && cfg.RiskWeights.Gamma == 0 {
		cfg.RiskWeights = defaultPolicyConfig().RiskWeights
	}
	if cfg.GlobalThreshold <= 0 {
		cfg.GlobalThreshold = defaultPolicyConfig().GlobalThreshold
	}
	for i := range cfg.Rules {
		cfg.Rules[i] = normalizePolicyRule(cfg.Rules[i])
	}
	sort.Slice(cfg.Rules, func(i, j int) bool {
		return cfg.Rules[i].Priority < cfg.Rules[j].Priority
	})
	return cfg
}

func normalizePolicyRule(rule PolicyRuleConfig) PolicyRuleConfig {
	rule.GateType = strings.TrimSpace(rule.GateType)
	rule.Action = strings.TrimSpace(rule.Action)
	if rule.Priority <= 0 {
		rule.Priority = 1
	}
	if rule.RiskThreshold < 0 {
		rule.RiskThreshold = 0
	}
	if rule.RiskThreshold > 100 {
		rule.RiskThreshold = 100
	}
	return rule
}

func clonePolicyConfig(cfg PolicyConfig) PolicyConfig {
	cfg.Rules = clonePolicyRules(cfg.Rules)
	return cfg
}

func clonePolicyRules(rules []PolicyRuleConfig) []PolicyRuleConfig {
	out := make([]PolicyRuleConfig, len(rules))
	copy(out, rules)
	return out
}

func detectNextRuleID(rules []PolicyRuleConfig) int {
	maxID := 100
	for _, rule := range rules {
		if !strings.HasPrefix(rule.ID, "rule-custom-") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(rule.ID, "rule-custom-"))
		if err == nil && n > maxID {
			maxID = n
		}
	}
	return maxID
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func canonicalRuleID(ruleID string) string {
	for _, baseID := range defaultRuleIDs {
		if ruleID == baseID || strings.HasPrefix(ruleID, baseID+"_") {
			return baseID
		}
	}
	return ruleID
}
