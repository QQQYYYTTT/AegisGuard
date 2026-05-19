package user

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Nickname     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(username, passwordHash, nickname string) (*User, error) {
	now := time.Now()
	result, err := r.db.Exec(
		`INSERT INTO users (username, password_hash, nickname, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		username,
		passwordHash,
		nickname,
		now,
		now,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, fmt.Errorf("username already exists")
		}
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func (r *Repository) GetByUsername(username string) (*User, error) {
	return r.queryOne(
		`SELECT id, username, password_hash, nickname, created_at, updated_at FROM users WHERE username = ?`,
		username,
	)
}

func (r *Repository) GetByID(id int64) (*User, error) {
	return r.queryOne(
		`SELECT id, username, password_hash, nickname, created_at, updated_at FROM users WHERE id = ?`,
		id,
	)
}

func (r *Repository) queryOne(query string, args ...any) (*User, error) {
	var item User
	err := r.db.QueryRow(query, args...).Scan(
		&item.ID,
		&item.Username,
		&item.PasswordHash,
		&item.Nickname,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &item, nil
}
