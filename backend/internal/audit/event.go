package audit

import "time"

type AuditEvent struct {
	RequestID         string    `json:"request_id"`
	Timestamp         time.Time `json:"timestamp"`
	GatewayKey        string    `json:"gateway_key,omitempty"`
	Method            string    `json:"method"`
	Path              string    `json:"path"`
	StatusCode        int       `json:"status_code"`
	DurationMs        int64     `json:"duration_ms"`
	BodyHash          string    `json:"body_hash,omitempty"`
	ClientIP          string    `json:"client_ip,omitempty"`
	Decision          string    `json:"decision,omitempty"`
	Reason            string    `json:"reason,omitempty"`
	GateType          string    `json:"gate_type,omitempty"`
	RiskScore         int       `json:"risk_score,omitempty"`
	RiskLevel         string    `json:"risk_level,omitempty"`
	MatchedRules      []string  `json:"matched_rules,omitempty"`
	TokenStatus       string    `json:"token_status,omitempty"`
	AuthMode          string    `json:"auth_mode,omitempty"`
	UnauthorizedAllow bool      `json:"unauthorized_allow,omitempty"`
	Error             string    `json:"error,omitempty"`
}

type LogInput struct {
	RequestID  string
	GatewayKey string
	Method     string
	Path       string
	Body       []byte
	ClientIP   string
}

type LogResponseInput struct {
	StatusCode        int
	Duration          time.Duration
	Decision          string
	Reason            string
	GateType          string
	RiskScore         int
	RiskLevel         string
	MatchedRules      []string
	TokenStatus       string
	AuthMode          string
	UnauthorizedAllow bool
	Error             string
}
