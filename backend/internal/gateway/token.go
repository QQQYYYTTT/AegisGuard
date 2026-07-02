package gateway

import (
	"net/http"

	"aegisguard/internal/auth"
)

// TokenIssuer 复用 auth.TokenStore 存储契约，避免网关层持有一套更弱的并行接口。
type TokenIssuer = auth.TokenStore

// TokenInjector [WRONG] [分工2] Token 注入器
//
// proxy.go 需要：将真实签发的 RequireToken 注入到转发请求中。
// 当前实现状态：
//   - GenerateAndInject: [WRONG] proxy.go 的 injectToken 注入的是 "placeholder-token" 字符串，
//     需替换为真实签发逻辑
type TokenInjector interface {
	GenerateAndInject(toolName string, params map[string]interface{}, r *http.Request) error
}
