package runtime

import (
	"fmt"
	"time"

	"aegisguard/backend/internal/audit"
	"aegisguard/backend/internal/security"
)

type Service struct {
	store *audit.Store
}

func NewService(store *audit.Store) *Service {
	return &Service{store: store}
}

func (s *Service) EvaluateProtectedRequest(body security.VerifyRequestBody) (map[string]any, error) {
	verification := security.VerifyRequireToken(body.Token, body)
	gateDecision := security.DecideGate(body, verification)
	sandboxResult := security.RunMemorySandbox(body.RawResult)

	entry := map[string]any{
		"id":              fmt.Sprintf("audit-%d", time.Now().UnixMilli()),
		"created_at":      time.Now().UTC().Format(time.RFC3339),
		"scenario_id":     emptyDefault(body.ScenarioID, "custom"),
		"user_goal":       body.UserGoal,
		"tool_name":       body.ToolName,
		"requested_scope": body.RequestedScope,
		"decision":        gateDecision.Action,
		"stage":           gateDecision.Stage,
		"reason":          gateDecision.Reason,
		"verification":    verification,
		"token_preview": map[string]any{
			"agent_id":   body.Token.AgentID,
			"session_id": body.Token.SessionID,
			"nonce":      body.Token.Nonce,
		},
		"sandbox": sandboxResult,
		"logs": []map[string]any{
			{"step": "Policy Center", "text": "Authorization token issued and bound to the current session."},
			{"step": "RequireShield", "text": ternary(verification.AllPassed, "Authorization verification passed.", "Authorization verification failed.")},
			{"step": gateDecision.Stage, "text": fmt.Sprintf("%s: %s", gateDecision.Action, gateDecision.Reason)},
			{"step": "Memory Sandbox", "text": fmt.Sprintf("Security summary generated with fingerprint %s.", sandboxResult.FingerprintSM3)},
		},
	}

	items, err := s.store.ReadAll()
	if err != nil {
		return nil, err
	}
	items = append([]map[string]any{entry}, items...)
	if len(items) > 120 {
		items = items[:120]
	}
	if err := s.store.WriteAll(items); err != nil {
		return nil, err
	}

	return map[string]any{
		"verification": verification,
		"gateDecision": gateDecision,
		"sandboxResult": sandboxResult,
		"auditLogs": entry["logs"],
		"auditEntry": entry,
	}, nil
}

func (s *Service) ListAudit() ([]map[string]any, error) {
	return s.store.ReadAll()
}

func (s *Service) ClearAudit() error {
	return s.store.WriteAll([]map[string]any{})
}

func emptyDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func ternary(condition bool, whenTrue, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}
