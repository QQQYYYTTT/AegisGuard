package contract

import "time"

// ============================================================================
// [分工3] 控制平面 → 执行平面：审计日志查询
// ============================================================================

// AuditReader [UNDEFINED] [分工3] 审计日志读取接口
//
// 控制平面暴露给执行平面的能力：查询攻击链和审计统计。
type AuditReader interface {
	Chains(severity string, limit int) ([]AttackChain, error)
	Stats() (*AuditStats, error)
}

// AttackChain [分工3] 攻击链分析结果
type AttackChain struct {
	ID        string    `json:"id"`
	Severity  string    `json:"severity"`
	Steps     []string  `json:"steps"`
	Summary   string    `json:"summary"`
	Timestamp time.Time `json:"timestamp"`
}

// AuditStats [分工3] 审计统计数据
type AuditStats struct {
	TotalEvents   int            `json:"total_events"`
	BlockedCount  int            `json:"blocked_count"`
	AllowedCount  int            `json:"allowed_count"`
	ByGate        map[string]int `json:"by_gate"`
	BySeverity    map[string]int `json:"by_severity"`
}
