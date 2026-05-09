package audit

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoggerPersistsTokenModeFields(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	logger := NewLogger(store)
	requestID := logger.LogRequest(LogInput{
		RequestID:  "req-audit-1",
		GatewayKey: "agk-test-001",
		Method:     "POST",
		Path:       "/v1/chat/completions",
		Body:       []byte(`{"test":true}`),
		ClientIP:   "127.0.0.1",
	})

	logger.LogResponse(requestID, LogResponseInput{
		StatusCode:        200,
		Duration:          25 * time.Millisecond,
		Decision:          "Allow",
		Reason:            "compat mode allow",
		GateType:          "action",
		RiskScore:         10,
		RiskLevel:         "low",
		MatchedRules:      []string{"test_rule"},
		TokenStatus:       "skipped",
		AuthMode:          "compat",
		UnauthorizedAllow: true,
	})

	events, err := store.ReadAll()
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.TokenStatus != "skipped" {
		t.Fatalf("token status = %q, want skipped", event.TokenStatus)
	}
	if event.AuthMode != "compat" {
		t.Fatalf("auth mode = %q, want compat", event.AuthMode)
	}
	if !event.UnauthorizedAllow {
		t.Fatalf("expected unauthorized_allow to be true")
	}
}
