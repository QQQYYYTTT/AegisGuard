package auth

import (
	"fmt"
	"sync"
	"time"
)

// TokenStore 提供最小可运行的 RequireToken 存储与签发能力。
// 当前使用内存实现，便于本地联调和页面演示。
type TokenStore struct {
	mu      sync.RWMutex
	active  map[string]*RequireToken
	revoked map[string]time.Time
	latest  string
}

func NewTokenStore() *TokenStore {
	return &TokenStore{
		active:  make(map[string]*RequireToken),
		revoked: make(map[string]time.Time),
	}
}

func (s *TokenStore) Issue(toolName, scope, agentID, sessionID, taskID string, ttl time.Duration, maxCalls int) (*RequireToken, error) {
	token, err := NewToken(toolName, scope, agentID, sessionID, taskID, ttl, maxCalls)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tokenCopy := *token
	s.active[token.Nonce] = &tokenCopy
	s.latest = token.Nonce

	return token, nil
}

func (s *TokenStore) Revoke(tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.active[tokenID]; !ok {
		return fmt.Errorf("token not found: %s", tokenID)
	}

	delete(s.active, tokenID)
	s.revoked[tokenID] = time.Now()
	if s.latest == tokenID {
		s.latest = ""
	}

	return nil
}

func (s *TokenStore) ListActive() []RequireToken {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	tokens := make([]RequireToken, 0, len(s.active))
	for tokenID, token := range s.active {
		if now.After(token.ExpiresAt) {
			delete(s.active, tokenID)
			if s.latest == tokenID {
				s.latest = ""
			}
			continue
		}
		tokens = append(tokens, *token)
	}

	return tokens
}

func (s *TokenStore) GetByID(tokenID string) (*RequireToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, ok := s.active[tokenID]
	if !ok {
		return nil, fmt.Errorf("token not found: %s", tokenID)
	}
	if time.Now().After(token.ExpiresAt) {
		delete(s.active, tokenID)
		if s.latest == tokenID {
			s.latest = ""
		}
		return nil, fmt.Errorf("token expired: %s", tokenID)
	}

	tokenCopy := *token
	return &tokenCopy, nil
}

func (s *TokenStore) GetLatest() (*RequireToken, error) {
	s.mu.RLock()
	latest := s.latest
	s.mu.RUnlock()

	if latest == "" {
		return nil, fmt.Errorf("no active token")
	}

	return s.GetByID(latest)
}

func (s *TokenStore) ActiveCount() int {
	return len(s.ListActive())
}

func (s *TokenStore) RevokedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.revoked)
}
