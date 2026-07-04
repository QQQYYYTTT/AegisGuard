// Package interfaces 定义 AegisGuard 跨包共享的公共 DTO 类型
//
// 本包只保留纯数据类型（结构体、类型别名），不再包含任何接口定义。
//
// 接口定义已按"消费方定义接口"原则迁至各消费方包：
//
//	gateway/  -> MessageEvaluator, ActionEvaluator, ReturnEvaluator, TokenInjector
//	http/     -> TokenIssuer, TokenVerifier, GateEvaluator, SandboxManager 等
//
// 各消费方接口的当前实现状态，请查阅该包内的接口定义注释。
//
// 分工标记说明（用于状态追踪）：
//
//	[分工1] - MessageGate / ActionGate / ReturnGate 风险检测与决策
//	[分工2] - RequireToken 与授权链路
//	[分工3] - 后端 API 与前后端联调
//	[分工4] - Sandbox 与返回隔离
package interfaces

import (
	"aegisguard/internal/auth"
	"encoding/json"
	"fmt"

	"time"
)

// ============================================================================
// 公共类型别名
// ============================================================================

// Decision 决策类型
type Decision int

const (
	Allow Decision = iota
	Block
	Degrade
	Deny
	HumanApproval
)

// String 返回决策字符串表示
func (d Decision) String() string {
	switch d {
	case Allow:
		return "Allow"
	case Block:
		return "Block"
	case Degrade:
		return "Degrade"
	case Deny:
		return "Deny"
	case HumanApproval:
		return "HumanApproval"
	default:
		return "Unknown"
	}
}

// MarshalJSON 实现 json.Marshaler，将 Decision 序列化为字符串
func (d Decision) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON 实现 json.Unmarshaler，从字符串反序列化 Decision
func (d *Decision) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "Allow":
		*d = Allow
	case "Block":
		*d = Block
	case "Degrade":
		*d = Degrade
	case "Deny":
		*d = Deny
	case "HumanApproval":
		*d = HumanApproval
	default:
		return fmt.Errorf("unknown Decision value: %s", s)
	}
	return nil
}

// RequireToken 借用 auth 包已有定义
type RequireToken = auth.RequireToken

// ============================================================================
// [分工1+2+3] 门控决策记录
// ============================================================================

// GateDecision 三门决策的通用记录格式，用于 HTTP API 返回和审计
type GateDecision struct {
	RequestID    string    `json:"request_id"`
	Timestamp    time.Time `json:"timestamp"`
	GateType     string    `json:"gate_type"`
	Decision     Decision  `json:"decision"`
	RiskScore    int       `json:"risk_score"`
	RiskLevel    string    `json:"risk_level"`
	MatchedRules []string  `json:"matched_rules"`
	Reason       string    `json:"reason"`
	ToolName     string    `json:"tool_name,omitempty"`
	AgentID      string    `json:"agent_id,omitempty"`
}

// EvaluateResult 门控评估的结构化结果，替代 string 格式传递风险元数据
type EvaluateResult struct {
	Decision     Decision
	Reason       string
	RiskScore    int
	RiskLevel    string
	MatchedRules []string
}

// ResourceDescriptor 描述被访问的资源，用于精细化的 scope 校验
type ResourceDescriptor struct {
	Path       string
	Method     string
	Type       string
	Parameters map[string]interface{}
}

// ============================================================================
// [分工4] Sandbox 与返回隔离
// ============================================================================

type TrustedContent struct {
	SystemPrompt    string   `json:"system_prompt"`
	ToolDefinitions []string `json:"tool_definitions"`
	Memory          string   `json:"memory"`
	TaskState       string   `json:"task_state,omitempty"`
}

type UntrustedContent struct {
	UserInput       string `json:"user_input"`
	ExternalData    string `json:"external_data"`
	InjectedContent string `json:"injected_content"`
	Source          string `json:"source,omitempty"`
	ContentType     string `json:"content_type,omitempty"`
}

type SandboxContext struct {
	ContextID      string           `json:"context_id"`
	AgentID        string           `json:"agent_id,omitempty"`
	SessionID      string           `json:"session_id,omitempty"`
	Source         string           `json:"source,omitempty"`
	Trusted        TrustedContent   `json:"trusted"`
	Untrusted      UntrustedContent `json:"untrusted"`
	SM3Fingerprint string           `json:"sm3_fingerprint"`
	RiskScore      int              `json:"risk_score,omitempty"`
	RiskLevel      string           `json:"risk_level,omitempty"`
	Status         string           `json:"status,omitempty"`
	IsolatedAt     time.Time        `json:"isolated_at"`
	UpdatedAt      time.Time        `json:"updated_at,omitempty"`
	ExpiresAt      time.Time        `json:"expires_at,omitempty"`
}

type TransferRecord struct {
	ID              string    `json:"id"`
	ContextID       string    `json:"context_id,omitempty"`
	From            string    `json:"from"`
	To              string    `json:"to"`
	Fields          []string  `json:"fields"`
	Summary         string    `json:"summary"`
	SM3Hash         string    `json:"sm3_hash"`
	RiskScore       int       `json:"risk_score,omitempty"`
	RiskLevel       string    `json:"risk_level,omitempty"`
	Action          string    `json:"action,omitempty"`
	ToolName        string    `json:"tool_name,omitempty"`
	Approved        bool      `json:"approved"`
	Reason          string    `json:"reason,omitempty"`
	MemorySource    string    `json:"memory_source,omitempty"`
	PromotionReason string    `json:"promotion_reason,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}
