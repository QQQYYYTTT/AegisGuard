package vkey

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type gatewayConfig struct {
	GatewayKey string `yaml:"gateway_key" mapstructure:"gateway_key"`
	LLMAPIKey  string `yaml:"llm_api_key" mapstructure:"llm_api_key"`
}

const gatewayKeyPrefix = "agk-"

func ExtractGatewayKey(authHeader string) string {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}

	key := strings.TrimPrefix(authHeader, "Bearer ")
	key = strings.TrimSpace(key)

	if strings.HasPrefix(key, gatewayKeyPrefix) {
		return key
	}

	return ""
}

type Manager struct {
	config *gatewayConfig
	logger *zap.Logger
}

func NewManager(logger *zap.Logger, configPath string) (*Manager, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取网关凭据配置失败: %w", err)
	}

	var cfg gatewayConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析网关凭据配置失败: %w", err)
	}

	if cfg.GatewayKey == "" {
		return nil, fmt.Errorf("gateway_key 不能为空，请在 %s 中配置", configPath)
	}
	if !strings.HasPrefix(cfg.GatewayKey, gatewayKeyPrefix) {
		return nil, fmt.Errorf("gateway_key 必须以 %s 开头", gatewayKeyPrefix)
	}
	if cfg.LLMAPIKey == "" {
		return nil, fmt.Errorf("llm_api_key 不能为空，请在 %s 中配置", configPath)
	}

	logger.Info("网关凭据加载完成",
		zap.String("gateway_key", maskKey(cfg.GatewayKey)),
		zap.String("llm_api_key", maskKey(cfg.LLMAPIKey)),
	)

	return &Manager{
		config: &cfg,
		logger: logger,
	}, nil
}

func (m *Manager) ValidateGatewayKey(key string) bool {
	if !strings.HasPrefix(key, gatewayKeyPrefix) {
		return false
	}
	return key == m.config.GatewayKey
}

func (m *Manager) GetLLMAPIKey() string {
	return m.config.LLMAPIKey
}

func (m *Manager) GatewayKeyID() string {
	return m.config.GatewayKey
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:7] + "..."
}
