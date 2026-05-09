package config

import (
	"os"
	"path/filepath"
)

// Config 应用配置结构体
type Config struct {
	RootDir          string // 项目根目录（AegisGuard/）
	BackendDir       string // 后端目录（AegisGuard/backend/）
	FrontendDir      string // 前端目录
	AuditFile        string // 审计日志文件路径
	Port             string // 服务端口
	LangGraphChatURL string // LangGraph 聊天服务地址
	SigningPrivateKey string // RequireToken SM2 签名私钥（十六进制）

	// 网关凭据配置
	GatewayConfigPath string // 网关凭据配置文件路径 (gateway.yaml)
	LogLevel          string // 日志级别: debug/info/warn/error
	LogEncoding       string // 日志编码: console/production
	PolicyMode        string // 策略模式: loose/balanced/strict

	// 运行模式
	DevMode bool // 开发模式：启用实验结果 API、调试端点等
}

// resolveRootDir 从 CWD 向上查找 go.mod 来确定项目根目录
// 回退策略：CWD → CWD/.. → "."
func resolveRootDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	// 从 CWD 开始，逐级向上查找 go.mod（最多 4 层）
	dir := cwd
	for i := 0; i < 4; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// 找到 go.mod，其父目录即为项目根目录
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// 回退：假设 CWD 就是项目根目录
	return cwd
}

// Load 加载配置
// 优先级: 环境变量 > 默认值
func Load() Config {
	rootDir := resolveRootDir()
	backendDir := filepath.Join(rootDir, "backend")

	// 基础配置
	port := getEnv("PORT", "8090")
	langGraphChatURL := getEnv("LANGGRAPH_CHAT_URL", "http://127.0.0.1:8765")

	// 审计日志路径，支持环境变量覆盖
	auditFile := getEnv("AEGIS_AUDIT_FILE", filepath.Join(backendDir, "data", "audit-store.jsonl"))

	// 网关核心配置
	gatewayConfigPath := getEnv("AEGIS_GATEWAY_CONFIG", filepath.Join(backendDir, "config", "gateway.yaml"))
	logLevel := getEnv("AEGIS_LOG_LEVEL", "debug")
	logEncoding := getEnv("AEGIS_LOG_ENCODING", "console")
	policyMode := getEnv("AEGIS_POLICY_MODE", "balanced")
	devMode := getEnv("AEGIS_DEV_MODE", "false") == "true"

	return Config{
		RootDir:          rootDir,
		BackendDir:       backendDir,
		FrontendDir:      filepath.Join(rootDir, "frontend"),
		AuditFile:        auditFile,
		Port:             port,
		LangGraphChatURL: langGraphChatURL,
		SigningPrivateKey: getEnv("AEGIS_SIGNING_PRIVATE_KEY", ""),

		// 网关核心配置
		GatewayConfigPath: gatewayConfigPath,
		LogLevel:          logLevel,
		LogEncoding:       logEncoding,
		PolicyMode:        policyMode,
		DevMode:           devMode,
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
