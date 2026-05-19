package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreBasic(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-audit.db")

	cfg := SQLiteConfig{
		Path:        dbPath,
		WALMode:     true,
		CacheSize:   64000,
		BusyTimeout: 5 * time.Second,
	}

	store, err := NewSQLiteStore(cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	event := AuditEvent{
		RequestID:    "test-001",
		Timestamp:    time.Now(),
		GatewayKey:   "agk-test",
		Method:       "POST",
		Path:         "/v1/chat",
		StatusCode:   200,
		DurationMs:   150,
		BodyHash:     "abc123",
		ClientIP:     "127.0.0.1",
		Decision:     "allow",
		Reason:       "test reason",
		GateType:     "message",
		RiskScore:    10,
		RiskLevel:    "low",
		MatchedRules: []string{"rule1", "rule2"},
		TokenStatus:  "valid",
		AuthMode:     "strict",
	}

	if err := store.Append(event); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	events, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].RequestID != event.RequestID {
		t.Errorf("expected RequestID %s, got %s", event.RequestID, events[0].RequestID)
	}

	if events[0].Decision != event.Decision {
		t.Errorf("expected Decision %s, got %s", event.Decision, events[0].Decision)
	}

	if len(events[0].MatchedRules) != 2 {
		t.Errorf("expected 2 matched rules, got %d", len(events[0].MatchedRules))
	}
}

func TestSQLiteStoreQuerySince(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-audit.db")

	cfg := DefaultSQLiteConfig(dbPath)
	store, err := NewSQLiteStore(cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	now := time.Now()

	oldEvent := AuditEvent{
		RequestID: "old-001",
		Timestamp: now.Add(-2 * time.Hour),
		Method:    "GET",
		Path:      "/v1/old",
	}
	if err := store.Append(oldEvent); err != nil {
		t.Fatalf("Append oldEvent failed: %v", err)
	}

	newEvent := AuditEvent{
		RequestID: "new-001",
		Timestamp: now,
		Method:    "POST",
		Path:      "/v1/new",
	}
	if err := store.Append(newEvent); err != nil {
		t.Fatalf("Append newEvent failed: %v", err)
	}

	events, err := store.QuerySince(now.Add(-1 * time.Hour))
	if err != nil {
		t.Fatalf("QuerySince failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].RequestID != "new-001" {
		t.Errorf("expected RequestID new-001, got %s", events[0].RequestID)
	}
}

func TestSQLiteStoreWALMode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-wal.db")

	cfg := SQLiteConfig{
		Path:        dbPath,
		WALMode:     true,
		CacheSize:   64000,
		BusyTimeout: 5 * time.Second,
	}

	store, err := NewSQLiteStore(cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore with WAL failed: %v", err)
	}
	defer store.Close()

	walPath := dbPath + "-wal"

	if _, err := os.Stat(walPath); os.IsNotExist(err) {
		t.Log("WAL file not created yet (expected after first write)")
	}

	event := AuditEvent{
		RequestID: "wal-test-001",
		Timestamp: time.Now(),
		Method:    "POST",
		Path:      "/v1/test",
	}
	if err := store.Append(event); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file should exist after write")
	}
}

func TestSQLiteStoreDeleteMode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-delete.db")

	cfg := SQLiteConfig{
		Path:        dbPath,
		WALMode:     false,
		CacheSize:   64000,
		BusyTimeout: 5 * time.Second,
	}

	store, err := NewSQLiteStore(cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore without WAL failed: %v", err)
	}
	defer store.Close()

	event := AuditEvent{
		RequestID: "delete-test-001",
		Timestamp: time.Now(),
		Method:    "POST",
		Path:      "/v1/test",
	}
	if err := store.Append(event); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	walPath := dbPath + "-wal"
	if _, err := os.Stat(walPath); !os.IsNotExist(err) {
		t.Error("WAL file should not exist in DELETE mode")
	}
}

func TestSQLiteStoreCount(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-count.db")

	store, err := NewSQLiteStore(DefaultSQLiteConfig(dbPath))
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	for i := 0; i < 5; i++ {
		event := AuditEvent{
			RequestID: "count-test-" + string(rune('0'+i)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Method:    "POST",
			Path:      "/v1/test",
		}
		if err := store.Append(event); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 5 {
		t.Errorf("expected count 5, got %d", count)
	}
}

func TestSQLiteStoreDeleteBefore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-delete-before.db")

	store, err := NewSQLiteStore(DefaultSQLiteConfig(dbPath))
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	now := time.Now()

	oldEvent := AuditEvent{
		RequestID: "delete-old-001",
		Timestamp: now.Add(-48 * time.Hour),
		Method:    "GET",
		Path:      "/v1/old",
	}
	if err := store.Append(oldEvent); err != nil {
		t.Fatalf("Append oldEvent failed: %v", err)
	}

	newEvent := AuditEvent{
		RequestID: "delete-new-001",
		Timestamp: now,
		Method:    "POST",
		Path:      "/v1/new",
	}
	if err := store.Append(newEvent); err != nil {
		t.Fatalf("Append newEvent failed: %v", err)
	}

	affected, err := store.DeleteBefore(now.Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("DeleteBefore failed: %v", err)
	}

	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}

	events, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("expected 1 event after delete, got %d", len(events))
	}
}

func TestSQLiteStoreQueryByDecision(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-decision.db")

	store, err := NewSQLiteStore(DefaultSQLiteConfig(dbPath))
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	events := []AuditEvent{
		{RequestID: "allow-001", Timestamp: time.Now(), Method: "POST", Path: "/v1/a", Decision: "allow"},
		{RequestID: "block-001", Timestamp: time.Now().Add(time.Second), Method: "POST", Path: "/v1/b", Decision: "block"},
		{RequestID: "allow-002", Timestamp: time.Now().Add(2 * time.Second), Method: "POST", Path: "/v1/c", Decision: "allow"},
	}

	for _, ev := range events {
		if err := store.Append(ev); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	allowEvents, err := store.QueryByDecision("allow", 10)
	if err != nil {
		t.Fatalf("QueryByDecision failed: %v", err)
	}

	if len(allowEvents) != 2 {
		t.Errorf("expected 2 allow events, got %d", len(allowEvents))
	}

	blockEvents, err := store.QueryByDecision("block", 10)
	if err != nil {
		t.Fatalf("QueryByDecision failed: %v", err)
	}

	if len(blockEvents) != 1 {
		t.Errorf("expected 1 block event, got %d", len(blockEvents))
	}
}

func TestSQLiteStoreCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-checkpoint.db")

	cfg := SQLiteConfig{
		Path:        dbPath,
		WALMode:     true,
		CacheSize:   64000,
		BusyTimeout: 5 * time.Second,
	}

	store, err := NewSQLiteStore(cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	for i := 0; i < 10; i++ {
		event := AuditEvent{
			RequestID: "checkpoint-test-" + string(rune('0'+i)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Method:    "POST",
			Path:      "/v1/test",
		}
		if err := store.Append(event); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	if err := store.Checkpoint(); err != nil {
		t.Fatalf("Checkpoint failed: %v", err)
	}
}

func TestSQLiteStoreConcurrentWrite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-concurrent.db")

	store, err := NewSQLiteStore(DefaultSQLiteConfig(dbPath))
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			event := AuditEvent{
				RequestID: "concurrent-test-" + string(rune('0'+id)),
				Timestamp: time.Now(),
				Method:    "POST",
				Path:      "/v1/test",
			}
			if err := store.Append(event); err != nil {
				t.Errorf("concurrent Append failed: %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 10 {
		t.Errorf("expected 10 events, got %d", count)
	}
}
