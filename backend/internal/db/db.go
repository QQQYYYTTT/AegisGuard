package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func OpenSQLite(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("sqlite dsn is empty")
	}

	if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
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
