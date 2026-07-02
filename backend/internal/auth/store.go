package auth

import (
	"fmt"
	"sync"
	"time"
)

// TokenStore 定义 RequireToken 的持久化存储契约。
// 读写路径统一经由该接口，便于在内存实现与 SQLite 实现之间切换。
type TokenStore interface {
	Issue(toolName, scope, agentID, sessionID, taskID string, ttl time.Duration, maxCalls int) (*RequireToken, error)
	Save(token *RequireToken) error
	Revoke(tokenID string) error
	ListActive() []RequireToken
	GetByID(tokenID string) (*RequireToken, error)
	GetLatest() (*RequireToken, error)
	ActiveCount() int
	RevokedCount() int
	Consume(tokenID string) (*RequireToken, error)
	State(tokenID string) (*RequireToken, error)
	Close() error
}

// MemoryTokenStore 提供最小可运行的 RequireToken 内存实现。
// 适合本地联调和 SQLite 不可用时的降级回退。
type MemoryTokenStore struct {
	mu      sync.RWMutex
	active  map[string]*RequireToken
	states  map[string]*tokenState
	family  map[string]string
	revoked map[string]time.Time
	latest  string
}

func NewTokenStore() TokenStore {
	return NewMemoryTokenStore()
}

func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{
		active:  make(map[string]*RequireToken),
		states:  make(map[string]*tokenState),
		family:  make(map[string]string),
		revoked: make(map[string]time.Time),
	}
}

func (s *MemoryTokenStore) Issue(toolName, scope, agentID, sessionID, taskID string, ttl time.Duration, maxCalls int) (*RequireToken, error) {
	token, err := NewToken(toolName, scope, agentID, sessionID, taskID, ttl, maxCalls)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	familyKey := tokenFamilyKey(toolName, scope, agentID, sessionID, taskID, maxCalls)
	if tokenID, ok := s.family[familyKey]; ok {
		if state, exists := s.states[tokenID]; exists && !state.Revoked && time.Now().Before(state.ExpiresAt) {
			token.TokenID = tokenID
			token.MaxCalls = state.MaxCalls
			token.CallCount = state.CallCount
			token.ExpiresAt = state.ExpiresAt
			if err := token.Sign(); err != nil {
				return nil, err
			}
		} else {
			delete(s.family, familyKey)
		}
	}

	tokenCopy := *token
	s.active[token.TokenID] = &tokenCopy
	s.latest = token.TokenID

	if _, ok := s.states[token.TokenID]; !ok {
		s.states[token.TokenID] = &tokenState{
			TokenID:   token.TokenID,
			ToolName:  toolName,
			Scope:     scope,
			AgentID:   agentID,
			SessionID: sessionID,
			TaskID:    taskID,
			ExpiresAt: token.ExpiresAt,
			MaxCalls:  maxCalls,
			CallCount: token.CallCount,
		}
	}
	s.family[familyKey] = token.TokenID

	return s.active[token.TokenID], nil
}

func (s *MemoryTokenStore) Save(token *RequireToken) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.active[token.TokenID]
	if !ok {
		return fmt.Errorf("token not found: %s", token.TokenID)
	}

	state, ok := s.states[token.TokenID]
	if !ok {
		return fmt.Errorf("token state not found: %s", token.TokenID)
	}

	tokenCopy := *token
	tokenCopy.CallCount = state.CallCount
	tokenCopy.MaxCalls = state.MaxCalls
	tokenCopy.ExpiresAt = state.ExpiresAt
	*stored = tokenCopy
	s.latest = token.TokenID
	return nil
}

func (s *MemoryTokenStore) Revoke(tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.active[tokenID]; !ok {
		return fmt.Errorf("token not found: %s", tokenID)
	}

	delete(s.active, tokenID)
	if state, ok := s.states[tokenID]; ok {
		state.Revoked = true
	}
	s.revoked[tokenID] = time.Now()
	if s.latest == tokenID {
		s.latest = ""
	}

	return nil
}

func (s *MemoryTokenStore) ListActive() []RequireToken {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	tokens := make([]RequireToken, 0, len(s.active))
	for tokenID, token := range s.active {
		state := s.states[tokenID]
		if state == nil {
			delete(s.active, tokenID)
			continue
		}
		if state.Revoked || now.After(state.ExpiresAt) {
			delete(s.active, tokenID)
			if s.latest == tokenID {
				s.latest = ""
			}
			continue
		}
		token.CallCount = state.CallCount
		token.ExpiresAt = state.ExpiresAt
		tokens = append(tokens, *token)
	}

	return tokens
}

func (s *MemoryTokenStore) GetByID(tokenID string) (*RequireToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, ok := s.active[tokenID]
	if !ok {
		return nil, fmt.Errorf("token not found: %s", tokenID)
	}
	state := s.states[tokenID]
	if state == nil {
		delete(s.active, tokenID)
		return nil, fmt.Errorf("token state not found: %s", tokenID)
	}
	if state.Revoked {
		delete(s.active, tokenID)
		return nil, fmt.Errorf("token revoked: %s", tokenID)
	}
	if time.Now().After(state.ExpiresAt) {
		delete(s.active, tokenID)
		if s.latest == tokenID {
			s.latest = ""
		}
		return nil, fmt.Errorf("token expired: %s", tokenID)
	}

	tokenCopy := *token
	tokenCopy.CallCount = state.CallCount
	tokenCopy.ExpiresAt = state.ExpiresAt
	return &tokenCopy, nil
}

func (s *MemoryTokenStore) GetLatest() (*RequireToken, error) {
	s.mu.RLock()
	latest := s.latest
	s.mu.RUnlock()

	if latest == "" {
		return nil, fmt.Errorf("no active token")
	}

	return s.GetByID(latest)
}

func (s *MemoryTokenStore) ActiveCount() int {
	return len(s.ListActive())
}

func (s *MemoryTokenStore) RevokedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.revoked)
}

func (s *MemoryTokenStore) Consume(tokenID string) (*RequireToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, ok := s.active[tokenID]
	if !ok {
		return nil, fmt.Errorf("token not found: %s", tokenID)
	}
	state, ok := s.states[tokenID]
	if !ok {
		return nil, fmt.Errorf("token state not found: %s", tokenID)
	}
	if state.Revoked {
		return nil, fmt.Errorf("token revoked: %s", tokenID)
	}
	if time.Now().After(state.ExpiresAt) {
		return nil, fmt.Errorf("token expired: %s", tokenID)
	}
	if state.MaxCalls > 0 && state.CallCount >= state.MaxCalls {
		return nil, fmt.Errorf("call budget exceeded: %d/%d calls used", state.CallCount, state.MaxCalls)
	}

	state.CallCount++
	token.CallCount = state.CallCount
	token.ExpiresAt = state.ExpiresAt

	tokenCopy := *token
	return &tokenCopy, nil
}

func (s *MemoryTokenStore) State(tokenID string) (*RequireToken, error) {
	return s.GetByID(tokenID)
}

func (s *MemoryTokenStore) Close() error {
	return nil
}
