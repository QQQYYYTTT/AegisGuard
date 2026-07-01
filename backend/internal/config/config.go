package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/subosito/gotenv"
)

type Config struct {
	RootDir                  string
	BackendDir               string
	FrontendDir              string
	AuditFile                string
	UserDBPath               string
	Port                     string
	LangGraphChatURL         string
	AssistantAPIBase         string
	AssistantAPIKey          string
	AssistantModel           string
	AssistantReasoningEffort string
	AssistantThinkingType    string
	SigningPrivateKey        string
	UserTokenSecret          string
	TokenMode                string

	GatewayConfigPath string
	LogLevel          string
	LogEncoding       string
	PolicyMode        string
	DevMode           bool

	DynamicRuleRoutingEnabled bool

	TDGEnabled   bool
	TDGMode      string
	TDGMaxNodes  int
	TDGMaxRepeat int
	TDGTTL       time.Duration

	ProvenanceEnabled bool
	ProvenanceMode    string

	PurificationEnabled bool
	PurificationMode    string

	AuditStorageMode string
	AuditDBPath      string
	SQLiteWALMode    bool

	ThreatMapTarget      string
	ThreatMapTargetCoord [2]float64
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
	_ = gotenv.Load(filepath.Join(rootDir, ".env"))

	backendDir := filepath.Join(rootDir, "backend")

	port := getEnv("PORT", "8090")
	langGraphChatURL := getEnv("LANGGRAPH_CHAT_URL", "http://127.0.0.1:8765")
	assistantAPIBase := getEnv("AEGIS_ASSISTANT_API_BASE", "https://api.siliconflow.cn/v1")
	assistantAPIKey := getEnv("AEGIS_ASSISTANT_API_KEY", "")
	assistantModel := getEnv("AEGIS_ASSISTANT_MODEL", "Qwen/Qwen3.5-9B")
	assistantReasoningEffort := getEnv("AEGIS_ASSISTANT_REASONING_EFFORT", "")
	assistantThinkingType := getEnv("AEGIS_ASSISTANT_THINKING_TYPE", "")
	auditFile := getEnv("AEGIS_AUDIT_FILE", filepath.Join(backendDir, "data", "audit-store.jsonl"))
	userDBPath := getEnv("AEGIS_USER_DB_PATH", filepath.Join(backendDir, "data", "aegisguard-users.db"))
	gatewayConfigPath := getEnv("AEGIS_GATEWAY_CONFIG", filepath.Join(backendDir, "config", "gateway.yaml"))
	logLevel := getEnv("AEGIS_LOG_LEVEL", "debug")
	logEncoding := getEnv("AEGIS_LOG_ENCODING", "console")
	policyMode := getEnv("AEGIS_POLICY_MODE", "balanced")
	tokenMode := normalizeTokenMode(getEnv("AEGIS_TOKEN_MODE", "strict"))
	devMode := strings.EqualFold(getEnv("AEGIS_DEV_MODE", "true"), "true")
	// Phase 1（动态规则路由）默认关闭，需显式开启；开关关闭时行为与开发前完全一致。
	dynamicRuleRoutingEnabled := strings.EqualFold(getEnv("AEGIS_DYNAMIC_RULE_ROUTING", "false"), "true")

	// Phase 2（TDG 拓扑校验）默认关闭，且默认 log-only（不阻断，仅记录违规），
	// 遵循"先 log-only 后 enforce"的分阶段上线原则。
	tdgEnabled := strings.EqualFold(getEnv("AEGIS_TDG_ENABLED", "false"), "true")
	tdgMode := getEnv("AEGIS_TDG_MODE", "log-only")
	tdgMaxNodes := getEnvInt("AEGIS_TDG_MAX_NODES", 50)
	tdgMaxRepeat := getEnvInt("AEGIS_TDG_MAX_REPEAT", 5)
	tdgTTL := getEnvDuration("AEGIS_TDG_TTL", 30*time.Minute)

	// Phase 3（参数溯源校验）默认关闭，且默认 log-only，同样遵循"先 log-only 后 enforce"原则。
	provenanceEnabled := strings.EqualFold(getEnv("AEGIS_PROVENANCE_ENABLED", "false"), "true")
	provenanceMode := getEnv("AEGIS_PROVENANCE_MODE", "log-only")

	// Phase 4（三态纯化引擎）默认关闭，且默认 log-only，同样遵循"先 log-only 后 enforce"原则。
	// 关闭时 sandbox.Manager.FilterToolResponse 完全回退到既有的 sanitize.JSON 黑名单扫描。
	purificationEnabled := strings.EqualFold(getEnv("AEGIS_PURIFICATION_ENABLED", "false"), "true")
	purificationMode := getEnv("AEGIS_PURIFICATION_MODE", "log-only")

	auditStorageMode := normalizeAuditStorageMode(getEnv("AEGIS_AUDIT_STORAGE_MODE", "sqlite"))
	auditDBPath := getEnv("AEGIS_AUDIT_DB_PATH", filepath.Join(backendDir, "data", "audit-store.db"))
	sqliteWALMode := strings.EqualFold(getEnv("AEGIS_SQLITE_WAL_MODE", "true"), "true")

	threatMapTarget, threatMapCoord := parseThreatMapTarget(getEnv("AEGIS_THREAT_MAP_TARGET", ""))

	return Config{
		RootDir:                   rootDir,
		BackendDir:                backendDir,
		FrontendDir:               filepath.Join(rootDir, "frontend"),
		AuditFile:                 auditFile,
		UserDBPath:                userDBPath,
		Port:                      port,
		LangGraphChatURL:          langGraphChatURL,
		AssistantAPIBase:          strings.TrimRight(assistantAPIBase, "/"),
		AssistantAPIKey:           assistantAPIKey,
		AssistantModel:            assistantModel,
		AssistantReasoningEffort:  assistantReasoningEffort,
		AssistantThinkingType:     assistantThinkingType,
		SigningPrivateKey:         getEnv("AEGIS_SIGNING_PRIVATE_KEY", ""),
		UserTokenSecret:           getEnv("AEGIS_USER_TOKEN_SECRET", "aegisguard-dev-user-secret"),
		TokenMode:                 tokenMode,
		GatewayConfigPath:         gatewayConfigPath,
		LogLevel:                  logLevel,
		LogEncoding:               logEncoding,
		PolicyMode:                policyMode,
		DevMode:                   devMode,
		DynamicRuleRoutingEnabled: dynamicRuleRoutingEnabled,
		TDGEnabled:                tdgEnabled,
		TDGMode:                   tdgMode,
		TDGMaxNodes:               tdgMaxNodes,
		TDGMaxRepeat:              tdgMaxRepeat,
		TDGTTL:                    tdgTTL,
		ProvenanceEnabled:         provenanceEnabled,
		ProvenanceMode:            provenanceMode,
		PurificationEnabled:       purificationEnabled,
		PurificationMode:          purificationMode,
		AuditStorageMode:          auditStorageMode,
		AuditDBPath:               auditDBPath,
		SQLiteWALMode:             sqliteWALMode,
		ThreatMapTarget:           threatMapTarget,
		ThreatMapTargetCoord:      threatMapCoord,
	}
}

func parseThreatMapTarget(raw string) (string, [2]float64) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		// 空值表示让程序根据服务器网卡 IP 自动推断
		return "", [2]float64{0, 0}
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 3 {
		return "", [2]float64{0, 0}
	}
	lon, err1 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	lat, err2 := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if err1 != nil || err2 != nil || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
		return "", [2]float64{0, 0}
	}
	return strings.TrimSpace(parts[0]), [2]float64{lon, lat}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return n
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return defaultValue
	}
	return d
}

func normalizeTokenMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "strict", "compat", "warn":
		return strings.TrimSpace(strings.ToLower(mode))
	default:
		return "strict"
	}
}

func normalizeAuditStorageMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "sqlite", "jsonl":
		return strings.TrimSpace(strings.ToLower(mode))
	default:
		return "sqlite"
	}
}
