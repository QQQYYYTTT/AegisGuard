package audit

import "time"

// AuditEvent 结构化审计事件
// 为每个经过网关的请求生成一条完整记录
type AuditEvent struct {
	RequestID  string    `json:"request_id"`            // 请求唯一标识 (UUID)
	Timestamp  time.Time `json:"timestamp"`             // 事件完成时间戳
	GatewayKey string    `json:"gateway_key,omitempty"` // 网关密钥标识
	Method     string    `json:"method"`                // HTTP 方法
	Path       string    `json:"path"`                  // 请求路径
	StatusCode int       `json:"status_code"`           // HTTP 状态码
	DurationMs int64     `json:"duration_ms"`           // 耗时（毫秒）
	BodyHash   string    `json:"body_hash,omitempty"`   // 请求体 SM3 哈希（前 1KB）
	ClientIP   string    `json:"client_ip,omitempty"`   // 客户端 IP
	Decision   string    `json:"decision,omitempty"`    // 闸门决策: allow/block/degrade/deny
	Reason     string    `json:"reason,omitempty"`      // 决策原因
	Error      string    `json:"error,omitempty"`       // 错误信息
}

// LogInput 请求阶段日志输入
type LogInput struct {
	RequestID  string
	GatewayKey string
	Method     string
	Path       string
	Body       []byte // 用于计算哈希，不持久化完整 body
	ClientIP   string
}

// LogResponseInput 响应阶段日志输入
type LogResponseInput struct {
	StatusCode int
	Duration   time.Duration
	Decision   string
	Reason     string
	Error      string
}
