package toolmeta

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
)

type Metadata struct {
	ToolName     string `json:"tool_name"`
	Schema       string `json:"schema,omitempty"`
	SchemaBase64 bool   `json:"schema_base64,omitempty"`
}

type Store struct {
	mu      sync.RWMutex
	entries map[string]Metadata
}

func NewStore() *Store {
	return &Store{
		entries: make(map[string]Metadata),
	}
}

func NewStoreFromFile(path string) (*Store, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return NewStore(), nil
	}
	data, err := os.ReadFile(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewStore(), nil
		}
		return nil, err
	}
	store := NewStore()
	if len(strings.TrimSpace(string(data))) == 0 {
		return store, nil
	}

	var items []Metadata
	if err := json.Unmarshal(data, &items); err == nil {
		for _, item := range items {
			store.Set(item)
		}
		return store, nil
	}

	var byName map[string]Metadata
	if err := json.Unmarshal(data, &byName); err != nil {
		return nil, err
	}
	for key, item := range byName {
		if strings.TrimSpace(item.ToolName) == "" {
			item.ToolName = key
		}
		store.Set(item)
	}
	return store, nil
}

func (s *Store) Set(item Metadata) {
	toolName := strings.TrimSpace(item.ToolName)
	if toolName == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[toolName] = Metadata{
		ToolName:     toolName,
		Schema:       item.Schema,
		SchemaBase64: item.SchemaBase64,
	}
}

func (s *Store) Schema(toolName string) ([]byte, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	item, ok := s.entries[strings.TrimSpace(toolName)]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	raw := strings.TrimSpace(item.Schema)
	if raw == "" {
		return nil, false
	}
	if item.SchemaBase64 {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, false
		}
		return decoded, true
	}
	return []byte(raw), true
}
