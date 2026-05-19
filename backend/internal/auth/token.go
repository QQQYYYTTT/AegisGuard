// backend/internal/auth/token.go
// Package auth 实现 AegisGuard 的授权令牌机制，基于国密 SM2 数字签名算法
package auth

import (
	"aegisguard/pkg/smcrypto"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/emmansun/gmsm/sm2"
)

// 全局签名密钥和 UID
// 这些变量在控制平面初始化时设置，用于令牌的签发和验证
var (
	signingPrivateKey atomic.Pointer[sm2.PrivateKey]    // 签名私钥（控制平面持有），使用 atomic 保证线程安全
	signingUID        = []byte("AegisGuard-Agent-Auth") // 自定义 UID，用于区分 AegisGuard 系统
)

// InitSigningKey 初始化签名私钥
// 参数：
//   - privateKeyHex: 私钥的十六进制字符串
//   - 如果为空字符串，则自动生成新的密钥对
//   - 如果提供值，则从十六进制加载已有私钥
//
// 返回：错误信息
func InitSigningKey(privateKeyHex string) error {
	// 如果未提供私钥，自动生成新的密钥对
	if privateKeyHex == "" {
		keyPair, err := smcrypto.GenerateKeyPair()
		if err != nil {
			return err
		}
		signingPrivateKey.Store(keyPair.PrivateKey)
		return nil
	}

	// 从十六进制字符串加载私钥
	privKey, err := smcrypto.LoadPrivateKeyFromHex(privateKeyHex)
	if err != nil {
		return err
	}
	signingPrivateKey.Store(privKey)
	return nil
}

// GetSigningPublicKey 获取签名公钥
// 返回：SM2 公钥对象，用于令牌验证
// 注意：如果私钥未初始化，返回 nil
func GetSigningPublicKey() *ecdsa.PublicKey {
	privKey := signingPrivateKey.Load()
	if privKey == nil {
		return nil
	}
	// 从私钥中提取公钥
	return &privKey.PublicKey
}

// RequireToken 运行时授权令牌结构
// 该令牌由控制平面签发，执行平面（Agent）携带此令牌访问受保护资源
// 所有字段都参与签名，确保令牌的完整性和真实性
type RequireToken struct {
	ToolName   string    `json:"tool_name"`   // 工具名称，标识请求的工具类型
	Scope      string    `json:"scope"`       // 权限范围，如 "read", "write", "admin"
	AgentID    string    `json:"agent_id"`    // Agent 唯一标识符
	SessionID  string    `json:"session_id"`  // 会话 ID，用于会话管理
	TaskID     string    `json:"task_id"`     // 任务 ID，关联具体任务
	ExpiresAt  time.Time `json:"expires_at"`  // 过期时间，超时后令牌失效
	Nonce      string    `json:"nonce"`       // 随机数，防止重放攻击
	RiskLevel  int       `json:"risk_level"`  // 风险等级，由策略中心评估（0-10）
	SchemaHash string    `json:"schema_hash"` // 工具 Schema 的 SM3 哈希，防止参数篡改
	MaxCalls   int       `json:"max_calls"`   // 最大调用次数预算（SAGA 风格防 DoS），0 表示无限制
	CallCount  int       `json:"call_count"`  // 当前已调用次数（由上层应用控制递增，不参与签名）
	Signature  string    `json:"signature"`   // SM2 数字签名，确保令牌真实性
}

// NewToken 签发新的授权令牌（由控制平面调用）
// 参数：
//   - toolName: 工具名称
//   - scope: 权限范围
//   - agentID: Agent ID
//   - sessionID: 会话 ID
//   - taskID: 任务 ID
//   - ttl: 令牌有效期（如 5*time.Minute）
//   - maxCalls: 最大调用次数预算（防 DoS），0 表示无限制
//
// 返回：签名后的令牌对象或错误
func NewToken(toolName, scope, agentID, sessionID, taskID string, ttl time.Duration, maxCalls int) (*RequireToken, error) {
	// 生成随机数（16 字节，32 个十六进制字符）
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 创建令牌对象（此时签名尚未生成）
	token := &RequireToken{
		ToolName:  toolName,
		Scope:     scope,
		AgentID:   agentID,
		SessionID: sessionID,
		TaskID:    taskID,
		ExpiresAt: time.Now().Add(ttl), // 计算过期时间
		Nonce:     hex.EncodeToString(nonce),
		RiskLevel: 0,        // 默认风险等级为 0，后续由策略中心评估
		MaxCalls:  maxCalls, // 设置调用次数预算
		CallCount: 0,        // 初始调用次数为 0
	}

	// 对令牌进行 SM2 签名
	if err := token.Sign(); err != nil {
		return nil, err
	}

	return token, nil
}

// Sign 对当前令牌进行 SM2 数字签名
// 该方法会修改令牌的 Signature 字段
// 返回：错误信息
func (t *RequireToken) Sign() error {
	// 检查签名私钥是否已初始化
	privKey := signingPrivateKey.Load()
	if privKey == nil {
		return fmt.Errorf("signing key not initialized")
	}

	// 构建待签名的消息（包含令牌的所有关键字段）
	message := t.buildSignMessage()

	// 使用 SM2 算法进行签名
	signature, err := smcrypto.SignMessageHex(privKey, message, signingUID)
	if err != nil {
		return fmt.Errorf("failed to sign token: %w", err)
	}

	// 将签名存储到令牌中
	t.Signature = signature
	return nil
}

// buildSignMessage 构建用于签名的消息体
// 将令牌的关键字段拼接成字符串，确保签名的完整性和可验证性
// 返回：待签名的字节数组
func (t *RequireToken) buildSignMessage() []byte {
	// 使用 "|" 作为分隔符拼接字段
	// 格式：ToolName|Scope|AgentID|SessionID|TaskID|Nonce|RiskLevel|SchemaHash|MaxCalls
	// 注意：CallCount 不参与签名，因为它会在每次调用时递增
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%s|%d",
		t.ToolName,
		t.Scope,
		t.AgentID,
		t.SessionID,
		t.TaskID,
		t.Nonce,
		t.RiskLevel,
		t.SchemaHash,
		t.MaxCalls,
	)
	return []byte(data)
}

// buildCacheKey 构建用于验证缓存的 key
// 使用相对稳定的字段组合，确保相同 token 在有效期内可以复用缓存
// 注意：不包含 CallCount（会变化）、Nonce（每次调用都变化）、Expiry（时间变化）
func (t *RequireToken) buildCacheKey() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%d",
		t.ToolName,
		t.Scope,
		t.AgentID,
		t.SessionID,
		t.TaskID,
		t.SchemaHash,
		t.RiskLevel,
		t.MaxCalls,
	)
}
