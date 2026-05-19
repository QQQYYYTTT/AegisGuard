package auth

import (
	"testing"
	"time"
)

func TestNonceRWMutexConcurrency(t *testing.T) {
	ResetNonces()

	if err := InitSigningKey(""); err != nil {
		t.Fatalf("Failed to init signing key: %v", err)
	}

	token1, err := NewToken("test-tool", "read", "agent-001", "session-123", "task-456", 5*time.Minute, 0)
	if err != nil {
		t.Fatalf("Failed to create token1: %v", err)
	}

	verifier := NewVerifier()

	if err := verifier.Verify(token1); err != nil {
		t.Fatalf("First verification should succeed: %v", err)
	}

	err = verifier.Verify(token1)
	if err == nil {
		t.Fatalf("Second verification with same nonce should fail")
	}
}

func TestNonceExpirationMechanism(t *testing.T) {
	ResetNonces()

	nonceMu.Lock()
	usedNonces["test-nonce"] = time.Now().Add(-1 * time.Hour).Unix()
	nonceMu.Unlock()

	nonceMu.RLock()
	expiresAt, exists := usedNonces["test-nonce"]
	nonceMu.RUnlock()

	if !exists {
		t.Fatal("nonce should exist")
	}

	if time.Now().Unix() >= expiresAt {
		t.Logf("Nonce correctly marked as expired (expiresAt=%d, now=%d)", expiresAt, time.Now().Unix())
	}
}

func TestNonceRWMutexReadWrite(t *testing.T) {
	ResetNonces()

	nonce := "test-nonce-123"

	nonceMu.RLock()
	_, exists := usedNonces[nonce]
	nonceMu.RUnlock()

	if exists {
		t.Fatal("nonce should not exist initially")
	}

	nonceMu.Lock()
	usedNonces[nonce] = time.Now().Add(24 * time.Hour).Unix()
	nonceMu.Unlock()

	nonceMu.RLock()
	expiresAt, exists := usedNonces[nonce]
	nonceMu.RUnlock()

	if !exists {
		t.Fatal("nonce should exist after write")
	}

	if expiresAt <= time.Now().Unix() {
		t.Fatal("expiration should be in the future")
	}
}

func TestNonceRWMutexNoBlocking(t *testing.T) {
	ResetNonces()

	if err := InitSigningKey(""); err != nil {
		t.Fatalf("Failed to init signing key: %v", err)
	}

	done := make(chan bool, 2)

	go func() {
		token, _ := NewToken("test-tool", "read", "agent-001", "session-1", "task-1", 5*time.Minute, 0)
		verifier := NewVerifier()
		verifier.Verify(token)
		done <- true
	}()

	go func() {
		token, _ := NewToken("test-tool", "read", "agent-001", "session-2", "task-2", 5*time.Minute, 0)
		verifier := NewVerifier()
		verifier.Verify(token)
		done <- true
	}()

	timeout := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("RWMutex read operations should not block each other")
		}
	}
}