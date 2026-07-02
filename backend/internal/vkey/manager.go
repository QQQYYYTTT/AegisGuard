package vkey

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type gatewayConfig struct {
	GatewayKey string `yaml:"gateway_key" mapstructure:"gateway_key"`
	TargetURL  string `yaml:"target_url" mapstructure:"target_url"`
	LLMAPIKey  string `yaml:"llm_api_key" mapstructure:"llm_api_key"`
}

const gatewayKeyPrefix = "agk-"
const gatewayKeyHeader = "X-Gateway-Key"

func ExtractGatewayCredential(headers http.Header) string {
	if headers == nil {
		return ""
	}
	if key := normalizeGatewayKey(headers.Get(gatewayKeyHeader)); key != "" {
		return key
	}
	return ExtractGatewayKey(headers.Get("Authorization"))
}

func ExtractGatewayKey(authHeader string) string {
	return normalizeGatewayKey(extractBearerToken(authHeader))
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
	if cfg.TargetURL == "" {
		return nil, fmt.Errorf("target_url 不能为空，请在 %s 中配置", configPath)
	}

	// 环境变量优先级高于配置文件，方便生产环境注入
	if envURL := os.Getenv("AEGIS_TARGET_URL"); envURL != "" {
		cfg.TargetURL = envURL
	}
	if envKey := os.Getenv("AEGIS_LLM_API_KEY"); envKey != "" {
		cfg.LLMAPIKey = envKey
	}

	logger.Info("网关凭据加载完成",
		zap.String("gateway_key", maskKey(cfg.GatewayKey)),
		zap.String("target_url", cfg.TargetURL),
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

func (m *Manager) GetTargetURL() string {
	return m.config.TargetURL
}

func (m *Manager) GatewayKeyID() string {
	return m.config.GatewayKey
}

func maskKey(key string) string {
	if strings.TrimSpace(key) == "" {
		return ""
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:7] + "..."
}

func extractBearerToken(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func normalizeGatewayKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, gatewayKeyPrefix) {
		return raw
	}
	return ""
}
