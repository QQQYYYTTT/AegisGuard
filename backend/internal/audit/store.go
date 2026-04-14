package audit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Store struct {
	file string
}

func NewStore(file string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(file, []byte("[]"), 0o644); err != nil {
			return nil, err
		}
	}
	return &Store{file: file}, nil
}

func (s *Store) ReadAll() ([]map[string]any, error) {
	raw, err := os.ReadFile(s.file)
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	if len(raw) == 0 {
		return []map[string]any{}, nil
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return []map[string]any{}, nil
	}
	return items, nil
}

func (s *Store) WriteAll(items []map[string]any) error {
	raw, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.file, raw, 0o644)
}
