package auth

import (
	"sync"
	"testing"
	"time"

	"aegisguard/pkg/smcrypto"
)

func TestCompareSchemaHashCaching(t *testing.T) {
	schemaCache = sync.Map{}

	toolSchema := []byte(`{"name": "test", "description": "a test schema"}`)
	correctHash := smcrypto.SM3Hex(toolSchema)

	token := &RequireToken{
		TokenID:    "token-001",
		ToolName:   "test-tool",
		SchemaHash: correctHash,
	}

	err := CompareSchemaHash(token, toolSchema)
	if err != nil {
		t.Fatalf("first CompareSchemaHash failed: %v", err)
	}

	err = CompareSchemaHash(token, toolSchema)
	if err != nil {
		t.Fatalf("second CompareSchemaHash with same schema failed: %v", err)
	}
}

func TestCompareSchemaHashCacheMiss(t *testing.T) {
	schemaCache = sync.Map{}

	toolSchema := []byte(`{"name": "test", "description": "a test schema"}`)

	token := &RequireToken{
		ToolName:   "test-tool",
		SchemaHash: "wronghash",
	}

	err := CompareSchemaHash(token, toolSchema)
	if err == nil {
		t.Error("CompareSchemaHash should fail with wrong hash")
	}
}

func TestCompareSchemaHashEmptyTokenHash(t *testing.T) {
	token := &RequireToken{
		ToolName:   "test-tool",
		SchemaHash: "",
	}

	toolSchema := []byte(`{"name": "test"}`)

	err := CompareSchemaHash(token, toolSchema)
	if err == nil {
		t.Error("CompareSchemaHash should fail with empty token hash")
	}
}

func TestCompareSchemaHashDifferentSchemas(t *testing.T) {
	schemaCache = sync.Map{}

	toolSchema1 := []byte(`{"name": "test1"}`)
	toolSchema2 := []byte(`{"name": "test2"}`)

	hash1 := smcrypto.SM3Hex(toolSchema1)
	hash2 := smcrypto.SM3Hex(toolSchema2)

	token1 := &RequireToken{ToolName: "t1", SchemaHash: hash1}
	token2 := &RequireToken{ToolName: "t2", SchemaHash: hash2}

	if err := CompareSchemaHash(token1, toolSchema1); err != nil {
		t.Fatalf("first failed: %v", err)
	}
	if err := CompareSchemaHash(token2, toolSchema2); err != nil {
		t.Fatalf("second failed: %v", err)
	}

	if err := CompareSchemaHash(token1, toolSchema1); err != nil {
		t.Error("cached call should not fail")
	}
}

func TestBuildSignMessageOptimization(t *testing.T) {
	token := &RequireToken{
		TokenID:    "token-001",
		ToolName:   "test-tool",
		Scope:      "read",
		AgentID:    "agent-001",
		SessionID:  "session-001",
		TaskID:     "task-001",
		ExpiresAt:  time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		Nonce:      "nonce-001",
		RiskLevel:  1,
		SchemaHash: "abc123",
		MaxCalls:   10,
	}

	msg1 := token.buildSignMessage()
	msg2 := token.buildSignMessage()

	if string(msg1) != string(msg2) {
		t.Error("buildSignMessage should return consistent result")
	}

	expected := "token-001|test-tool|read|agent-001|session-001|task-001|2026-07-02T12:00:00Z|nonce-001|1|abc123|10"
	if string(msg1) != expected {
		t.Errorf("buildSignMessage output mismatch:\ngot:  %s\nexpected: %s", string(msg1), expected)
	}
}

func TestBuildSignMessageOptimizationPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	token := &RequireToken{
		TokenID:    "token-001",
		ToolName:   "test-tool-with-longer-name",
		Scope:      "read-write-execute",
		AgentID:    "agent-001-uuid",
		SessionID:  "session-001-uuid",
		TaskID:     "task-001-uuid",
		ExpiresAt:  time.Date(2026, 7, 2, 12, 0, 0, 123000000, time.UTC),
		Nonce:      "nonce-001-very-long-nonce",
		RiskLevel:  5,
		SchemaHash: "abc123def456",
		MaxCalls:   100,
	}

	t.Run("Deterministic", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			msg := token.buildSignMessage()
			expected := "token-001|test-tool-with-longer-name|read-write-execute|agent-001-uuid|session-001-uuid|task-001-uuid|2026-07-02T12:00:00.123Z|nonce-001-very-long-nonce|5|abc123def456|100"
			if string(msg) != expected {
				t.Errorf("iteration %d: mismatch", i)
			}
		}
	})
}
