package vkey

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("virtual key not found")

type Store interface {
	Save(ctx context.Context, key *VirtualKey) error
	Get(ctx context.Context, keyID string) (*VirtualKey, error)
	Delete(ctx context.Context, keyID string) error
	List(ctx context.Context) ([]*VirtualKey, error)
	Update(ctx context.Context, key *VirtualKey) error
}

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.init(); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return s, nil
}

func (s *SQLiteStore) init() error {
	schema := `CREATE TABLE IF NOT EXISTS virtual_keys (
		key_id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		real_api_key TEXT NOT NULL,
		scope TEXT NOT NULL DEFAULT 'read',
		session_id TEXT,
		rate_limit INTEGER DEFAULT 0,
		expires_at TEXT,
		created_at TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1
	);`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) Save(ctx context.Context, key *VirtualKey) error {
	query := `INSERT OR REPLACE INTO virtual_keys
		(key_id, agent_id, real_api_key, scope, session_id, rate_limit, expires_at, created_at, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query,
		key.KeyID, key.AgentID, key.RealAPIKey, key.Scope, key.SessionID,
		key.RateLimit, key.ExpiresAt, key.CreatedAt, boolToInt(key.Enabled))
	return err
}

func (s *SQLiteStore) Get(ctx context.Context, keyID string) (*VirtualKey, error) {
	query := `SELECT key_id, agent_id, real_api_key, scope, session_id, rate_limit, expires_at, created_at, enabled
		FROM virtual_keys WHERE key_id = ?`
	row := s.db.QueryRowContext(ctx, query, keyID)
	var vk VirtualKey
	var sessionID, expiresAt sql.NullString
	var enabled int
	err := row.Scan(&vk.KeyID, &vk.AgentID, &vk.RealAPIKey, &vk.Scope,
		&sessionID, &vk.RateLimit, &expiresAt, &vk.CreatedAt, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	vk.SessionID = sessionID.String
	vk.ExpiresAt = expiresAt.String
	vk.Enabled = intToBool(enabled)
	return &vk, nil
}

func (s *SQLiteStore) Delete(ctx context.Context, keyID string) error {
	query := `DELETE FROM virtual_keys WHERE key_id = ?`
	result, err := s.db.ExecContext(ctx, query, keyID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) List(ctx context.Context) ([]*VirtualKey, error) {
	query := `SELECT key_id, agent_id, real_api_key, scope, session_id, rate_limit, expires_at, created_at, enabled
		FROM virtual_keys ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*VirtualKey
	for rows.Next() {
		var vk VirtualKey
		var sessionID, expiresAt sql.NullString
		var enabled int
		err := rows.Scan(&vk.KeyID, &vk.AgentID, &vk.RealAPIKey, &vk.Scope,
			&sessionID, &vk.RateLimit, &expiresAt, &vk.CreatedAt, &enabled)
		if err != nil {
			return nil, err
		}
		vk.SessionID = sessionID.String
		vk.ExpiresAt = expiresAt.String
		vk.Enabled = intToBool(enabled)
		keys = append(keys, &vk)
	}
	return keys, rows.Err()
}

func (s *SQLiteStore) Update(ctx context.Context, key *VirtualKey) error {
	query := `UPDATE virtual_keys SET agent_id=?, real_api_key=?, scope=?, session_id=?,
		rate_limit=?, expires_at=?, created_at=?, enabled=? WHERE key_id=?`
	result, err := s.db.ExecContext(ctx, query,
		key.AgentID, key.RealAPIKey, key.Scope, key.SessionID,
		key.RateLimit, key.ExpiresAt, key.CreatedAt, boolToInt(key.Enabled), key.KeyID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

type InMemoryStore struct {
	keys map[string]*VirtualKey
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{keys: make(map[string]*VirtualKey)}
}

func (s *InMemoryStore) Save(ctx context.Context, key *VirtualKey) error {
	s.keys[key.KeyID] = key
	return nil
}

func (s *InMemoryStore) Get(ctx context.Context, keyID string) (*VirtualKey, error) {
	key, ok := s.keys[keyID]
	if !ok {
		return nil, ErrNotFound
	}
	return key, nil
}

func (s *InMemoryStore) Delete(ctx context.Context, keyID string) error {
	if _, ok := s.keys[keyID]; !ok {
		return ErrNotFound
	}
	delete(s.keys, keyID)
	return nil
}

func (s *InMemoryStore) List(ctx context.Context) ([]*VirtualKey, error) {
	keys := make([]*VirtualKey, 0, len(s.keys))
	for _, k := range s.keys {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *InMemoryStore) Update(ctx context.Context, key *VirtualKey) error {
	if _, ok := s.keys[key.KeyID]; !ok {
		return ErrNotFound
	}
	s.keys[key.KeyID] = key
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}

type StoreFactory func(path string) (Store, error)

var storeFactories = map[string]StoreFactory{
	"sqlite": func(path string) (Store, error) {
		return NewSQLiteStore(path)
	},
	"memory": func(_ string) (Store, error) {
		return NewInMemoryStore(), nil
	},
}

func RegisterStoreFactory(scheme string, factory StoreFactory) {
	storeFactories[scheme] = factory
}

func CreateStore(dsn string) (Store, error) {
	scheme := "sqlite"
	path := dsn

	if idx := strings.Index(dsn, ":"); idx > 0 {
		candidate := dsn[:idx]
		// 跨平台兼容：Linux 的 scheme 是纯字母，Windows 盘符是单字母（如 D:）
		// 如果冒号前只有一个字母，可能是 Windows 盘符，不作为 scheme 处理
		if len(candidate) > 1 {
			scheme = candidate
			path = dsn[idx+1:]
		}
	}

	if f, ok := storeFactories[scheme]; ok {
		return f(path)
	}
	return nil, fmt.Errorf("unsupported store scheme: %s", scheme)
}
