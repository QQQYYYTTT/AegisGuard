package user

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type tokenClaims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Type     string `json:"type"`
	Expires  int64  `json:"exp"`
}

type Session struct {
	Avatar       string   `json:"avatar"`
	Username     string   `json:"username"`
	Nickname     string   `json:"nickname"`
	Roles        []string `json:"roles"`
	Permissions  []string `json:"permissions"`
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken"`
	Expires      string   `json:"expires"`
}

type Profile struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	CreatedAt string `json:"created_at"`
}

type Service struct {
	repo       *Repository
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewService(repo *Repository, secret string) *Service {
	return &Service{
		repo:       repo,
		secret:     []byte(secret),
		accessTTL:  24 * time.Hour,
		refreshTTL: 7 * 24 * time.Hour,
	}
}

func (s *Service) Register(username, password, nickname string) (*Session, error) {
	username = strings.TrimSpace(username)
	nickname = strings.TrimSpace(nickname)

	if err := validateCredentials(username, password); err != nil {
		return nil, err
	}
	if nickname == "" {
		nickname = username
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	item, err := s.repo.CreateUser(username, string(passwordHash), nickname)
	if err != nil {
		return nil, err
	}

	return s.issueSession(item)
}

func (s *Service) Login(username, password string) (*Session, error) {
	username = strings.TrimSpace(username)
	if username == "" || strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	item, err := s.repo.GetByUsername(username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, fmt.Errorf("invalid username or password")
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(item.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	return s.issueSession(item)
}

func (s *Service) Refresh(refreshToken string) (*Session, error) {
	claims, err := s.verifyToken(refreshToken, "refresh")
	if err != nil {
		return nil, err
	}

	item, err := s.repo.GetByID(claims.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	return s.issueSession(item)
}

func (s *Service) ParseAccessToken(accessToken string) (*Profile, error) {
	claims, err := s.verifyToken(accessToken, "access")
	if err != nil {
		return nil, err
	}

	item, err := s.repo.GetByID(claims.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	return &Profile{
		ID:        item.ID,
		Username:  item.Username,
		Nickname:  item.Nickname,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Service) issueSession(item *User) (*Session, error) {
	accessExpiresAt := time.Now().Add(s.accessTTL)
	refreshExpiresAt := time.Now().Add(s.refreshTTL)

	accessToken, err := s.signToken(tokenClaims{
		UserID:   item.ID,
		Username: item.Username,
		Type:     "access",
		Expires:  accessExpiresAt.Unix(),
	})
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.signToken(tokenClaims{
		UserID:   item.ID,
		Username: item.Username,
		Type:     "refresh",
		Expires:  refreshExpiresAt.Unix(),
	})
	if err != nil {
		return nil, err
	}

	role := "common"
	permissions := []string{"permission:btn:add", "permission:btn:edit"}
	if strings.EqualFold(item.Username, "admin") {
		role = "admin"
		permissions = []string{"*:*:*"}
	}

	return &Session{
		Avatar:       "",
		Username:     item.Username,
		Nickname:     item.Nickname,
		Roles:        []string{role},
		Permissions:  permissions,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expires:      accessExpiresAt.Format(time.RFC3339),
	}, nil
}

func (s *Service) signToken(claims tokenClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal token: %w", err)
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := s.sign(encodedPayload)
	return encodedPayload + "." + signature, nil
}

func (s *Service) verifyToken(token, expectedType string) (*tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token")
	}

	if !hmac.Equal([]byte(parts[1]), []byte(s.sign(parts[0]))) {
		return nil, fmt.Errorf("invalid token signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid token payload")
	}

	var claims tokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("invalid token payload")
	}
	if claims.Type != expectedType {
		return nil, fmt.Errorf("invalid token type")
	}
	if time.Now().Unix() >= claims.Expires {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

func (s *Service) sign(value string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validateCredentials(username, password string) error {
	if len(username) < 3 || len(username) > 32 {
		return fmt.Errorf("username length must be between 3 and 32")
	}
	if len(password) < 6 || len(password) > 64 {
		return fmt.Errorf("password length must be between 6 and 64")
	}
	return nil
}
