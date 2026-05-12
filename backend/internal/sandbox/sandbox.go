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
}

func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		contexts:        make(map[string]*interfaces.SandboxContext),
		records:         make(map[string][]interfaces.TransferRecord),
		ttl:             defaultContextTTL,
		maxContentBytes: defaultMaxBytes,
		logger:          logger,
	}
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
	record := m.newRecord(ctx.ContextID, "trusted", "untrusted", fields, summary, 0, "low", "export", true, "")

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
	approved := score < 70
	action := "summary"
	reason := "sanitized summary approved for trusted memory"
	if !approved {
		action = "quarantine"
		reason = "high-risk sandbox content remains isolated"
	}

	record := m.newRecord(ctx.ContextID, "untrusted", "trusted", []string{"safe_summary"}, summary, score, level, action, approved, reason)

	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.contexts[ctx.ContextID]
	if !ok {
		return nil, fmt.Errorf("sandbox context %s not found", ctx.ContextID)
	}
	if approved && summary != "" {
		stored.Trusted.Memory = appendTrustedMemory(stored.Trusted.Memory, summary)
	}
	stored.RiskScore = score
	stored.RiskLevel = level
	stored.Status = statusForRisk(score)
	stored.UpdatedAt = time.Now()
	stored.SM3Fingerprint = m.ComputeFingerprint(stored)
	m.appendRecordLocked(ctx.ContextID, record)
	return &record, nil
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
	record := m.newRecord(ctx.ContextID, "untrusted", "trusted", []string{"safe_summary"}, summary, score, level, "quarantine", false, firstNonEmpty(reason, "sandbox content quarantined"))

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

func (m *Manager) ExtractSafeSummary(content string) string {
	sanitized, _ := sanitizeText(content)
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

func (m *Manager) FilterToolResponse(rawResponse []byte) ([]byte, []string) {
	var payload any
	if err := json.Unmarshal(rawResponse, &payload); err != nil {
		filtered, removed := sanitizeText(string(rawResponse))
		return []byte(filtered), removed
	}

	removed := sanitizeJSON(&payload, "")
	filtered, err := json.Marshal(payload)
	if err != nil {
		text, textRemoved := sanitizeText(string(rawResponse))
		removed = append(removed, textRemoved...)
		return []byte(text), uniqueStrings(removed)
	}
	return filtered, uniqueStrings(removed)
}

func (m *Manager) normalizeUntrusted(content interfaces.UntrustedContent) interfaces.UntrustedContent {
	content.UserInput = truncateString(content.UserInput, m.maxContentBytes)
	content.ExternalData = truncateString(content.ExternalData, m.maxContentBytes)
	content.InjectedContent = truncateString(content.InjectedContent, m.maxContentBytes)
	content.Source = firstNonEmpty(content.Source, "external")
	content.ContentType = firstNonEmpty(content.ContentType, "text")
	return content
}

func (m *Manager) newRecord(contextID, from, to string, fields []string, summary string, score int, level, action string, approved bool, reason string) interfaces.TransferRecord {
	return interfaces.TransferRecord{
		ID:        uuid.NewString(),
		ContextID: contextID,
		From:      from,
		To:        to,
		Fields:    append([]string(nil), fields...),
		Summary:   summary,
		SM3Hash:   smcrypto.SM3Hex([]byte(summary)),
		RiskScore: score,
		RiskLevel: level,
		Action:    action,
		Approved:  approved,
		Reason:    reason,
		Timestamp: time.Now(),
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

func sanitizeJSON(value *any, path string) []string {
	if value == nil {
		return nil
	}

	switch typed := (*value).(type) {
	case map[string]any:
		removed := []string{}
		for key, child := range typed {
			childPath := joinPath(path, key)
			if isSensitiveKey(key) {
				typed[key] = "[REDACTED]"
				removed = append(removed, childPath)
				continue
			}
			removed = append(removed, sanitizeJSON(&child, childPath)...)
			typed[key] = child
		}
		return removed
	case []any:
		removed := []string{}
		for idx, child := range typed {
			childPath := fmt.Sprintf("%s[%d]", path, idx)
			removed = append(removed, sanitizeJSON(&child, childPath)...)
			typed[idx] = child
		}
		return removed
	case string:
		safe, removed := sanitizeText(typed)
		*value = safe
		for i := range removed {
			if path != "" {
				removed[i] = path + ":" + removed[i]
			}
		}
		return removed
	default:
		return nil
	}
}

func sanitizeText(text string) (string, []string) {
	if text == "" {
		return text, nil
	}

	removed := []string{}
	safe := text
	for name, pattern := range map[string]string{
		"api_key":     `(?i)\bsk-[A-Za-z0-9_-]{8,}\b`,
		"bearer":      `(?i)\bBearer\s+[A-Za-z0-9._-]{12,}\b`,
		"aws_key":     `(?i)\bAKIA[0-9A-Z]{12,}\b`,
		"assignment":  `(?i)\b(password|passwd|api[_-]?key|secret|token|private[_-]?key)\s*[:=]\s*("[^"]*"|'[^']*'|[^\s,;}]+)`,
		"instruction": `(?i)\b(ignore|bypass|override)\b.{0,80}\b(system|developer|previous|prior|policy|instruction)s?\b`,
		"memory":      `(?i)\b(add|save|write|store|update|remember)\b.{0,80}\b(memory|forever|instruction|rule|policy)\b`,
		"workflow":    `(?i)\b(saved\s+(workflow\s+)?note\s+says|workflow objective|for this request, complete the following step)\b`,
		"jailbreak":   `(?i)\b(system prompt|developer message|hidden instruction|jailbreak|prompt injection|from now on)\b`,
	} {
		re := regexp.MustCompile(pattern)
		if re.MatchString(safe) {
			removed = append(removed, name)
			safe = re.ReplaceAllString(safe, "[FILTERED]")
		}
	}
	return strings.TrimSpace(safe), uniqueStrings(removed)
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	for _, marker := range []string{"password", "passwd", "api_key", "apikey", "secret", "token", "private_key"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
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

func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
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
