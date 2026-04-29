// internal/vkey/manager.go
// 虚拟密钥管理模块 - 实现低侵入接入的核心组件
// Agent 使用 vsk- 前缀的虚拟密钥，网关自动替换为真实 API Key
package vkey

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// VirtualKey 虚拟密钥结构体
// 包含 Agent 身份信息和对应的真实 LLM API Key
type VirtualKey struct {
	KeyID      string `json:"key_id" yaml:"key_id" mapstructure:"key_id"`                   // 虚拟密钥 ID，格式：vsk-xxxxx
	AgentID    string `json:"agent_id" yaml:"agent_id" mapstructure:"agent_id"`             // Agent 唯一标识
	RealAPIKey string `json:"real_api_key" yaml:"real_api_key" mapstructure:"real_api_key"` // 真实的 LLM API Key (sk-xxx)
	Scope      string `json:"scope" yaml:"scope" mapstructure:"scope"`                      // 权限范围：read/write/admin
	SessionID  string `json:"session_id" yaml:"session_id" mapstructure:"session_id"`       // 会话 ID
	RateLimit  int    `json:"rate_limit" yaml:"rate_limit" mapstructure:"rate_limit"`       // 每分钟最大请求数，0 表示无限制
	ExpiresAt  string `json:"expires_at" yaml:"expires_at" mapstructure:"expires_at"`       // 过期时间，ISO 8601 格式
	CreatedAt  string `json:"created_at" yaml:"created_at" mapstructure:"created_at"`       // 创建时间，ISO 8601 格式
	Enabled    bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`                // 是否启用
}

// parseExpiresAt 解析过期时间
func (vk *VirtualKey) parseExpiresAt() (time.Time, error) {
	if vk.ExpiresAt == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, vk.ExpiresAt)
}

// parseCreatedAt 解析创建时间
func (vk *VirtualKey) parseCreatedAt() (time.Time, error) {
	if vk.CreatedAt == "" {
		return time.Now(), nil
	}
	return time.Parse(time.RFC3339, vk.CreatedAt)
}

// IsExpired 检查是否已过期
func (vk *VirtualKey) IsExpired() bool {
	if vk.ExpiresAt == "" {
		return false
	}
	expiresAt, err := vk.parseExpiresAt()
	if err != nil {
		return false
	}
	return time.Now().After(expiresAt)
}

// Manager 虚拟密钥管理器
type Manager struct {
	mu         sync.RWMutex
	store      Store
	keys       map[string]*VirtualKey // key: vsk-xxxxx  内存缓存
	logger     *zap.Logger
	configPath string
}

// NewManager 创建虚拟密钥管理器
func NewManager(logger *zap.Logger, configPath string) (*Manager, error) {
	store, err := CreateStore("sqlite:data/vkeys.db")
	if err != nil {
		logger.Warn("创建 SQLite 存储失败，降级为内存存储", zap.Error(err))
		store = nil
	}
	return NewManagerWithStore(logger, configPath, store)
}

// NewManagerWithStore 创建虚拟密钥管理器（指定存储后端）
func NewManagerWithStore(logger *zap.Logger, configPath string, store Store) (*Manager, error) {
	m := &Manager{
		keys:       make(map[string]*VirtualKey),
		store:      store,
		logger:     logger,
		configPath: configPath,
	}

	if store != nil {
		if err := m.loadFromStore(); err != nil {
			logger.Warn("从存储加载虚拟密钥失败，使用配置文件", zap.Error(err))
			if err := m.loadFromConfig(); err != nil {
				logger.Warn("加载虚拟密钥配置失败，使用空配置", zap.Error(err))
			}
		}
	} else {
		if err := m.loadFromConfig(); err != nil {
			logger.Warn("加载虚拟密钥配置失败，使用空配置", zap.Error(err))
		}
	}

	return m, nil
}

// loadFromStore 从持久化存储加载虚拟密钥
func (m *Manager) loadFromStore() error {
	ctx := context.Background()
	keys, err := m.store.List(ctx)
	if err != nil {
		return fmt.Errorf("从存储加载密钥失败: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		key := key
		m.keys[key.KeyID] = key
	}

	m.logger.Info("从存储加载虚拟密钥", zap.Int("count", len(m.keys)))
	return nil
}

// loadFromConfig 从配置文件加载虚拟密钥
func (m *Manager) loadFromConfig() error {
	viper.SetConfigFile(m.configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var keys []VirtualKey
	if err := viper.UnmarshalKey("virtual_keys", &keys); err != nil {
		return fmt.Errorf("解析虚拟密钥配置失败：%w", err)
	}

	m.logger.Info("【调试】YAML 解析结果", zap.Int("keys 数量", len(keys)))
	for i, key := range keys {
		m.logger.Info("【调试】密钥详情", zap.Int("索引", i), zap.String("key_id", key.KeyID), zap.String("agent_id", key.AgentID), zap.String("real_key", key.RealAPIKey))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		key := key // 创建局部副本，避免循环变量地址引用问题
		m.keys[key.KeyID] = &key
	}

	m.logger.Info("虚拟密钥加载完成", zap.Int("count", len(m.keys)))
	return nil
}

// ExtractVKey 从 Authorization Header 中提取虚拟密钥
// 格式: Bearer vsk-xxxxx -> vsk-xxxxx
func ExtractVKey(authHeader string) string {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}

	key := strings.TrimPrefix(authHeader, "Bearer ")
	key = strings.TrimSpace(key)

	// 检查是否是虚拟密钥格式
	if strings.HasPrefix(key, "vsk-") {
		return key
	}

	return ""
}

// IsVirtualKey 检查密钥是否是虚拟密钥格式
func IsVirtualKey(key string) bool {
	return strings.HasPrefix(key, "vsk-")
}

// IsRealAPIKey 检查密钥是否是真实 API Key 格式
func IsRealAPIKey(key string) bool {
	return strings.HasPrefix(key, "sk-") || strings.HasPrefix(key, "sk-proj-")
}

// ValidateAndResolve 验证虚拟密钥并返回真实密钥信息
// 这是低侵入接入的核心：Agent 用 vsk- 请求，网关返回真实的 sk- 信息
func (m *Manager) ValidateAndResolve(vkeyID string) (*VirtualKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vk, ok := m.keys[vkeyID]
	if !ok {
		return nil, fmt.Errorf("虚拟密钥不存在: %s", vkeyID)
	}

	if !vk.Enabled {
		return nil, fmt.Errorf("虚拟密钥已禁用：%s", vkeyID)
	}

	if vk.IsExpired() {
		return nil, fmt.Errorf("虚拟密钥已过期：%s", vkeyID)
	}

	return vk, nil
}

// GetRealAPIKey 获取虚拟密钥对应的真实 API Key
func (m *Manager) GetRealAPIKey(vkeyID string) (string, error) {
	vk, err := m.ValidateAndResolve(vkeyID)
	if err != nil {
		return "", err
	}
	return vk.RealAPIKey, nil
}

// CreateKey 创建新的虚拟密钥
func (m *Manager) CreateKey(agentID, realAPIKey, scope string, rateLimit int, ttl time.Duration) (*VirtualKey, error) {
	if !IsRealAPIKey(realAPIKey) {
		return nil, fmt.Errorf("无效的 real API Key 格式，必须以 sk- 开头")
	}

	vkeyID := generateVKeyID()

	vk := &VirtualKey{
		KeyID:      vkeyID,
		AgentID:    agentID,
		RealAPIKey: realAPIKey,
		Scope:      scope,
		RateLimit:  rateLimit,
		CreatedAt:  time.Now().Format(time.RFC3339),
		Enabled:    true,
	}

	if ttl > 0 {
		vk.ExpiresAt = time.Now().Add(ttl).Format(time.RFC3339)
	}

	m.mu.Lock()
	m.keys[vkeyID] = vk
	m.mu.Unlock()

	if m.store != nil {
		ctx := context.Background()
		if err := m.store.Save(ctx, vk); err != nil {
			m.logger.Warn("持久化虚拟密钥失败", zap.Error(err))
		}
	}

	m.logger.Info("创建虚拟密钥",
		zap.String("vkey_id", vkeyID),
		zap.String("agent_id", agentID),
		zap.String("scope", scope),
	)

	return vk, nil
}

// RevokeKey 吊销虚拟密钥
func (m *Manager) RevokeKey(vkeyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vk, ok := m.keys[vkeyID]
	if !ok {
		return fmt.Errorf("虚拟密钥不存在: %s", vkeyID)
	}

	vk.Enabled = false

	if m.store != nil {
		ctx := context.Background()
		if err := m.store.Update(ctx, vk); err != nil {
			m.logger.Warn("持久化吊销状态失败", zap.Error(err))
		}
	}

	m.logger.Info("吊销虚拟密钥", zap.String("vkey_id", vkeyID))
	return nil
}

// ListKeys 列出所有虚拟密钥（脱敏）
func (m *Manager) ListKeys() []*VirtualKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*VirtualKey, 0, len(m.keys))
	for _, vk := range m.keys {
		// 脱敏：隐藏真实 API Key
		vkCopy := *vk
		vkCopy.RealAPIKey = maskAPIKey(vk.RealAPIKey)
		result = append(result, &vkCopy)
	}
	return result
}

// GetKeyInfo 获取虚拟密钥信息
func (m *Manager) GetKeyInfo(vkeyID string) (*VirtualKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vk, ok := m.keys[vkeyID]
	if !ok {
		return nil, fmt.Errorf("虚拟密钥不存在: %s", vkeyID)
	}

	// 返回脱敏副本
	vkCopy := *vk
	vkCopy.RealAPIKey = maskAPIKey(vk.RealAPIKey)
	return &vkCopy, nil
}

// generateVKeyID 生成虚拟密钥 ID
// 格式: vsk-{uuid前8位}-{uuid前4位}
func generateVKeyID() string {
	uid := uuid.New().String()
	return fmt.Sprintf("vsk-%s-%s", uid[:8], uid[9:13])
}

// maskAPIKey 脱敏 API Key
// sk-a7AxxxxxxxxxxxxxxxxxqboexHelKt -> sk-a7A...HelKt
func maskAPIKey(key string) string {
	if len(key) <= 12 {
		return "***"
	}
	return key[:7] + "..." + key[len(key)-6:]
}
