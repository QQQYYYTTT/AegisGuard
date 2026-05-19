package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	RootDir           string
	BackendDir        string
	FrontendDir       string
	AuditFile         string
	UserDBPath        string
	Port              string
	LangGraphChatURL  string
	SigningPrivateKey string
	UserTokenSecret   string
	TokenMode         string

	GatewayConfigPath string
	LogLevel          string
	LogEncoding       string
	PolicyMode        string
	DevMode           bool
}

func resolveRootDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	dir := cwd
	for i := 0; i < 4; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return cwd
}

func Load() Config {
	rootDir := resolveRootDir()
	backendDir := filepath.Join(rootDir, "backend")

	port := getEnv("PORT", "8090")
	langGraphChatURL := getEnv("LANGGRAPH_CHAT_URL", "http://127.0.0.1:8765")
	auditFile := getEnv("AEGIS_AUDIT_FILE", filepath.Join(backendDir, "data", "audit-store.jsonl"))
	userDBPath := getEnv("AEGIS_USER_DB_PATH", filepath.Join(backendDir, "data", "aegisguard-users.db"))
	gatewayConfigPath := getEnv("AEGIS_GATEWAY_CONFIG", filepath.Join(backendDir, "config", "gateway.yaml"))
	logLevel := getEnv("AEGIS_LOG_LEVEL", "debug")
	logEncoding := getEnv("AEGIS_LOG_ENCODING", "console")
	policyMode := getEnv("AEGIS_POLICY_MODE", "balanced")
	tokenMode := normalizeTokenMode(getEnv("AEGIS_TOKEN_MODE", "strict"))
	devMode := strings.EqualFold(getEnv("AEGIS_DEV_MODE", "true"), "true")

	return Config{
		RootDir:           rootDir,
		BackendDir:        backendDir,
		FrontendDir:       filepath.Join(rootDir, "frontend"),
		AuditFile:         auditFile,
		UserDBPath:        userDBPath,
		Port:              port,
		LangGraphChatURL:  langGraphChatURL,
		SigningPrivateKey: getEnv("AEGIS_SIGNING_PRIVATE_KEY", ""),
		UserTokenSecret:   getEnv("AEGIS_USER_TOKEN_SECRET", "aegisguard-dev-user-secret"),
		TokenMode:         tokenMode,
		GatewayConfigPath: gatewayConfigPath,
		LogLevel:          logLevel,
		LogEncoding:       logEncoding,
		PolicyMode:        policyMode,
		DevMode:           devMode,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func normalizeTokenMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "strict", "compat", "warn":
		return strings.TrimSpace(strings.ToLower(mode))
	default:
		return "strict"
	}
}
