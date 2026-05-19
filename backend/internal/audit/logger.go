package audit

import (
	"fmt"
	"sync"
	"time"

	"aegisguard/pkg/smcrypto"
)

type Logger struct {
	store  Storer
	pended map[string]AuditEvent
	mu     sync.Mutex
}

func NewLogger(store Storer) *Logger {
	return &Logger{
		store:  store,
		pended: make(map[string]AuditEvent),
	}
}

func (l *Logger) LogRequest(input LogInput) string {
	if input.RequestID == "" {
		input.RequestID = fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}

	bodyHash := computeMetaFingerprint(input)

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

func computeMetaFingerprint(input LogInput) string {
	meta := fmt.Sprintf("%s|%s|%s|%s|%s|%d",
		input.RequestID,
		input.Method,
		input.Path,
		input.ClientIP,
		input.GatewayKey,
		len(input.Body),
	)
	return smcrypto.SM3Hex([]byte(meta))
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
	ev.TokenStatus = input.TokenStatus
	ev.AuthMode = input.AuthMode
	ev.UnauthorizedAllow = input.UnauthorizedAllow
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
