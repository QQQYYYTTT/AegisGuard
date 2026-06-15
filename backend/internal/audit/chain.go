package audit

import (
	"fmt"
	"strings"
	"time"
)

type ChainEvent struct {
	ID                string    `json:"id"`
	RequestID         string    `json:"request_id"`
	Timestamp         time.Time `json:"timestamp"`
	Method            string    `json:"method"`
	Path              string    `json:"path"`
	StatusCode        int       `json:"status_code"`
	Status            int       `json:"status"`
	DurationMs        int64     `json:"duration_ms"`
	Decision          string    `json:"decision"`
	RiskScore         int       `json:"risk_score"`
	RiskLevel         string    `json:"risk_level"`
	GateType          string    `json:"gate_type"`
	Reason            string    `json:"reason"`
	MatchedRules      []string  `json:"matched_rules,omitempty"`
	TokenStatus       string    `json:"token_status,omitempty"`
	AuthMode          string    `json:"auth_mode,omitempty"`
	UnauthorizedAllow bool      `json:"unauthorized_allow,omitempty"`
	BodyHash          string    `json:"body_hash,omitempty"`
	ClientIP          string    `json:"client_ip,omitempty"`
	AgentID           string    `json:"agent_id,omitempty"`
	SessionID         string    `json:"session_id,omitempty"`
	ToolName          string    `json:"tool_name,omitempty"`
	EventType         string    `json:"event_type"`
	Description       string    `json:"description"`
}

type AttackChain struct {
	ChainID   string       `json:"chain_id"`
	Events    []ChainEvent `json:"events"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
	Severity  string       `json:"severity"`
	Summary   string       `json:"summary"`
}

func BuildAttackChains(events []AuditEvent, limit int) []AttackChain {
	chains := make([]AttackChain, 0)
	for _, event := range events {
		if !IsSecurityRelevant(event) {
			continue
		}
		chains = append(chains, buildAttackChain(event))
		if limit > 0 && len(chains) >= limit {
			break
		}
	}
	return chains
}

func CountAttackChains(events []AuditEvent) int {
	count := 0
	for _, event := range events {
		if IsSecurityRelevant(event) {
			count++
		}
	}
	return count
}

func IsSecurityRelevant(event AuditEvent) bool {
	decision := strings.ToLower(strings.TrimSpace(event.Decision))
	switch decision {
	case "block", "deny", "degrade", "humanapproval", "human_approval":
		return true
	}
	return event.RiskScore > 0 || len(event.MatchedRules) > 0 || event.UnauthorizedAllow
}

func buildAttackChain(event AuditEvent) AttackChain {
	requestID := strings.TrimSpace(event.RequestID)
	if requestID == "" {
		requestID = fmt.Sprintf("audit-%d", event.Timestamp.UnixNano())
	}

	gateType := firstNonEmptyString(event.GateType, "message")
	decision := firstNonEmptyString(event.Decision, "Allow")
	reason := firstNonEmptyString(event.Reason, "security policy evaluated the request")
	start := event.Timestamp
	detectionTime := start.Add(time.Millisecond)
	end := detectionTime.Add(time.Millisecond)

	events := []ChainEvent{
		newChainEvent(event, requestID+"-input", start, "input", "Gateway received the agent request"),
		newChainEvent(
			event,
			requestID+"-gate",
			detectionTime,
			"gate",
			fmt.Sprintf("%s gate evaluated the request: %s", gateType, reason),
		),
	}

	outcomeType := "allow"
	if strings.EqualFold(decision, "Block") || strings.EqualFold(decision, "Deny") {
		outcomeType = "block"
	}
	events = append(events, newChainEvent(
		event,
		requestID+"-outcome",
		end,
		outcomeType,
		fmt.Sprintf("AegisGuard decision: %s", decision),
	))

	return AttackChain{
		ChainID:   "chain-" + requestID,
		Events:    events,
		StartTime: start,
		EndTime:   end,
		Severity:  chainSeverity(event),
		Summary: fmt.Sprintf(
			"%s gate produced %s (risk %d): %s",
			gateType,
			decision,
			event.RiskScore,
			reason,
		),
	}
}

func newChainEvent(
	event AuditEvent,
	id string,
	timestamp time.Time,
	eventType string,
	description string,
) ChainEvent {
	return ChainEvent{
		ID:                id,
		RequestID:         event.RequestID,
		Timestamp:         timestamp,
		Method:            event.Method,
		Path:              event.Path,
		StatusCode:        event.StatusCode,
		Status:            event.StatusCode,
		DurationMs:        event.DurationMs,
		Decision:          event.Decision,
		RiskScore:         event.RiskScore,
		RiskLevel:         firstNonEmptyString(event.RiskLevel, chainSeverity(event)),
		GateType:          event.GateType,
		Reason:            event.Reason,
		MatchedRules:      event.MatchedRules,
		TokenStatus:       event.TokenStatus,
		AuthMode:          event.AuthMode,
		UnauthorizedAllow: event.UnauthorizedAllow,
		BodyHash:          event.BodyHash,
		ClientIP:          event.ClientIP,
		AgentID:           event.GatewayKey,
		SessionID:         event.RequestID,
		EventType:         eventType,
		Description:       description,
	}
}

func chainSeverity(event AuditEvent) string {
	level := strings.ToLower(strings.TrimSpace(event.RiskLevel))
	switch level {
	case "critical", "high", "medium", "low":
		return level
	}

	decision := strings.ToLower(strings.TrimSpace(event.Decision))
	switch {
	case event.RiskScore >= 85 || decision == "deny":
		return "critical"
	case event.RiskScore >= 65 || decision == "block":
		return "high"
	case event.RiskScore >= 35 || decision == "degrade":
		return "medium"
	default:
		return "low"
	}
}

func firstNonEmptyString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
