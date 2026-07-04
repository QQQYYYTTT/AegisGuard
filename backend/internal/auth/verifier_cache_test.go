package auth

import (
	"testing"
	"time"
)

func TestVerifierCache(t *testing.T) {
	InitSigningKey("")

	v := NewVerifier()
	ResetNonces()

	token := &RequireToken{
		ToolName:   "test-tool",
		Scope:      "read",
		AgentID:    "agent-001",
		SessionID:  "session-001",
		TaskID:     "task-001",
		Nonce:      "nonce-001",
		RiskLevel:  1,
		SchemaHash: "abc123",
		MaxCalls:   10,
		ExpiresAt:  time.Now().Add(time.Hour),
	}

	if err := token.Sign(); err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	cacheKey := token.buildCacheKey()
	if cacheKey == "" {
		t.Error("buildCacheKey returned empty string")
	}

	checks := v.Inspect(token)
	if !checks.IsValid() {
		t.Fatalf("first inspection failed: %+v", checks)
	}

	if _, ok := v.cache.Load(cacheKey); !ok {
		t.Error("cache should contain entry after successful inspection")
	}

	checks = v.Inspect(token)
	if !checks.IsValid() {
		t.Fatalf("second inspection with cached result failed: %+v", checks)
	}
}

func TestVerifierCacheInvalidSignature(t *testing.T) {
	InitSigningKey("")

	v := NewVerifier()
	ResetNonces()

	token := &RequireToken{
		ToolName:   "test-tool",
		Scope:      "read",
		AgentID:    "agent-001",
		SessionID:  "session-001",
		TaskID:     "task-001",
		Nonce:      "nonce-002",
		RiskLevel:  1,
		SchemaHash: "abc123",
		MaxCalls:   10,
		ExpiresAt:  time.Now().Add(time.Hour),
		Signature:  "invalid-signature",
	}

	cacheKey := token.buildCacheKey()

	err := v.Verify(token)
	if err == nil {
		t.Error("verification should fail with invalid signature")
	}

	if _, ok := v.cache.Load(cacheKey); !ok {
		t.Error("cache should contain entry for failed verification")
	}

	var cached *cachedVerification
	if cv, ok := v.cache.Load(cacheKey); ok {
		cached = cv.(*cachedVerification)
		if cached.valid {
			t.Error("cached entry should be invalid")
		}
	}
}

func TestNonceGC(t *testing.T) {
	ResetNonces()
	nonceExpiration = time.Second

	nonceMu.Lock()
	usedNonces["expired-nonce"] = time.Now().Add(-time.Second).Unix()
	usedNonces["valid-nonce"] = time.Now().Add(time.Hour).Unix()
	nonceMu.Unlock()

	StartNonceGC(100 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	StopNonceGC()

	nonceMu.RLock()
	if _, exists := usedNonces["valid-nonce"]; !exists {
		t.Error("valid-nonce should still exist after GC")
	}
	nonceMu.RUnlock()
}

func TestSetNonceMaxEntriesPrunesOldestExpirations(t *testing.T) {
	ResetNonces()
	nonceMu.Lock()
	usedNonces["nonce-1"] = time.Now().Add(1 * time.Minute).Unix()
	usedNonces["nonce-2"] = time.Now().Add(2 * time.Minute).Unix()
	usedNonces["nonce-3"] = time.Now().Add(3 * time.Minute).Unix()
	nonceMu.Unlock()

	SetNonceMaxEntries(2)

	nonceMu.RLock()
	defer nonceMu.RUnlock()
	if len(usedNonces) != 2 {
		t.Fatalf("expected nonce cache to be pruned to 2 entries, got %d", len(usedNonces))
	}
	if _, exists := usedNonces["nonce-1"]; exists {
		t.Fatalf("expected earliest-expiring nonce to be evicted")
	}
}

func TestVerifyNoncePrunesExpiredEntriesBeforeInsert(t *testing.T) {
	ResetNonces()
	SetNonceMaxEntries(1)
	SetNonceExpiration(time.Hour)

	nonceMu.Lock()
	usedNonces["expired-nonce"] = time.Now().Add(-time.Minute).Unix()
	nonceMu.Unlock()

	v := NewVerifier()
	if err := v.verifyNonce(&RequireToken{Nonce: "fresh-nonce"}, true); err != nil {
		t.Fatalf("verifyNonce returned error: %v", err)
	}
	if got := NonceCount(); got != 1 {
		t.Fatalf("expected exactly one nonce after pruning + insert, got %d", got)
	}
	nonceMu.RLock()
	defer nonceMu.RUnlock()
	if _, exists := usedNonces["expired-nonce"]; exists {
		t.Fatalf("expired nonce should have been pruned before insert")
	}
	if _, exists := usedNonces["fresh-nonce"]; !exists {
		t.Fatalf("fresh nonce should have been inserted")
	}
}

func TestBuildCacheKeyDeterministic(t *testing.T) {
	if err := InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}

	token1 := &RequireToken{
		ToolName:   "test-tool",
		Scope:      "read",
		AgentID:    "agent-001",
		SessionID:  "session-001",
		TaskID:     "task-001",
		Nonce:      "nonce-001",
		RiskLevel:  1,
		SchemaHash: "abc123",
		MaxCalls:   10,
		ExpiresAt:  time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
	}
	if err := token1.Sign(); err != nil {
		t.Fatalf("failed to sign token1: %v", err)
	}

	token2 := &RequireToken{
		ToolName:   "test-tool",
		Scope:      "read",
		AgentID:    "agent-001",
		SessionID:  "session-001",
		TaskID:     "task-001",
		Nonce:      "different-nonce",
		RiskLevel:  1,
		SchemaHash: "abc123",
		MaxCalls:   10,
		ExpiresAt:  time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
	}
	if err := token2.Sign(); err != nil {
		t.Fatalf("failed to sign token2: %v", err)
	}

	key1 := token1.buildCacheKey()
	key2 := token2.buildCacheKey()

	if key1 == key2 {
		t.Error("cache key should differ when signed token contents differ")
	}

	token3 := &RequireToken{
		ToolName:   "test-tool",
		Scope:      "read",
		AgentID:    "agent-001",
		SessionID:  "session-001",
		TaskID:     "task-001",
		Nonce:      "nonce-001",
		RiskLevel:  1,
		SchemaHash: "abc123",
		MaxCalls:   10,
		ExpiresAt:  time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		Signature:  "different-signature",
	}

	key3 := token3.buildCacheKey()
	if key1 == key3 {
		t.Error("cache key should differ when signature changes")
	}
}
