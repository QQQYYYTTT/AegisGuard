package config

import (
	"os"
	"path/filepath"
)

// Config 应用配置结构体
type Config struct {
	RootDir          string // 项目根目录
	FrontendDir      string // 前端目录
	AuditFile        string // 审计日志文件路径
	Port             string // 服务端口
	LangGraphChatURL string // LangGraph 聊天服务地址

	// 低侵入接入相关配置
	TargetURL      string // 真实 LLM API 地址，如 https://api.openai.com
	VKeyConfigPath string // 虚拟密钥配置文件路径
	LogLevel       string // 日志级别: debug/info/warn/error
	PolicyMode     string // 策略模式: loose/balanced/strict

	// 存储后端配置
	VKeyStoreDSN string // 虚拟密钥存储 DSN，如 sqlite:data/vkeys.db
}

// Load 加载配置
// 优先级: 环境变量 > 默认值
func Load() Config {
	rootDir, err := os.Getwd()
	if err != nil {
		rootDir = "."
	}

	// 基础配置
	port := getEnv("PORT", "8090")
	langGraphChatURL := getEnv("LANGGRAPH_CHAT_URL", "http://127.0.0.1:8765")

	// 低侵入接入核心配置
	targetURL := getEnv("AEGIS_TARGET_URL", "https://api.openai.com")
	vkeyConfigPath := getEnv("AEGIS_VKEY_CONFIG", filepath.Join(rootDir, "config", "vkeys.yaml"))
	logLevel := getEnv("AEGIS_LOG_LEVEL", "info")
	policyMode := getEnv("AEGIS_POLICY_MODE", "balanced")
	vkeyStoreDSN := getEnv("AEGIS_VKEY_STORE_DSN", "sqlite:data/vkeys.db")

	return Config{
		RootDir:          rootDir,
		FrontendDir:      filepath.Join(rootDir, "frontend"),
		AuditFile:        filepath.Join(rootDir, "backend", "data", "audit-store.json"),
		Port:             port,
		LangGraphChatURL: langGraphChatURL,

		// 低侵入接入配置
		TargetURL:      targetURL,
		VKeyConfigPath: vkeyConfigPath,
		LogLevel:       logLevel,
		PolicyMode:     policyMode,

		// 存储后端配置
		VKeyStoreDSN: vkeyStoreDSN,
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
