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

func TestMetaFingerprintDeterministic(t *testing.T) {
	input1 := LogInput{
		RequestID:  "req-001",
		GatewayKey: "agk-001",
		Method:     "POST",
		Path:       "/v1/chat",
		Body:       []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
		ClientIP:   "192.168.1.1",
	}

	input2 := LogInput{
		RequestID:  "req-001",
		GatewayKey: "agk-001",
		Method:     "POST",
		Path:       "/v1/chat",
		Body:       []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
		ClientIP:   "192.168.1.1",
	}

	hash1 := computeMetaFingerprint(input1)
	hash2 := computeMetaFingerprint(input2)

	if hash1 != hash2 {
		t.Fatalf("same inputs should produce same hash: %s != %s", hash1, hash2)
	}
}

func TestMetaFingerprintDifferentInputs(t *testing.T) {
	input1 := LogInput{
		RequestID:  "req-001",
		GatewayKey: "agk-001",
		Method:     "POST",
		Path:       "/v1/chat",
		Body:       []byte(`{"test":true}`),
		ClientIP:   "192.168.1.1",
	}

	input2 := LogInput{
		RequestID:  "req-002",
		GatewayKey: "agk-001",
		Method:     "POST",
		Path:       "/v1/chat",
		Body:       []byte(`{"test":true}`),
		ClientIP:   "192.168.1.1",
	}

	hash1 := computeMetaFingerprint(input1)
	hash2 := computeMetaFingerprint(input2)

	if hash1 == hash2 {
		t.Fatalf("different inputs should produce different hash: %s == %s", hash1, hash2)
	}
}

func TestMetaFingerprintUsesBodyLength(t *testing.T) {
	input1 := LogInput{
		RequestID:  "req-001",
		GatewayKey: "agk-001",
		Method:     "POST",
		Path:       "/v1/chat",
		Body:       []byte(`{"short":true}`),
		ClientIP:   "192.168.1.1",
	}

	input2 := LogInput{
		RequestID:  "req-001",
		GatewayKey: "agk-001",
		Method:     "POST",
		Path:       "/v1/chat",
		Body:       []byte(`{"much_longer_content":true,"extra":"data"}`),
		ClientIP:   "192.168.1.1",
	}

	hash1 := computeMetaFingerprint(input1)
	hash2 := computeMetaFingerprint(input2)

	if hash1 == hash2 {
		t.Fatalf("different body lengths should produce different hash")
	}
}

func TestMetaFingerprintVsBodyHash(t *testing.T) {
	input := LogInput{
		RequestID:  "req-001",
		GatewayKey: "agk-001",
		Method:     "POST",
		Path:       "/v1/chat",
		Body:       []byte(`{"test":true}`),
		ClientIP:   "192.168.1.1",
	}

	hash := computeMetaFingerprint(input)

	if len(hash) != 64 {
		t.Fatalf("SM3 hex should be 64 chars, got %d", len(hash))
	}
}
