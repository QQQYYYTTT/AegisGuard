package gates

import "aegisguard/internal/interfaces"

// Decision 决策类型（类型别名，主定义在 interfaces 包）
type Decision = interfaces.Decision

// 重新导出门控决策常量
const (
	Allow         = interfaces.Allow
	Block         = interfaces.Block
	Degrade       = interfaces.Degrade
	Deny          = interfaces.Deny
	HumanApproval = interfaces.HumanApproval
)
