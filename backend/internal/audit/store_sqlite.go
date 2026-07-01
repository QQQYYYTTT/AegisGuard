package audit

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	mu   sync.RWMutex
	db   *sql.DB
	path string
}

type SQLiteConfig struct {
	Path        string
	WALMode     bool
	CacheSize   int
	BusyTimeout time.Duration
}

func DefaultSQLiteConfig(path string) SQLiteConfig {
	return SQLiteConfig{
		Path:        path,
		WALMode:     true,
		CacheSize:   64000,
		BusyTimeout: 5 * time.Second,
	}
}

func NewSQLiteStore(cfg SQLiteConfig) (*SQLiteStore, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("sqlite path is empty")
	}

	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := configurePragma(db, cfg); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure pragma: %w", err)
	}

	if err := migrateAuditTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate audit tables: %w", err)
	}

	return &SQLiteStore{db: db, path: cfg.Path}, nil
}

func configurePragma(db *sql.DB, cfg SQLiteConfig) error {
	pragmas := []string{
		fmt.Sprintf("PRAGMA busy_timeout=%d;", cfg.BusyTimeout.Milliseconds()),
		fmt.Sprintf("PRAGMA cache_size=%d;", cfg.CacheSize),
		"PRAGMA foreign_keys=ON;",
	}

	if cfg.WALMode {
		pragmas = append(pragmas, "PRAGMA journal_mode=WAL;")
		pragmas = append(pragmas, "PRAGMA synchronous=NORMAL;")
	} else {
		pragmas = append(pragmas, "PRAGMA journal_mode=DELETE;")
		pragmas = append(pragmas, "PRAGMA synchronous=FULL;")
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("exec pragma %s: %w", pragma, err)
		}
	}

	return nil
}

func migrateAuditTables(db *sql.DB) error {
	const auditTable = `
CREATE TABLE IF NOT EXISTS audit_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	request_id TEXT NOT NULL,
	timestamp DATETIME NOT NULL,
	gateway_key TEXT,
	method TEXT NOT NULL,
	path TEXT NOT NULL,
	status_code INTEGER,
	duration_ms INTEGER,
	body_hash TEXT,
	client_ip TEXT,
	decision TEXT,
	reason TEXT,
	gate_type TEXT,
	risk_score INTEGER DEFAULT 0,
	risk_level TEXT,
	matched_rules TEXT,
	token_status TEXT,
	auth_mode TEXT,
	unauthorized_allow INTEGER DEFAULT 0,
	error TEXT
);`

	const indexes = `
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_request_id ON audit_events(request_id);
CREATE INDEX IF NOT EXISTS idx_audit_decision ON audit_events(decision);
CREATE INDEX IF NOT EXISTS idx_audit_gate_type ON audit_events(gate_type);
CREATE INDEX IF NOT EXISTS idx_audit_risk_level ON audit_events(risk_level);`

	if _, err := db.Exec(auditTable); err != nil {
		return fmt.Errorf("create audit_events table: %w", err)
	}

	for _, idx := range strings.Split(indexes, ";") {
		idx = strings.TrimSpace(idx)
		if idx == "" {
			continue
		}
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}

	return nil
}

