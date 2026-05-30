package httpapi

import (
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

type PolicyRule struct {
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

type RiskWeights struct {
	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
	Gamma float64 `json:"gamma"`
}

type PolicyConfig struct {
	RiskWeights     RiskWeights  `json:"risk_weights"`
	GlobalThreshold int          `json:"global_threshold"`
	Rules           []PolicyRule `json:"rules"`
}

var (
	policyMu     sync.RWMutex
	policyConfig = defaultPolicyConfig()
	nextRuleID   = 100
)

func defaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		RiskWeights: RiskWeights{
			Alpha: 0.35,
			Beta:  0.40,
			Gamma: 0.25,
		},
		GlobalThreshold: 85,
		Rules: []PolicyRule{
			{ID: "rule-1", Name: "Prompt Injection 检测", Description: "检测提示注入攻击，包含忽略/绕过系统指令等模式", GateType: "message", Condition: `(?i)\b(ignore|forget|bypass|override|system prompt|jailbreak)\b`, Action: "Block", Priority: 1, Enabled: true, RiskThreshold: 35},
			{ID: "rule-2", Name: "越权操作检测", Description: "检测企图获取管理员权限、执行敏感系统命令的行为", GateType: "action", Condition: `(?i)\b(admin|root|sudo|privileged|shell|exec|spawn)\b`, Action: "Block", Priority: 2, Enabled: true, RiskThreshold: 30},
			{ID: "rule-3", Name: "敏感数据泄露防护", Description: "保护 API Key、密码、Token 等敏感凭证不被泄露", GateType: "return", Condition: `(?i)\b(api[_-]?key|password|secret|private key|sk-[A-Za-z0-9]{8,})\b`, Action: "Block", Priority: 3, Enabled: true, RiskThreshold: 35},
			{ID: "rule-4", Name: "高危操作拦截", Description: "拦截生产环境删除、转账、支付等高危操作", GateType: "action", Condition: `(?i)\b(delete|transfer|wire|withdraw|pay|refund)\b.{0,80}\b(production|customer|account|database|payment|fund)\b`, Action: "Block", Priority: 4, Enabled: true, RiskThreshold: 32},
			{ID: "rule-5", Name: "记忆投毒防护", Description: "检测并阻止试图污染 Agent 长期记忆的恶意指令", GateType: "return", Condition: `(?i)\b(save|store|remember|persist)\b.{0,80}\b(instruction|rule|memory|policy)\b`, Action: "Block", Priority: 5, Enabled: true, RiskThreshold: 45},
			{ID: "rule-6", Name: "非法金融活动检测", Description: "检测洗钱、内幕交易、逃税等非法金融活动模式", GateType: "message", Condition: `(?i)\b(money laundering|insider trading|tax evasion|fraudulent|stolen card)\b`, Action: "Block", Priority: 6, Enabled: true, RiskThreshold: 70},
			{ID: "rule-7", Name: "正常请求放行", Description: "允许所有不匹配任何高危模式的正常用户请求", GateType: "message", Condition: "", Action: "Allow", Priority: 7, Enabled: true, RiskThreshold: 0},
			{ID: "rule-8", Name: "回调污染检测", Description: "检测外部工具返回结果中注入恶意指令的回调污染攻击", GateType: "return", Condition: `(?i)\b(observation injection|tool output|external content)\b.{0,80}\b(instruction|command|override)\b`, Action: "Degrade", Priority: 8, Enabled: true, RiskThreshold: 25},
			{ID: "rule-9", Name: "重放攻击检测", Description: "识别会话重放、重复提权操作等重放攻击行为", GateType: "action", Condition: `(?i)\b(replay|repeat|again|retry)\b.{0,80}\b(privileged|action|export|admin)\b`, Action: "Deny", Priority: 9, Enabled: true, RiskThreshold: 40},
			{ID: "rule-10", Name: "工具误用检测", Description: "检测 Agent 工具调用超出合理范围的行为", GateType: "action", Condition: `(?i)\b(delete_file|shell_exec|drop_table|rm\s+-rf|format)\b`, Action: "Block", Priority: 10, Enabled: true, RiskThreshold: 50},
		},
	}
}

