package auth

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteTokenStorePersistsState(t *testing.T) {
	ResetNonces()
	if err := InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}

	path := filepath.Join(t.TempDir(), "tokens.db")
	store, err := NewSQLiteTokenStore(path)
	if err != nil {
		t.Fatalf("new sqlite token store: %v", err)
	}
	defer store.Close()

	token, err := store.Issue("weather.query", "weather.query:invoke", "agent-1", "session-1", "task-1", time.Minute, 2)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	token.SchemaHash = "schema-hash-1"
	if err := token.Sign(); err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if err := store.Save(token); err != nil {
		t.Fatalf("save token: %v", err)
	}

	consumed, err := store.Consume(token.TokenID)
	if err != nil {
		t.Fatalf("consume token: %v", err)
	}
	if consumed.CallCount != 1 {
		t.Fatalf("call count = %d, want 1", consumed.CallCount)
	}

	reopened, err := NewSQLiteTokenStore(path)
	if err != nil {
		t.Fatalf("reopen sqlite token store: %v", err)
	}
	defer reopened.Close()

	stored, err := reopened.GetByID(token.TokenID)
	if err != nil {
		t.Fatalf("get token after reopen: %v", err)
	}
	if stored.CallCount != 1 {
		t.Fatalf("persisted call count = %d, want 1", stored.CallCount)
	}
	if stored.SchemaHash != "schema-hash-1" {
		t.Fatalf("persisted schema hash = %q", stored.SchemaHash)
	}

	if err := reopened.Revoke(token.TokenID); err != nil {
		t.Fatalf("revoke token: %v", err)
	}
	if _, err := reopened.GetByID(token.TokenID); err == nil {
		t.Fatal("expected revoked token lookup to fail")
	}
}
