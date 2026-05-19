package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteConfig struct {
	DSN         string
	WALMode     bool
	CacheSize   int
	BusyTimeout time.Duration
}

func DefaultSQLiteConfig(dsn string) SQLiteConfig {
	return SQLiteConfig{
		DSN:         dsn,
		WALMode:     true,
		CacheSize:   64000,
		BusyTimeout: 5 * time.Second,
	}
}

func OpenSQLite(dsn string) (*sql.DB, error) {
	return OpenSQLiteWithConfig(DefaultSQLiteConfig(dsn))
}

func OpenSQLiteWithConfig(cfg SQLiteConfig) (*sql.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("sqlite dsn is empty")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DSN), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}

	conn, err := sql.Open("sqlite", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := configurePragma(conn, cfg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("configure pragma: %w", err)
	}

	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func configurePragma(conn *sql.DB, cfg SQLiteConfig) error {
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
		if _, err := conn.Exec(pragma); err != nil {
			return fmt.Errorf("exec pragma %s: %w", pragma, err)
		}
	}

	return nil
}

func migrate(conn *sql.DB) error {
	const usersTable = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	nickname TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

	const usersIndex = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);`

	if _, err := conn.Exec(usersTable); err != nil {
		return fmt.Errorf("create users table: %w", err)
	}
	if _, err := conn.Exec(usersIndex); err != nil {
		return fmt.Errorf("create users index: %w", err)
	}

	return nil
}
