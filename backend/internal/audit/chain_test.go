package audit

import (
	"testing"
	"time"
)

func TestBuildAttackChainsFiltersNormalTraffic(t *testing.T) {
	now := time.Now()
	events := []AuditEvent{
		{
			RequestID:  "normal",
			Timestamp:  now,
			Decision:   "Allow",
			RiskScore:  0,
			StatusCode: 200,
		},
		{
			RequestID:   "attack",
			Timestamp:   now.Add(time.Second),
			Decision:    "Block",
			RiskScore:   70,
			RiskLevel:   "high",
			GateType:    "message",
			MatchedRules: []string{"prompt_injection", "sensitive_access"},
			Reason:       "prompt injection with sensitive intent",
			StatusCode:   403,
		},
	}

	chains := BuildAttackChains(events, 50)
	if len(chains) != 1 {
		t.Fatalf("len(chains) = %d, want 1", len(chains))
	}
	if chains[0].ChainID != "chain-attack" {
		t.Fatalf("chain ID = %q, want %q", chains[0].ChainID, "chain-attack")
	}
	if len(chains[0].Events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(chains[0].Events))
	}
	if chains[0].Events[2].EventType != "block" {
		t.Fatalf("outcome event type = %q, want block", chains[0].Events[2].EventType)
	}
}

func TestBuildAttackChainsIncludesDegradedRequest(t *testing.T) {
	events := []AuditEvent{
		{
			RequestID:  "degraded",
			Timestamp:  time.Now(),
			Decision:   "Degrade",
			RiskScore:  35,
			RiskLevel:  "low",
			GateType:   "message",
			StatusCode: 200,
		},
	}

	chains := BuildAttackChains(events, 50)
	if len(chains) != 1 {
		t.Fatalf("len(chains) = %d, want 1", len(chains))
	}
}
