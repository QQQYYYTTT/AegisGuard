package audit

import (
	"fmt"
	"sync"
	"time"

	"aegisguard/pkg/smcrypto"
)

// maxBodyHashBytes 审计 body 哈希截取的最大字节数
// 存前 1KB 的 SM3 哈希，既保留了注入检测的语义，又避免存储大型 payload
const maxBodyHashBytes = 1024

// Logger 审计日志记录器
// 负责组装 AuditEvent 并通过 Store 持久化
type Logger struct {
	store  *Store
	pended map[string]AuditEvent // request_id → 未完成的审计事件
	mu     sync.Mutex
}

// NewLogger 创建审计日志记录器
func NewLogger(store *Store) *Logger {
	return &Logger{
		store:  store,
		pended: make(map[string]AuditEvent),
	}
}

// LogRequest 记录请求进入
// 参数：
//   - input: 请求阶段信息
//
// 返回：requestID，调用方需在请求结束时传入 LogResponse
//
// 此方法不写入 Store，仅缓存 pending 事件。
// 完整的 AuditEvent 在 LogResponse 调用时一次性写入。
func (l *Logger) LogRequest(input LogInput) string {
	if input.RequestID == "" {
		input.RequestID = fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}

	bodyHash := smcrypto.SM3HexTruncated(input.Body, maxBodyHashBytes)

	ev := AuditEvent{
		RequestID:  input.RequestID,
		Timestamp:  time.Now(),
		GatewayKey: input.GatewayKey,
		Method:     input.Method,
		Path:       input.Path,
		BodyHash:   bodyHash,
		ClientIP:   input.ClientIP,
	}

	l.mu.Lock()
	l.pended[input.RequestID] = ev
	l.mu.Unlock()

	return input.RequestID
}

// LogResponse 记录请求完成并写入持久化存储
// 参数：
//   - requestID: LogRequest 返回的 ID
//   - input: 响应阶段信息
//
// 如果 requestID 对应的 pending 事件不存在（例如超时清理），
// 仍构造一条最小审计记录以确保不丢事件
func (l *Logger) LogResponse(requestID string, input LogResponseInput) {
	l.mu.Lock()
	ev, ok := l.pended[requestID]
	if ok {
		delete(l.pended, requestID)
	}
	l.mu.Unlock()

	// 更新完成信息
	ev.StatusCode = input.StatusCode
	ev.DurationMs = input.Duration.Milliseconds()
	ev.Decision = input.Decision
	ev.Reason = input.Reason
	ev.GateType = input.GateType
	ev.RiskScore = input.RiskScore
	ev.RiskLevel = input.RiskLevel
	ev.MatchedRules = input.MatchedRules
	ev.Error = input.Error
	if requestID != "" {
		ev.RequestID = requestID
	}

	// 写入持久化存储
	if err := l.store.Append(ev); err != nil {
		// Store 写入失败无法恢复，静默丢弃
		// 后续可考虑增加重试或告警机制
	}
}

// PendingCount 返回当前待完成的审计事件数
// 用于健康检查和调试
func (l *Logger) PendingCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.pended)
}
