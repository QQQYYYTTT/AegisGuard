package gateway

import (
	"net/http"
	"time"

	"aegisguard/internal/auth"
)

// TokenIssuer 提供最小可运行的 RequireToken 签发能力。
type TokenIssuer interface {
	Issue(toolName, scope, agentID, sessionID, taskID string, ttl time.Duration, maxCalls int) (*auth.RequireToken, error)
}

// TokenInjector [WRONG] [分工2] Token 注入器
//
// proxy.go 需要：将真实签发的 RequireToken 注入到转发请求中。
// 当前实现状态：
//   - GenerateAndInject: [WRONG] proxy.go 的 injectToken 注入的是 "placeholder-token" 字符串，
//     需替换为真实签发逻辑
type TokenInjector interface {
	GenerateAndInject(toolName string, params map[string]interface{}, r *http.Request) error
}
