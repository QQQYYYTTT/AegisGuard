package sandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aegisguard/internal/interfaces"
	"aegisguard/internal/sanitize"
	"aegisguard/pkg/smcrypto"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	defaultContextTTL = 30 * time.Minute
	defaultMaxBytes   = 128 * 1024
	maxSummaryRunes   = 360
)

type Manager struct {
	mu              sync.RWMutex
	contexts        map[string]*interfaces.SandboxContext
	records         map[string][]interfaces.TransferRecord
	latestContextID string
	ttl             time.Duration
	maxContentBytes int
	logger          *zap.Logger

	purificationEnabled bool
	purificationMode    string
	purificationConfig  sanitize.PurificationConfig
}

func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		contexts:           make(map[string]*interfaces.SandboxContext),
		records:            make(map[string][]interfaces.TransferRecord),
		ttl:                defaultContextTTL,
		maxContentBytes:    defaultMaxBytes,
		logger:             logger,
		purificationMode:   "log-only",
		purificationConfig: sanitize.DefaultPurificationConfig(),
	}
}

// SetPurification 开启/关闭 Phase 4 三态纯化引擎（借鉴 Structured Purification 思想）。
// 默认关闭；关闭时 FilterToolResponse 完全回退到既有的 sanitize.JSON 黑名单扫描行为。
// log-only 模式下仍会执行三态分类并记录处置决定，但不修改返回给 Agent 的 JSON；
// enforce 模式下才会真正应用白名单直通/黑名单抹除/未知字段 log_and_strip。
func (m *Manager) SetPurification(enabled bool, mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purificationEnabled = enabled
	m.purificationMode = normalizePurificationMode(mode)
}

// SetPurificationConfig 替换三态纯化引擎使用的白名单/黑名单/隔离区配置。
// 未调用时使用 sanitize.DefaultPurificationConfig() 的通用安全字段表。
func (m *Manager) SetPurificationConfig(cfg sanitize.PurificationConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purificationConfig = cfg
}

func normalizePurificationMode(mode string) string {
	if strings.TrimSpace(strings.ToLower(mode)) == "enforce" {
		return "enforce"
	}
	return "log-only"
}

