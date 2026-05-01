package contract

// ============================================================================
// [分工1] 控制平面 → 执行平面：三门统一策略引擎
// ============================================================================

// PolicyEngine [UNDEFINED] [分工1] 三门统一策略引擎接口
//
// 控制平面暴露给执行平面的能力：评分、决策、策略配置。
// 当前实现状态：完全未定义，三门各自独立判断
type PolicyEngine interface {
	Score(eventContext interface{}) (int, []string)
	ShouldBlock(riskScore int) bool
	ShouldDegrade(riskScore int) bool
	ShouldHumanReview(riskScore int) bool
}

// PolicyRule [分工3] 策略规则定义
type PolicyRule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	GateType  string `json:"gate_type"`
	Enabled   bool   `json:"enabled"`
	Priority  int    `json:"priority"`
	Condition string `json:"condition"`
	Action    string `json:"action"`
}

// PolicyConfig [分工3] 策略配置
type PolicyConfig struct {
	Mode  string       `json:"mode"`
	Rules []PolicyRule `json:"rules"`
}