func (r *Router) handleGetPolicyConfig(c *gin.Context) {
	policyMu.RLock()
	defer policyMu.RUnlock()

	rules := make([]PolicyRule, len(policyConfig.Rules))
	copy(rules, policyConfig.Rules)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	resp := PolicyConfig{
		RiskWeights:     policyConfig.RiskWeights,
		GlobalThreshold: policyConfig.GlobalThreshold,
		Rules:           rules,
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

func (r *Router) handleGetPolicyRules(c *gin.Context) {
	policyMu.RLock()
	defer policyMu.RUnlock()

	rules := make([]PolicyRule, len(policyConfig.Rules))
	copy(rules, policyConfig.Rules)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rules})
}

func (r *Router) handleUpdatePolicyRule(c *gin.Context) {
	var req PolicyRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	policyMu.Lock()
	defer policyMu.Unlock()

	found := false
	for i, rule := range policyConfig.Rules {
		if rule.ID == req.ID {
			policyConfig.Rules[i].Name = req.Name
			policyConfig.Rules[i].Description = req.Description
			policyConfig.Rules[i].GateType = req.GateType
			policyConfig.Rules[i].Condition = req.Condition
			policyConfig.Rules[i].Action = req.Action
			policyConfig.Rules[i].Priority = req.Priority
			policyConfig.Rules[i].Enabled = req.Enabled
			policyConfig.Rules[i].RiskThreshold = req.RiskThreshold
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "rule not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "rule updated"})
}

func (r *Router) handleCreatePolicyRule(c *gin.Context) {
	var req PolicyRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	policyMu.Lock()
	defer policyMu.Unlock()

	nextRuleID++
	req.ID = "rule-custom-" + strconv.Itoa(nextRuleID)

	policyConfig.Rules = append(policyConfig.Rules, req)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": req, "message": "rule created"})
}

func (r *Router) handleDeletePolicyRule(c *gin.Context) {
	ruleID := c.Param("id")

	policyMu.Lock()
	defer policyMu.Unlock()

	found := false
	for i, rule := range policyConfig.Rules {
		if rule.ID == ruleID {
			policyConfig.Rules = append(policyConfig.Rules[:i], policyConfig.Rules[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "rule not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "rule deleted"})
}

func (r *Router) handleReorderPolicyRules(c *gin.Context) {
	var req struct {
		RuleIDs []string `json:"rule_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request"})
		return
	}

	policyMu.Lock()
	defer policyMu.Unlock()

	ruleMap := make(map[string]PolicyRule, len(policyConfig.Rules))
	for _, rule := range policyConfig.Rules {
		ruleMap[rule.ID] = rule
	}

	reordered := make([]PolicyRule, 0, len(req.RuleIDs))
	for i, id := range req.RuleIDs {
		if r, ok := ruleMap[id]; ok {
			r.Priority = i + 1
			reordered = append(reordered, r)
		}
	}

	for _, rule := range policyConfig.Rules {
		if _, exists := ruleMap[rule.ID]; !exists {
			reordered = append(reordered, rule)
		}
	}

	policyConfig.Rules = reordered
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "rules reordered"})
}

func (r *Router) handleUpdatePolicyConfig(c *gin.Context) {
	var req struct {
		RiskWeights     *RiskWeights `json:"risk_weights,omitempty"`
		GlobalThreshold *int         `json:"global_threshold,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request"})
		return
	}

	policyMu.Lock()
	defer policyMu.Unlock()

	if req.RiskWeights != nil {
		policyConfig.RiskWeights = *req.RiskWeights
	}
	if req.GlobalThreshold != nil {
		policyConfig.GlobalThreshold = *req.GlobalThreshold
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "config updated"})
}
