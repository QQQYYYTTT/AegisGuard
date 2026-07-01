package contract

import (
	"net/http"

	"aegisguard/internal/interfaces"
)

// ============================================================================
// [分工1+3] 执行平面 → 控制平面：门控评估与查询
// ============================================================================

// GateQuery [UNDEFINED] [分工1+3] 三门决策查询接口（只读）
//
// 执行平面暴露给控制平面的能力：查询三门概览统计和决策历史。
type GateQuery interface {
	Overview() (*GateOverview, error)
	Decisions(limit int, gateType, action string) ([]interfaces.GateDecision, error)
}

// GateEvaluator [UNDEFINED] [分工1+3] 手动触发三门评估接口（调试/模拟用）
//
// 执行平面暴露给控制平面的能力：手动触发门控评估，返回决策。
// 前端对应：gate.ts evaluateGate() -> POST /aegis/gate/evaluate
type GateEvaluator interface {
	EvaluateMessage(requestID string, body []byte, agentID string) interfaces.EvaluateResult
	EvaluateAction(requestID string, toolName string, params map[string]interface{}, headers http.Header, agentID string) interfaces.EvaluateResult
	EvaluateReturn(requestID string, body []byte, agentID string) interfaces.EvaluateResult
}

// GateOverview [分工3] 三门概览统计数据
type GateOverview struct {
	MessageGate     map[string]int            `json:"message_gate"`
	ActionGate      map[string]int            `json:"action_gate"`
	ReturnGate      map[string]int            `json:"return_gate"`
	RecentDecisions []interfaces.GateDecision `json:"recent_decisions"`
}
