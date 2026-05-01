package contract

import "aegisguard/internal/interfaces"

// ============================================================================
// [分工4] 执行平面 → 控制平面：Sandbox 与返回隔离
// ============================================================================

// SandboxManager [UNDEFINED] [分工4] Memory Sandbox 管理接口
//
// 执行平面暴露给控制平面的能力：创建、查询、销毁沙箱上下文。
// 当前实现状态：sandbox/sandbox.go 为空壳
type SandboxManager interface {
	CreateContext(trusted interfaces.TrustedContent, untrusted interfaces.UntrustedContent) (*interfaces.SandboxContext, error)
	GetContext(contextID string) (*interfaces.SandboxContext, error)
	DestroyContext(contextID string) error
	ComputeFingerprint(ctx *interfaces.SandboxContext) string
}

// TransferManager [UNDEFINED] [分工4] 可信/不可信区域数据转移管理接口
//
// 执行平面暴露给控制平面的能力：查询上下文转移历史。
// 当前实现状态：完全未定义
type TransferManager interface {
	TrustedToUntrusted(contextID string, fields []string) (*interfaces.TransferRecord, error)
	UntrustedToTrusted(contextID string, data interfaces.UntrustedContent) (*interfaces.TransferRecord, error)
	GetRecords(contextID string, limit int) ([]interfaces.TransferRecord, error)
}

// ContentFilter [UNDEFINED] [分工4] 安全摘要提取与工具返回过滤接口
//
// 执行平面暴露给控制平面的能力：被 ReturnGate 和 Sandbox 间接使用。
// 当前实现状态：完全未定义
type ContentFilter interface {
	ExtractSafeSummary(content string) string
	FilterToolResponse(rawResponse []byte) ([]byte, []string)
}
