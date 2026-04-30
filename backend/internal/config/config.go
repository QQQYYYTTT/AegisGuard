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

	// 网关凭据配置
	GatewayConfigPath string // 网关凭据配置文件路径 (gateway.yaml)
	LogLevel          string // 日志级别: debug/info/warn/error
	LogEncoding       string // 日志编码: console/production
	PolicyMode        string // 策略模式: loose/balanced/strict
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

	// 审计日志路径，支持环境变量覆盖
	auditFile := getEnv("AEGIS_AUDIT_FILE", filepath.Join(rootDir, "backend", "data", "audit-store.jsonl"))

	// 网关核心配置
	gatewayConfigPath := getEnv("AEGIS_GATEWAY_CONFIG", filepath.Join(rootDir, "config", "gateway.yaml"))
	logLevel := getEnv("AEGIS_LOG_LEVEL", "debug")
	logEncoding := getEnv("AEGIS_LOG_ENCODING", "console")
	policyMode := getEnv("AEGIS_POLICY_MODE", "balanced")

	return Config{
		RootDir:          rootDir,
		FrontendDir:      filepath.Join(rootDir, "frontend"),
		AuditFile:        auditFile,
		Port:             port,
		LangGraphChatURL: langGraphChatURL,

		// 网关核心配置
		GatewayConfigPath: gatewayConfigPath,
		LogLevel:          logLevel,
		LogEncoding:       logEncoding,
		PolicyMode:        policyMode,
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