func (s *SQLiteStore) Append(event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	const query = `
INSERT INTO audit_events (
	request_id, timestamp, gateway_key, method, path,
	status_code, duration_ms, body_hash, client_ip,
	decision, reason, gate_type, risk_score, risk_level,
	matched_rules, token_status, auth_mode, unauthorized_allow, error
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	matchedRulesJSON := strings.Join(event.MatchedRules, ",")
	unauthorizedAllow := 0
	if event.UnauthorizedAllow {
		unauthorizedAllow = 1
	}

	_, err := s.db.Exec(query,
		event.RequestID,
		event.Timestamp,
		event.GatewayKey,
		event.Method,
		event.Path,
		event.StatusCode,
		event.DurationMs,
		event.BodyHash,
		event.ClientIP,
		event.Decision,
		event.Reason,
		event.GateType,
		event.RiskScore,
		event.RiskLevel,
		matchedRulesJSON,
		event.TokenStatus,
		event.AuthMode,
		unauthorizedAllow,
		event.Error,
	)

	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}

	return nil
}

func (s *SQLiteStore) ReadAll() ([]AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	const query = `
SELECT request_id, timestamp, gateway_key, method, path,
	status_code, duration_ms, body_hash, client_ip,
	decision, reason, gate_type, risk_score, risk_level,
	matched_rules, token_status, auth_mode, unauthorized_allow, error
FROM audit_events
ORDER BY timestamp DESC;`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query audit events: %w", err)
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var ev AuditEvent
		var matchedRulesStr sql.NullString
		var unauthorizedAllow int

		err := rows.Scan(
			&ev.RequestID,
			&ev.Timestamp,
			&ev.GatewayKey,
			&ev.Method,
			&ev.Path,
			&ev.StatusCode,
			&ev.DurationMs,
			&ev.BodyHash,
			&ev.ClientIP,
			&ev.Decision,
			&ev.Reason,
			&ev.GateType,
			&ev.RiskScore,
			&ev.RiskLevel,
			&matchedRulesStr,
			&ev.TokenStatus,
			&ev.AuthMode,
			&unauthorizedAllow,
			&ev.Error,
		)
		if err != nil {
			continue
		}

		if matchedRulesStr.Valid && matchedRulesStr.String != "" {
			ev.MatchedRules = strings.Split(matchedRulesStr.String, ",")
		}
		ev.UnauthorizedAllow = unauthorizedAllow == 1

		events = append(events, ev)
	}

	if events == nil {
		events = []AuditEvent{}
	}

	return events, nil
}

func (s *SQLiteStore) QuerySince(since time.Time) ([]AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	const query = `
SELECT request_id, timestamp, gateway_key, method, path,
	status_code, duration_ms, body_hash, client_ip,
	decision, reason, gate_type, risk_score, risk_level,
	matched_rules, token_status, auth_mode, unauthorized_allow, error
FROM audit_events
WHERE timestamp >= ?
ORDER BY timestamp DESC;`

	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, fmt.Errorf("query audit events since: %w", err)
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var ev AuditEvent
		var matchedRulesStr sql.NullString
		var unauthorizedAllow int

		err := rows.Scan(
			&ev.RequestID,
			&ev.Timestamp,
			&ev.GatewayKey,
			&ev.Method,
			&ev.Path,
			&ev.StatusCode,
			&ev.DurationMs,
			&ev.BodyHash,
			&ev.ClientIP,
			&ev.Decision,
			&ev.Reason,
			&ev.GateType,
			&ev.RiskScore,
			&ev.RiskLevel,
			&matchedRulesStr,
			&ev.TokenStatus,
			&ev.AuthMode,
			&unauthorizedAllow,
			&ev.Error,
		)
		if err != nil {
			continue
		}

		if matchedRulesStr.Valid && matchedRulesStr.String != "" {
			ev.MatchedRules = strings.Split(matchedRulesStr.String, ",")
		}
		ev.UnauthorizedAllow = unauthorizedAllow == 1

		events = append(events, ev)
	}

	if events == nil {
		events = []AuditEvent{}
	}

	return events, nil
}

func (s *SQLiteStore) QueryByDecision(decision string, limit int) ([]AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	const query = `
SELECT request_id, timestamp, gateway_key, method, path,
	status_code, duration_ms, body_hash, client_ip,
	decision, reason, gate_type, risk_score, risk_level,
	matched_rules, token_status, auth_mode, unauthorized_allow, error
FROM audit_events
WHERE decision = ?
ORDER BY timestamp DESC
LIMIT ?;`

	rows, err := s.db.Query(query, decision, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit events by decision: %w", err)
	}
	defer rows.Close()

	return scanAuditEvents(rows)
}

func (s *SQLiteStore) QueryByGateType(gateType string, limit int) ([]AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	const query = `
SELECT request_id, timestamp, gateway_key, method, path,
	status_code, duration_ms, body_hash, client_ip,
	decision, reason, gate_type, risk_score, risk_level,
	matched_rules, token_status, auth_mode, unauthorized_allow, error
FROM audit_events
WHERE gate_type = ?
ORDER BY timestamp DESC
LIMIT ?;`

	rows, err := s.db.Query(query, gateType, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit events by gate type: %w", err)
	}
	defer rows.Close()

	return scanAuditEvents(rows)
}

func (s *SQLiteStore) QueryByTimeRange(start, end time.Time, limit int) ([]AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	const query = `
SELECT request_id, timestamp, gateway_key, method, path,
	status_code, duration_ms, body_hash, client_ip,
	decision, reason, gate_type, risk_score, risk_level,
	matched_rules, token_status, auth_mode, unauthorized_allow, error
FROM audit_events
WHERE timestamp >= ? AND timestamp <= ?
ORDER BY timestamp DESC
LIMIT ?;`

	rows, err := s.db.Query(query, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit events by time range: %w", err)
	}
	defer rows.Close()

	return scanAuditEvents(rows)
}

func (s *SQLiteStore) Count() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	const query = `SELECT COUNT(*) FROM audit_events;`
	var count int64
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count audit events: %w", err)
	}
	return count, nil
}

func (s *SQLiteStore) AggregateThreatSources(since time.Time) ([]ThreatSourceRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	const query = `
SELECT client_ip, risk_level, decision, timestamp
FROM audit_events
WHERE timestamp >= ?
  AND risk_level IN ('high', 'critical')
  AND decision IN ('Block', 'Deny')
  AND client_ip IS NOT NULL AND client_ip != ''
ORDER BY timestamp DESC
LIMIT 5000`

	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, fmt.Errorf("query threat sources: %w", err)
	}
	defer rows.Close()

	var out []ThreatSourceRow
	for rows.Next() {
		var r ThreatSourceRow
		if err := rows.Scan(&r.ClientIP, &r.RiskLevel, &r.Decision, &r.Timestamp); err != nil {
			continue
		}
		if isPrivateIP(r.ClientIP) {
			continue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) DeleteBefore(before time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	const query = `DELETE FROM audit_events WHERE timestamp < ?;`
	result, err := s.db.Exec(query, before)
	if err != nil {
		return 0, fmt.Errorf("delete audit events: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get affected rows: %w", err)
	}

	return affected, nil
}

func (s *SQLiteStore) Checkpoint() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
	if err != nil {
		return fmt.Errorf("wal checkpoint: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func scanAuditEvents(rows *sql.Rows) ([]AuditEvent, error) {
	var events []AuditEvent
	for rows.Next() {
		var ev AuditEvent
		var matchedRulesStr sql.NullString
		var unauthorizedAllow int

		err := rows.Scan(
			&ev.RequestID,
			&ev.Timestamp,
			&ev.GatewayKey,
			&ev.Method,
			&ev.Path,
			&ev.StatusCode,
			&ev.DurationMs,
			&ev.BodyHash,
			&ev.ClientIP,
			&ev.Decision,
			&ev.Reason,
			&ev.GateType,
			&ev.RiskScore,
			&ev.RiskLevel,
			&matchedRulesStr,
			&ev.TokenStatus,
			&ev.AuthMode,
			&unauthorizedAllow,
			&ev.Error,
		)
		if err != nil {
			continue
		}

		if matchedRulesStr.Valid && matchedRulesStr.String != "" {
			ev.MatchedRules = strings.Split(matchedRulesStr.String, ",")
		}
		ev.UnauthorizedAllow = unauthorizedAllow == 1

		events = append(events, ev)
	}

	if events == nil {
		events = []AuditEvent{}
	}

	return events, nil
}