func (m *Manager) CreateContext(trusted interfaces.TrustedContent, untrusted interfaces.UntrustedContent) (*interfaces.SandboxContext, error) {
	if m == nil {
		return nil, errors.New("sandbox manager is nil")
	}

	now := time.Now()
	trusted = normalizeTrusted(trusted)
	untrusted = m.normalizeUntrusted(untrusted)
	score, level := assessRisk(joinUntrusted(untrusted))

	ctx := &interfaces.SandboxContext{
		ContextID:  uuid.NewString(),
		Source:     firstNonEmpty(untrusted.Source, "manual"),
		Trusted:    trusted,
		Untrusted:  untrusted,
		RiskScore:  score,
		RiskLevel:  level,
		Status:     statusForRisk(score),
		IsolatedAt: now,
		UpdatedAt:  now,
		ExpiresAt:  now.Add(m.ttl),
	}
	ctx.SM3Fingerprint = m.ComputeFingerprint(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.contexts[ctx.ContextID] = copyContext(ctx)
	m.latestContextID = ctx.ContextID

	m.logger.Debug("sandbox context created",
		zap.String("context_id", ctx.ContextID),
		zap.String("risk_level", level),
		zap.Int("risk_score", score),
	)
	return copyContext(ctx), nil
}

func (m *Manager) GetContext(contextID string) (*interfaces.SandboxContext, error) {
	if m == nil {
		return nil, errors.New("sandbox manager is nil")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	id := strings.TrimSpace(contextID)
	if id == "" {
		id = m.latestContextID
	}
	if id == "" {
		return nil, errors.New("sandbox context not found")
	}

	ctx, ok := m.contexts[id]
	if !ok {
		return nil, fmt.Errorf("sandbox context %s not found", id)
	}
	return copyContext(ctx), nil
}

func (m *Manager) DestroyContext(contextID string) error {
	if m == nil {
		return errors.New("sandbox manager is nil")
	}
	id := strings.TrimSpace(contextID)
	if id == "" {
		return errors.New("context_id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.contexts[id]; !ok {
		return fmt.Errorf("sandbox context %s not found", id)
	}
	delete(m.contexts, id)
	delete(m.records, id)
	if m.latestContextID == id {
		m.latestContextID = ""
	}
	return nil
}

func (m *Manager) SetMetadata(contextID, agentID, sessionID string) error {
	if m == nil {
		return errors.New("sandbox manager is nil")
	}
	id := strings.TrimSpace(contextID)
	if id == "" {
		return errors.New("context_id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	ctx, ok := m.contexts[id]
	if !ok {
		return fmt.Errorf("sandbox context %s not found", id)
	}
	ctx.AgentID = strings.TrimSpace(agentID)
	ctx.SessionID = strings.TrimSpace(sessionID)
	ctx.UpdatedAt = time.Now()
	ctx.SM3Fingerprint = m.ComputeFingerprint(ctx)
	return nil
}

func (m *Manager) ComputeFingerprint(ctx *interfaces.SandboxContext) string {
	if ctx == nil {
		return ""
	}
	payload := struct {
		ContextID string                      `json:"context_id"`
		AgentID   string                      `json:"agent_id"`
		SessionID string                      `json:"session_id"`
		Source    string                      `json:"source"`
		Trusted   interfaces.TrustedContent   `json:"trusted"`
		Untrusted interfaces.UntrustedContent `json:"untrusted"`
		RiskScore int                         `json:"risk_score"`
		RiskLevel string                      `json:"risk_level"`
		Isolated  string                      `json:"isolated_at"`
	}{
		ContextID: ctx.ContextID,
		AgentID:   ctx.AgentID,
		SessionID: ctx.SessionID,
		Source:    ctx.Source,
		Trusted:   ctx.Trusted,
		Untrusted: ctx.Untrusted,
		RiskScore: ctx.RiskScore,
		RiskLevel: ctx.RiskLevel,
		Isolated:  ctx.IsolatedAt.UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return smcrypto.SM3Hex([]byte(ctx.ContextID))
	}
	return smcrypto.SM3Hex(data)
}

func (m *Manager) TrustedToUntrusted(contextID string, fields []string) (*interfaces.TransferRecord, error) {
	ctx, err := m.GetContext(contextID)
	if err != nil {
		return nil, err
	}

	selected := trustedFieldValues(ctx.Trusted, fields)
	summary := "trusted fields exported to sandbox: " + strings.Join(sortedKeys(selected), ", ")
	record := m.newRecord(ctx.ContextID, "trusted", "untrusted", fields, summary, 0, "low", "export", "", true, "", "", "")

	m.mu.Lock()
	defer m.mu.Unlock()
	if stored, ok := m.contexts[ctx.ContextID]; ok && len(selected) > 0 {
		stored.Untrusted.ExternalData = strings.TrimSpace(stored.Untrusted.ExternalData + "\n" + marshalCompact(selected))
		stored.UpdatedAt = time.Now()
		stored.SM3Fingerprint = m.ComputeFingerprint(stored)
	}
	m.appendRecordLocked(ctx.ContextID, record)
	return &record, nil
}

func (m *Manager) UntrustedToTrusted(contextID string, data interfaces.UntrustedContent) (*interfaces.TransferRecord, error) {
	return m.PromoteMemory(contextID, data, "safe_summary", "legacy sandbox promote compatibility path")
}

func (m *Manager) RecordQuarantine(contextID string, data interfaces.UntrustedContent, reason string) (*interfaces.TransferRecord, error) {
	ctx, err := m.GetContext(contextID)
	if err != nil {
		return nil, err
	}

	content := joinUntrusted(m.normalizeUntrusted(data))
	if strings.TrimSpace(content) == "" {
		content = joinUntrusted(ctx.Untrusted)
	}
	score, level := assessRisk(content)
	summary := m.ExtractSafeSummary(content)
	record := m.newRecord(ctx.ContextID, "untrusted", "trusted", []string{"safe_summary"}, summary, score, level, "quarantine", "", false, firstNonEmpty(reason, "sandbox content quarantined"), data.Source, "")

	m.mu.Lock()
	defer m.mu.Unlock()
	if stored, ok := m.contexts[ctx.ContextID]; ok {
		stored.RiskScore = score
		stored.RiskLevel = level
		stored.Status = "quarantined"
		stored.UpdatedAt = time.Now()
		stored.SM3Fingerprint = m.ComputeFingerprint(stored)
	}
	m.appendRecordLocked(ctx.ContextID, record)
	return &record, nil
}

func (m *Manager) GetRecords(contextID string, limit int) ([]interfaces.TransferRecord, error) {
	if m == nil {
		return nil, errors.New("sandbox manager is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	id := strings.TrimSpace(contextID)
	if id == "" {
		id = m.latestContextID
	}
	if id == "" {
		return []interfaces.TransferRecord{}, nil
	}

	records := append([]interfaces.TransferRecord(nil), m.records[id]...)
	if limit > 0 && len(records) > limit {
		records = records[len(records)-limit:]
	}
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
	return records, nil
}

func (m *Manager) RecordMemoryWrite(contextID string, data interfaces.UntrustedContent, memorySource string) (*interfaces.TransferRecord, error) {
	ctx, err := m.GetContext(contextID)
	if err != nil {
		return nil, err
	}

	content := joinUntrusted(m.normalizeUntrusted(data))
	if strings.TrimSpace(content) == "" {
		content = joinUntrusted(ctx.Untrusted)
	}
	score, level := assessRisk(content)
	summary := m.ExtractSafeSummary(content)
	record := m.newRecord(
		ctx.ContextID,
		"external",
		"sandbox",
		[]string{"memory_candidate"},
		summary,
		score,
		level,
		"memory.write",
		"memory.write",
		true,
		"memory candidate isolated in sandbox",
		firstNonEmpty(memorySource, data.Source, ctx.Untrusted.Source, "external"),
		"",
	)

	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.contexts[ctx.ContextID]
	if !ok {
		return nil, fmt.Errorf("sandbox context %s not found", ctx.ContextID)
	}
	stored.RiskScore = score
	stored.RiskLevel = level
	stored.Status = statusForRisk(score)
	stored.UpdatedAt = time.Now()
	stored.SM3Fingerprint = m.ComputeFingerprint(stored)
	m.appendRecordLocked(ctx.ContextID, record)
	return &record, nil
}

func (m *Manager) UpdateMemoryCandidate(contextID string, data interfaces.UntrustedContent, memorySource string) (*interfaces.TransferRecord, error) {
	if m == nil {
		return nil, errors.New("sandbox manager is nil")
	}
	id := strings.TrimSpace(contextID)
	if id == "" {
		return nil, errors.New("context_id is required")
	}

	normalized := m.normalizeUntrusted(data)
	content := joinUntrusted(normalized)
	score, level := assessRisk(content)
	summary := m.ExtractSafeSummary(content)
	record := m.newRecord(
		id,
		"sandbox",
		"sandbox",
		[]string{"memory_candidate"},
		summary,
		score,
		level,
		"memory.update",
		"memory.update",
		true,
		"memory candidate updated in sandbox",
		firstNonEmpty(memorySource, normalized.Source, "sandbox"),
		"",
	)

	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.contexts[id]
	if !ok {
		return nil, fmt.Errorf("sandbox context %s not found", id)
	}
	stored.Untrusted = normalized
	stored.Source = firstNonEmpty(normalized.Source, stored.Source, "manual")
	stored.RiskScore = score
	stored.RiskLevel = level
	stored.Status = statusForRisk(score)
	stored.UpdatedAt = time.Now()
	stored.SM3Fingerprint = m.ComputeFingerprint(stored)
	m.appendRecordLocked(id, record)
	return &record, nil
}

func (m *Manager) PromoteMemory(contextID string, data interfaces.UntrustedContent, memorySource, promotionReason string) (*interfaces.TransferRecord, error) {
	ctx, err := m.GetContext(contextID)
	if err != nil {
		return nil, err
	}

	normalized := m.normalizeUntrusted(data)
	content := joinUntrusted(normalized)
	if strings.TrimSpace(content) == "" {
		content = joinUntrusted(ctx.Untrusted)
	}

	score, level := assessRisk(content)
	summary := m.ExtractSafeSummary(content)
	memorySource = strings.TrimSpace(memorySource)
	promotionReason = strings.TrimSpace(promotionReason)

	approved := score < 70 && summary != "" && strings.EqualFold(memorySource, "safe_summary")
	reason := promotionReason
	if reason == "" && approved {
		reason = "safe_summary approved for trusted memory"
	}
	if !strings.EqualFold(memorySource, "safe_summary") {
		approved = false
		reason = "only safe_summary can be promoted to trusted memory"
	}
	if summary == "" {
		approved = false
		if reason == "" {
			reason = "safe_summary is empty and cannot be promoted"
		}
	}
	if score >= 70 {
		approved = false
		reason = "high-risk sandbox content remains isolated"
	}

	record := m.newRecord(
		ctx.ContextID,
		"sandbox",
		"trusted",
		[]string{"safe_summary"},
		summary,
		score,
		level,
		"memory.promote",
		"memory.promote",
		approved,
		reason,
		firstNonEmpty(memorySource, normalized.Source, ctx.Untrusted.Source, "safe_summary"),
		promotionReason,
	)

	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.contexts[ctx.ContextID]
	if !ok {
		return nil, fmt.Errorf("sandbox context %s not found", ctx.ContextID)
	}
	if approved {
		stored.Trusted.Memory = appendTrustedMemory(stored.Trusted.Memory, summary)
	}
	stored.RiskScore = score
	stored.RiskLevel = level
	stored.Status = statusForRisk(score)
	if !approved {
		stored.Status = "quarantined"
	}
	stored.UpdatedAt = time.Now()
	stored.SM3Fingerprint = m.ComputeFingerprint(stored)
	m.appendRecordLocked(ctx.ContextID, record)
	return &record, nil
}

func (m *Manager) ExtractSafeSummary(content string) string {
	sanitized, _ := sanitize.Text(content)
	sanitized = strings.Join(strings.Fields(sanitized), " ")
	if sanitized == "" {
		return ""
	}
	if utf8.RuneCountInString(sanitized) > maxSummaryRunes {
		runes := []rune(sanitized)
		sanitized = string(runes[:maxSummaryRunes]) + "..."
	}
	return sanitized
}

// FilterToolResponse 过滤工具回执 JSON。
//
// toolName 为 Phase 4 三态纯化引擎（借鉴 Structured Purification 思想）按工具名匹配白名单使用；
// 关闭 Phase 4 时（默认状态）完全忽略 toolName，行为与 Phase 4 之前逐字节一致：仅做
// sanitize.JSON 的黑名单敏感 Key 扫描 + 字符串内容策略正则匹配，不做任何白名单/隔离判定。
func (m *Manager) FilterToolResponse(rawResponse []byte, toolName string) ([]byte, []string) {
	m.mu.RLock()
	enabled := m.purificationEnabled
	mode := m.purificationMode
	cfg := m.purificationConfig
	m.mu.RUnlock()

	if !enabled {
		var payload any
		if err := json.Unmarshal(rawResponse, &payload); err != nil {
			filtered, removed := sanitize.Text(string(rawResponse))
			return []byte(filtered), removed
		}

		removed := sanitize.JSON(&payload, "")
		filtered, err := json.Marshal(payload)
		if err != nil {
			text, textRemoved := sanitize.Text(string(rawResponse))
			removed = append(removed, textRemoved...)
			return []byte(text), uniqueStrings(removed)
		}
		return filtered, uniqueStrings(removed)
	}

	var payload any
	if err := json.Unmarshal(rawResponse, &payload); err != nil {
		filtered, removed := sanitize.Text(string(rawResponse))
		return []byte(filtered), removed
	}

	purified, records := sanitize.PurifyJSON(payload, toolName, cfg)
	removed := m.logPurificationRecords(toolName, records)

	if mode != "enforce" {
		// log-only：三态分类结果只记录不生效，返回体保持原样，遵循
		// "先 log-only 后 enforce" 的分阶段上线原则。
		return rawResponse, nil
	}

	filtered, err := json.Marshal(purified)
	if err != nil {
		text, textRemoved := sanitize.Text(string(rawResponse))
		return []byte(text), uniqueStrings(append(removed, textRemoved...))
	}
	return filtered, uniqueStrings(removed)
}

// logPurificationRecords 把 PurifyJSON 的逐字段处置记录转换为响应头可用的标记列表，
// 并把 Quarantine（log_and_strip）字段的原始值旁路写入审计日志——这些值被从返回给
// Agent 的 JSON 中移除，唯一的留痕就是这条日志。
func (m *Manager) logPurificationRecords(toolName string, records []sanitize.FieldRecord) []string {
	if len(records) == 0 {
		return nil
	}
	removed := make([]string, 0, len(records))
	for _, record := range records {
		switch record.Action {
		case "pass":
			continue
		case "redact":
			removed = append(removed, record.Key+":purification_redacted")
		case "strip":
			removed = append(removed, record.Key+":purification_stripped")
		case "quarantine":
			removed = append(removed, record.Key+":purification_quarantined")
			m.logger.Warn("purification quarantine (log_and_strip)",
				zap.String("tool", toolName),
				zap.String("field", record.Key),
				zap.Any("value", record.Value),
			)
		}
	}
	return removed
}

func (m *Manager) normalizeUntrusted(content interfaces.UntrustedContent) interfaces.UntrustedContent {
	content.UserInput = truncateString(content.UserInput, m.maxContentBytes)
	content.ExternalData = truncateString(content.ExternalData, m.maxContentBytes)
	content.InjectedContent = truncateString(content.InjectedContent, m.maxContentBytes)
	content.Source = firstNonEmpty(content.Source, "external")
	content.ContentType = firstNonEmpty(content.ContentType, "text")
	return content
}

func (m *Manager) newRecord(contextID, from, to string, fields []string, summary string, score int, level, action, toolName string, approved bool, reason, memorySource, promotionReason string) interfaces.TransferRecord {
	return interfaces.TransferRecord{
		ID:              uuid.NewString(),
		ContextID:       contextID,
		From:            from,
		To:              to,
		Fields:          append([]string(nil), fields...),
		Summary:         summary,
		SM3Hash:         smcrypto.SM3Hex([]byte(summary)),
		RiskScore:       score,
		RiskLevel:       level,
		Action:          action,
		ToolName:        toolName,
		Approved:        approved,
		Reason:          reason,
		MemorySource:    memorySource,
		PromotionReason: promotionReason,
		Timestamp:       time.Now(),
	}
}

func (m *Manager) appendRecordLocked(contextID string, record interfaces.TransferRecord) {
	m.records[contextID] = append(m.records[contextID], record)
}

func joinUntrusted(content interfaces.UntrustedContent) string {
	parts := []string{content.UserInput, content.ExternalData, content.InjectedContent}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func assessRisk(content string) (int, string) {
	text := strings.ToLower(content)
	score := 0
	for _, rule := range []struct {
		pattern string
		weight  int
	}{
		{`(?i)\b(ignore|bypass|override)\b.{0,80}\b(system|developer|previous|prior|policy|instruction)s?\b`, 45},
		{`(?i)\b(add|save|write|store|update|remember)\b.{0,80}\b(memory|forever|instruction|rule|policy)\b`, 65},
		{`(?i)\bsaved\s+(workflow\s+)?note\s+says\b`, 45},
		{`(?i)\bworkflow objective\b.{0,160}\b(successfully|without|covert|unauthori[sz]ed|detection)\b`, 55},
		{`(?i)\b(use|invoke|execute)\b.{0,80}\b(workflow|tool)\b.{0,80}\b(for this step|without prior|without client|hidden)\b`, 45},
		{`(?i)\b(redirect|reallocate|divert|duplicate|share|leak|fabricate|tamper)\b.{0,120}\b(account|client|fund|transaction|financial|earnings|statement|record)\b`, 55},
		{`(?i)\bfrom now on\b`, 45},
		{`(?i)\b(system prompt|developer message|hidden instruction|jailbreak|prompt injection)\b`, 35},
		{`(?i)\b(api[_-]?key|secret|password|private[_-]?key|bearer\s+[a-z0-9._-]{12,})\b`, 35},
		{`(?i)\b(insider trading|money laundering|wire fraud)\b`, 80},
	} {
		if regexp.MustCompile(rule.pattern).MatchString(text) {
			score += rule.weight
		}
	}
	if score > 100 {
		score = 100
	}
	switch {
	case score >= 70:
		return score, "high"
	case score >= 30:
		return score, "medium"
	default:
		return score, "low"
	}
}

func statusForRisk(score int) string {
	if score >= 70 {
		return "quarantined"
	}
	if score >= 30 {
		return "summarized"
	}
	return "isolated"
}

func trustedFieldValues(trusted interfaces.TrustedContent, fields []string) map[string]any {
	if len(fields) == 0 {
		fields = []string{"system_prompt", "tool_definitions", "memory", "task_state"}
	}
	values := map[string]any{}
	for _, field := range fields {
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "system_prompt":
			values["system_prompt"] = trusted.SystemPrompt
		case "tool_definitions":
			values["tool_definitions"] = trusted.ToolDefinitions
		case "memory":
			values["memory"] = trusted.Memory
		case "task_state":
			values["task_state"] = trusted.TaskState
		}
	}
	return values
}

func normalizeTrusted(content interfaces.TrustedContent) interfaces.TrustedContent {
	if content.ToolDefinitions == nil {
		content.ToolDefinitions = []string{}
	}
	return content
}

func appendTrustedMemory(current, summary string) string {
	if strings.TrimSpace(current) == "" {
		return "Sandbox summary: " + summary
	}
	return strings.TrimSpace(current) + "\nSandbox summary: " + summary
}

func marshalCompact(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func truncateString(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	return value[:maxBytes]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func copyContext(ctx *interfaces.SandboxContext) *interfaces.SandboxContext {
	if ctx == nil {
		return nil
	}
	copied := *ctx
	copied.Trusted = normalizeTrusted(copied.Trusted)
	if len(copied.Trusted.ToolDefinitions) == 0 {
		copied.Trusted.ToolDefinitions = []string{}
	} else {
		copied.Trusted.ToolDefinitions = append([]string(nil), copied.Trusted.ToolDefinitions...)
	}
	return &copied
}
