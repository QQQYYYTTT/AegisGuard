package auth

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteTokenStoreConfig struct {
	Path        string
	WALMode     bool
	CacheSize   int
	BusyTimeout time.Duration
}

func DefaultSQLiteTokenStoreConfig(path string) SQLiteTokenStoreConfig {
	return SQLiteTokenStoreConfig{
		Path:        path,
		WALMode:     true,
		CacheSize:   64000,
		BusyTimeout: 5 * time.Second,
	}
}

type SQLiteTokenStore struct {
	db *sql.DB
}

func NewSQLiteTokenStore(path string) (*SQLiteTokenStore, error) {
	return NewSQLiteTokenStoreWithConfig(DefaultSQLiteTokenStoreConfig(path))
}

func NewSQLiteTokenStoreWithConfig(cfg SQLiteTokenStoreConfig) (*SQLiteTokenStore, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("sqlite token store path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}

	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := configureSQLiteTokenPragmas(db, cfg); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure pragma: %w", err)
	}
	if err := migrateSQLiteTokenStore(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate token store: %w", err)
	}
	return &SQLiteTokenStore{db: db}, nil
}

func configureSQLiteTokenPragmas(db *sql.DB, cfg SQLiteTokenStoreConfig) error {
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

func migrateSQLiteTokenStore(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS token_families (
	family_key TEXT PRIMARY KEY,
	token_id TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS require_tokens (
	token_id TEXT PRIMARY KEY,
	tool_name TEXT NOT NULL,
	scope TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	task_id TEXT NOT NULL,
	expires_at DATETIME NOT NULL,
	nonce TEXT NOT NULL,
	risk_level INTEGER NOT NULL DEFAULT 0,
	schema_hash TEXT NOT NULL DEFAULT '',
	max_calls INTEGER NOT NULL DEFAULT 0,
	call_count INTEGER NOT NULL DEFAULT 0,
	signature TEXT NOT NULL DEFAULT '',
	revoked INTEGER NOT NULL DEFAULT 0,
	latest_seen_at DATETIME NOT NULL,
	created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_require_tokens_active ON require_tokens(revoked, expires_at, latest_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_require_tokens_latest ON require_tokens(latest_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_require_tokens_agent_session_task ON require_tokens(agent_id, session_id, task_id);
`
	_, err := db.Exec(schema)
	return err
}

func (s *SQLiteTokenStore) Issue(toolName, scope, agentID, sessionID, taskID string, ttl time.Duration, maxCalls int) (*RequireToken, error) {
	token, err := NewToken(toolName, scope, agentID, sessionID, taskID, ttl, maxCalls)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	familyKey := tokenFamilyKey(toolName, scope, agentID, sessionID, taskID, maxCalls)

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin issue tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if reused, err := loadReusableToken(tx, familyKey); err != nil {
		return nil, err
	} else if reused != nil {
		token.TokenID = reused.TokenID
		token.MaxCalls = reused.MaxCalls
		token.CallCount = reused.CallCount
		token.ExpiresAt = reused.ExpiresAt
		if err := token.Sign(); err != nil {
			return nil, err
		}
	}

	if err := upsertToken(tx, token, false, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`
INSERT INTO token_families (family_key, token_id) VALUES (?, ?)
ON CONFLICT(family_key) DO UPDATE SET token_id=excluded.token_id;
`, familyKey, token.TokenID); err != nil {
		return nil, fmt.Errorf("upsert token family: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit issue tx: %w", err)
	}
	tx = nil
	return s.GetByID(token.TokenID)
}

func loadReusableToken(tx *sql.Tx, familyKey string) (*RequireToken, error) {
	const query = `
SELECT t.token_id, t.tool_name, t.scope, t.agent_id, t.session_id, t.task_id,
       t.expires_at, t.nonce, t.risk_level, t.schema_hash, t.max_calls, t.call_count,
       t.signature
FROM token_families f
JOIN require_tokens t ON t.token_id = f.token_id
WHERE f.family_key = ?
  AND t.revoked = 0
  AND t.expires_at > ?
LIMIT 1;`
	row := tx.QueryRow(query, familyKey, time.Now())
	token, revoked, err := scanTokenRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("load reusable token: %w", err)
	}
	if revoked {
		return nil, nil
	}
	return token, nil
}

func (s *SQLiteTokenStore) Save(token *RequireToken) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}

	current, err := s.getTokenRecord(token.TokenID)
	if err != nil {
		return err
	}
	token.CallCount = current.CallCount
	token.MaxCalls = current.MaxCalls
	token.ExpiresAt = current.ExpiresAt

	_, err = s.db.Exec(`
UPDATE require_tokens
SET tool_name = ?, scope = ?, agent_id = ?, session_id = ?, task_id = ?,
    expires_at = ?, nonce = ?, risk_level = ?, schema_hash = ?, max_calls = ?,
    call_count = ?, signature = ?, latest_seen_at = ?
WHERE token_id = ? AND revoked = 0;
`,
		token.ToolName,
		token.Scope,
		token.AgentID,
		token.SessionID,
		token.TaskID,
		token.ExpiresAt,
		token.Nonce,
		token.RiskLevel,
		token.SchemaHash,
		token.MaxCalls,
		token.CallCount,
		token.Signature,
		time.Now(),
		token.TokenID,
	)
	if err != nil {
		return fmt.Errorf("update token: %w", err)
	}
	return nil
}

func (s *SQLiteTokenStore) Revoke(tokenID string) error {
	result, err := s.db.Exec(`
UPDATE require_tokens
SET revoked = 1, latest_seen_at = ?
WHERE token_id = ? AND revoked = 0;
`, time.Now(), tokenID)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke token rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("token not found: %s", tokenID)
	}
	return nil
}

func (s *SQLiteTokenStore) ListActive() []RequireToken {
	rows, err := s.db.Query(`
SELECT token_id, tool_name, scope, agent_id, session_id, task_id,
       expires_at, nonce, risk_level, schema_hash, max_calls, call_count,
       signature, revoked
FROM require_tokens
WHERE revoked = 0 AND expires_at > ?
ORDER BY latest_seen_at DESC;
`, time.Now())
	if err != nil {
		return []RequireToken{}
	}
	defer rows.Close()

	var out []RequireToken
	for rows.Next() {
		token, revoked, err := scanTokenRows(rows)
		if err != nil || revoked {
			continue
		}
		out = append(out, *token)
	}
	if out == nil {
		return []RequireToken{}
	}
	return out
}

func (s *SQLiteTokenStore) GetByID(tokenID string) (*RequireToken, error) {
	token, err := s.getTokenRecord(tokenID)
	if err != nil {
		return nil, err
	}
	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("token expired: %s", tokenID)
	}
	return token, nil
}

func (s *SQLiteTokenStore) getTokenRecord(tokenID string) (*RequireToken, error) {
	row := s.db.QueryRow(`
SELECT token_id, tool_name, scope, agent_id, session_id, task_id,
       expires_at, nonce, risk_level, schema_hash, max_calls, call_count,
       signature, revoked
FROM require_tokens
WHERE token_id = ?
LIMIT 1;
`, tokenID)
	token, revoked, err := scanTokenRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("token not found: %s", tokenID)
		}
		return nil, err
	}
	if revoked {
		return nil, fmt.Errorf("token revoked: %s", tokenID)
	}
	return token, nil
}

func (s *SQLiteTokenStore) GetLatest() (*RequireToken, error) {
	row := s.db.QueryRow(`
SELECT token_id, tool_name, scope, agent_id, session_id, task_id,
       expires_at, nonce, risk_level, schema_hash, max_calls, call_count,
       signature, revoked
FROM require_tokens
WHERE revoked = 0 AND expires_at > ?
ORDER BY latest_seen_at DESC
LIMIT 1;
`, time.Now())
	token, revoked, err := scanTokenRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active token")
		}
		return nil, err
	}
	if revoked {
		return nil, fmt.Errorf("no active token")
	}
	return token, nil
}

func (s *SQLiteTokenStore) ActiveCount() int {
	var count int
	if err := s.db.QueryRow(`
SELECT COUNT(*)
FROM require_tokens
WHERE revoked = 0 AND expires_at > ?;
`, time.Now()).Scan(&count); err != nil {
		return 0
	}
	return count
}

func (s *SQLiteTokenStore) RevokedCount() int {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM require_tokens WHERE revoked = 1;`).Scan(&count); err != nil {
		return 0
	}
	return count
}

func (s *SQLiteTokenStore) Consume(tokenID string) (*RequireToken, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin consume tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	row := tx.QueryRow(`
SELECT token_id, tool_name, scope, agent_id, session_id, task_id,
       expires_at, nonce, risk_level, schema_hash, max_calls, call_count,
       signature, revoked
FROM require_tokens
WHERE token_id = ?
LIMIT 1;
`, tokenID)
	token, revoked, err := scanTokenRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("token not found: %s", tokenID)
		}
		return nil, err
	}
	if revoked {
		return nil, fmt.Errorf("token revoked: %s", tokenID)
	}
	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("token expired: %s", tokenID)
	}
	if token.MaxCalls > 0 && token.CallCount >= token.MaxCalls {
		return nil, fmt.Errorf("call budget exceeded: %d/%d calls used", token.CallCount, token.MaxCalls)
	}

	token.CallCount++
	if _, err := tx.Exec(`
UPDATE require_tokens
SET call_count = ?, latest_seen_at = ?
WHERE token_id = ?;
`, token.CallCount, time.Now(), tokenID); err != nil {
		return nil, fmt.Errorf("update consumed token: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit consume tx: %w", err)
	}
	tx = nil
	return token, nil
}

func (s *SQLiteTokenStore) State(tokenID string) (*RequireToken, error) {
	return s.getTokenRecord(tokenID)
}

func (s *SQLiteTokenStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func upsertToken(tx *sql.Tx, token *RequireToken, revoked bool, now time.Time) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	revokedInt := 0
	if revoked {
		revokedInt = 1
	}
	_, err := tx.Exec(`
INSERT INTO require_tokens (
	token_id, tool_name, scope, agent_id, session_id, task_id,
	expires_at, nonce, risk_level, schema_hash, max_calls, call_count,
	signature, revoked, latest_seen_at, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(token_id) DO UPDATE SET
	tool_name = excluded.tool_name,
	scope = excluded.scope,
	agent_id = excluded.agent_id,
	session_id = excluded.session_id,
	task_id = excluded.task_id,
	expires_at = excluded.expires_at,
	nonce = excluded.nonce,
	risk_level = excluded.risk_level,
	schema_hash = excluded.schema_hash,
	max_calls = excluded.max_calls,
	call_count = excluded.call_count,
	signature = excluded.signature,
	revoked = excluded.revoked,
	latest_seen_at = excluded.latest_seen_at;
`,
		token.TokenID,
		token.ToolName,
		token.Scope,
		token.AgentID,
		token.SessionID,
		token.TaskID,
		token.ExpiresAt,
		token.Nonce,
		token.RiskLevel,
		token.SchemaHash,
		token.MaxCalls,
		token.CallCount,
		token.Signature,
		revokedInt,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("upsert token: %w", err)
	}
	return nil
}

type tokenScanner interface {
	Scan(dest ...any) error
}

func scanTokenRow(scanner tokenScanner) (*RequireToken, bool, error) {
	var token RequireToken
	var revoked int
	err := scanner.Scan(
		&token.TokenID,
		&token.ToolName,
		&token.Scope,
		&token.AgentID,
		&token.SessionID,
		&token.TaskID,
		&token.ExpiresAt,
		&token.Nonce,
		&token.RiskLevel,
		&token.SchemaHash,
		&token.MaxCalls,
		&token.CallCount,
		&token.Signature,
		&revoked,
	)
	if err != nil {
		return nil, false, err
	}
	return &token, revoked == 1, nil
}

func scanTokenRows(rows *sql.Rows) (*RequireToken, bool, error) {
	return scanTokenRow(rows)
}
