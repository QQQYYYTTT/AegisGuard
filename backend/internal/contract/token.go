// Package contract 定义控制平面与执行平面之间的通信契约接口。
//
// 设计原则：
//   - 控制平面包（auth/, audit/, rules/）通过此包暴露能力给执行平面
//   - 执行平面包（gates/, gateway/, sandbox/）通过此包暴露能力给控制平面
//   - http/（路由层/组合根）依赖此包组装两个平面
//   - 任何实现包不直接 import 另一个平面的实现包
//
// 部署模式：
//   - 单进程：组合根将具体实现注入给消费者
//   - 多进程/容器化：只需将 contract 接口替换为 HTTP/gRPC client 实现
package contract

import (
	"time"

	"aegisguard/internal/auth"
	"aegisguard/internal/interfaces"
)

// ============================================================================
// [分工2] 控制平面 → 执行平面：Token 签发与校验
// ============================================================================

// TokenIssuer [PARTIAL] [分工2] 控制平面 Token 签发接口
//
// 控制平面暴露给执行平面的能力：签发、吊销、查询 token。
// 当前实现由 auth 包提供，状态：
//   - Issue:  [PARTIAL] auth/token.go 有 NewToken 可本地签发，但无 HTTP 接口暴露
//   - Revoke: [UNDEFINED] 无吊销逻辑
//   - List:   [UNDEFINED] 无 token 列表查询
//   - GetByID [UNDEFINED] 无 token 查询
type TokenIssuer interface {
	Issue(toolName, scope, agentID, sessionID, taskID string, ttl time.Duration, maxCalls int) (*auth.RequireToken, error)
	Revoke(tokenID string) error
	ListActive() []auth.RequireToken
	GetByID(tokenID string) (*auth.RequireToken, error)
}

// TokenVerifier [PARTIAL] [分工2] 执行平面 Token 校验接口
//
// 执行平面暴露给控制平面的能力：校验 token 完整链路。
// 当前实现由 auth.Verifier 提供，状态：
//   - Verify:            [PARTIAL] auth/verifier.go 有 Verify 方法，做签名/过期/nonce/调用次数/SchemaHash 校验
//   - CheckNonce:        [PARTIAL] auth/verifier.go 用内存 map，需持久化/TTL 缓存
//   - VerifyAgentBinding:[UNDEFINED] 未校验 agent 绑定
//   - VerifySession:     [UNDEFINED] 未校验 session 绑定
//   - VerifyScope:       [WRONG] gates/action.go 的 checkScope 仅做字符串前缀匹配，需改为精细化校验
type TokenVerifier interface {
	Verify(token *auth.RequireToken) error
	CheckNonce(nonce string) (bool, error)
	VerifyAgentBinding(tokenAgentID, requestAgentID string) bool
	VerifySessionBinding(tokenSessionID, requestSessionID string) bool
	VerifyScopeFineGrained(scope string, resource interfaces.ResourceDescriptor) bool
}

// AuthStatus [分工3] 认证系统状态（控制平面提供给管理界面的数据）
type AuthStatus struct {
	SM2Active     bool      `json:"sm2_active"`
	SM3Active     bool      `json:"sm3_active"`
	SM4Active     bool      `json:"sm4_active"`
	KeyExpiresAt  time.Time `json:"key_expires_at"`
	ActiveTokens  int       `json:"active_tokens"`
	RevokedTokens int       `json:"revoked_tokens"`
}
