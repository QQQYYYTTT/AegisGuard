package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type tokenState struct {
	TokenID   string
	ToolName  string
	Scope     string
	AgentID   string
	SessionID string
	TaskID    string
	ExpiresAt time.Time
	MaxCalls  int
	CallCount int
	Revoked   bool
}

func newTokenID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate token id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func tokenFamilyKey(toolName, scope, agentID, sessionID, taskID string, maxCalls int) string {
	return strings.Join([]string{
		toolName,
		scope,
		agentID,
		sessionID,
		taskID,
		fmt.Sprintf("%d", maxCalls),
	}, "|")
}
