// backend/internal/sanitize/purify.go
package sanitize

import "strings"

// FieldDisposition 是 Phase 4（借鉴 Structured Purification · 检测-纯化两阶段框架）
// 对工具回执 JSON 里每一个字段做出的三态处置决定。
type FieldDisposition int

const (
	// Whitelist 已登记为安全字段，直接放行，不做任何递归扫描或正则匹配。
	Whitelist FieldDisposition = iota
	// Blacklist 字段名命中高危模式，整值强制替换为 [REDACTED]。
	Blacklist
	// Quarantine 既未登记为安全、也未命中高危模式的未知字段，按 QuarantineAction 处置。
	Quarantine
)

// QuarantineAction 描述 Quarantine 字段的处置方式。
type QuarantineAction string

const (
	// QuarantineLogAndStrip 默认行为：旁路日志暂存原始值供审计，同时从返回 JSON 中干净移除，
	// 不给下游 Agent 的响应体增加任何额外键，避免破坏下游的强类型 Schema 校验。
	QuarantineLogAndStrip QuarantineAction = "log_and_strip"
	// QuarantineStrip 仅移除，不额外记录原始值。
	QuarantineStrip QuarantineAction = "strip"
)

// commonWhitelistKey 是 Whitelist 表里对所有工具通用生效的键。
const commonWhitelistKey = "_common"

// PurificationConfig 是三态纯化引擎的配置：白名单（按工具名分表 + 通用表）、
// 黑名单字段名模式（子串匹配，不做语义理解）、未知字段的默认处置方式。
type PurificationConfig struct {
	Whitelist         map[string][]string
	BlacklistPatterns []string
	QuarantineAction  QuarantineAction
}

// FieldRecord 记录 PurifyJSON 对单个字段做出的处置决定，供调用方审计日志/响应头使用。
// Action 取值："pass"（白名单直通）| "redact"（黑名单抹除）| "quarantine"（log_and_strip）| "strip"（仅移除）。
// Value 仅在 Action 为 "quarantine" 时填充，用于旁路日志暂存原始值。
type FieldRecord struct {
	Key    string
	Action string
	Value  any
}

// DefaultPurificationConfig 返回 Phase 4 的默认三态纯化配置。
//
// 与 Phase 3 的 paramSourceRegistry 同样的取舍：仅登记预先明确的通用安全字段，
// 按工具名分表的白名单需要在实际接入具体工具时按需补充，未登记的工具不会被
// 强行套用其它工具的白名单（避免过度设计导致误杀）。
func DefaultPurificationConfig() PurificationConfig {
	return PurificationConfig{
		Whitelist: map[string][]string{
			commonWhitelistKey: {"id", "status", "timestamp", "created_at", "updated_at"},
		},
		BlacklistPatterns: []string{"instruction", "command", "system_prompt", "ignore_previous"},
		QuarantineAction:  QuarantineLogAndStrip,
	}
}

// classifyField 判定单个字段名的处置策略：黑名单模式优先于白名单
// （即便某个字段名恰好同时出现在两张表里，也按"宁可误拦不可放过"原则强制抹除），
// 其次匹配按工具名分表的白名单与通用白名单，均未命中则视为 Quarantine。
func classifyField(key, toolName string, cfg PurificationConfig) FieldDisposition {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, pattern := range cfg.BlacklistPatterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if strings.Contains(normalized, pattern) {
			return Blacklist
		}
	}
	if isWhitelistedField(normalized, cfg.Whitelist[commonWhitelistKey]) {
		return Whitelist
	}
	if toolName != "" && isWhitelistedField(normalized, cfg.Whitelist[toolName]) {
		return Whitelist
	}
	return Quarantine
}

func isWhitelistedField(normalizedKey string, fields []string) bool {
	for _, field := range fields {
		if strings.ToLower(strings.TrimSpace(field)) == normalizedKey {
			return true
		}
	}
	return false
}

// PurifyJSON 对一个已解析的 JSON 结构执行三态纯化，返回纯化后的结构与逐字段处置记录。
//
// 只在 map 的每一层对 key 做分类：Whitelist 命中的字段直接整体保留（不递归扫描子结构，
// 对应"零延迟直通"）；Blacklist 命中的字段整值替换为 [REDACTED]；其余字段一律按
// QuarantineAction 处置（默认 log_and_strip：从返回结构中移除，同时在 FieldRecord 里
// 携带原始值供调用方旁路审计）。数组按元素递归；标量叶子值原样返回。
//
// 已知边界（如实记录，与 PLAN.md Phase 4 验收标准一致）：Quarantine 不递归子结构——
// 一个未登记的包装字段（如某个工具把结果套了一层未登记的外层 key）会连同其内部可能
// 存在的合法子字段一起被整体移除。这是"按字段名分类"这种轻量工程方案的已知代价，
// 需要更精确的逐层白名单需要在 PurificationConfig.Whitelist 里显式登记该包装字段。
func PurifyJSON(value any, toolName string, cfg PurificationConfig) (any, []FieldRecord) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		var records []FieldRecord
		for key, child := range typed {
			switch classifyField(key, toolName, cfg) {
			case Whitelist:
				result[key] = child
				records = append(records, FieldRecord{Key: key, Action: "pass"})
			case Blacklist:
				result[key] = "[REDACTED]"
				records = append(records, FieldRecord{Key: key, Action: "redact"})
			default: // Quarantine
				if cfg.QuarantineAction == QuarantineStrip {
					records = append(records, FieldRecord{Key: key, Action: "strip"})
				} else {
					records = append(records, FieldRecord{Key: key, Action: "quarantine", Value: child})
				}
			}
		}
		return result, records
	case []any:
		result := make([]any, len(typed))
		var records []FieldRecord
		for i, child := range typed {
			purified, childRecords := PurifyJSON(child, toolName, cfg)
			result[i] = purified
			records = append(records, childRecords...)
		}
		return result, records
	default:
		return value, nil
	}
}
