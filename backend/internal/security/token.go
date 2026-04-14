package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const demoSecret = "aegisguard-demo-secret"

type TokenPayload struct {
	ToolName      string `json:"tool_name"`
	Scope         string `json:"scope"`
	AgentID       string `json:"agent_id"`
	SessionID     string `json:"session_id"`
	TaskID        string `json:"task_id"`
	ExpiresAt     string `json:"expires_at"`
	Nonce         string `json:"nonce"`
	RiskLevel     string `json:"risk_level"`
	SchemaHashSM3 string `json:"schema_hash_sm3"`
}

type RequireToken struct {
	TokenPayload
	SM2Signature string `json:"sm2_signature"`
}

type IssueTokenRequest struct {
	ToolName  string `json:"toolName"`
	Scope     string `json:"scope"`
	AgentID   string `json:"agentId"`
	SessionID string `json:"sessionId"`
	TaskID    string `json:"taskId"`
}

type VerifyRequestBody struct {
	ScenarioID     string       `json:"scenarioId"`
	UserGoal       string       `json:"userGoal"`
	AgentID        string       `json:"agentId"`
	SessionID      string       `json:"sessionId"`
	TaskID         string       `json:"taskId"`
	ToolName       string       `json:"toolName"`
	Scope          string       `json:"scope"`
	RequestedScope string       `json:"requestedScope"`
	RawResult      string       `json:"rawResult"`
	Token          RequireToken `json:"token"`
}

type Verification struct {
	SignatureValid bool `json:"signatureValid"`
	NotExpired     bool `json:"notExpired"`
	SessionMatch   bool `json:"sessionMatch"`
	ScopeAllowed   bool `json:"scopeAllowed"`
	AgentMatch     bool `json:"agentMatch"`
	SchemaMatch    bool `json:"schemaMatch"`
	AllPassed      bool `json:"allPassed"`
}

func SimpleHash(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])[:16]
}

func signPayload(payload TokenPayload) string {
	raw, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, []byte(demoSecret))
	_, _ = mac.Write(raw)
	return fmt.Sprintf("sm2-sim-%s", hex.EncodeToString(mac.Sum(nil))[:20])
}

func IssueRequireToken(body IssueTokenRequest) RequireToken {
	taskID := body.TaskID
	if taskID == "" {
		taskID = fmt.Sprintf("task-%d", time.Now().UnixMilli()%1000000)
	}

	payload := TokenPayload{
		ToolName:      body.ToolName,
		Scope:         body.Scope,
		AgentID:       body.AgentID,
		SessionID:     body.SessionID,
		TaskID:        taskID,
		ExpiresAt:     time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339),
		Nonce:         fmt.Sprintf("nonce-%s", SimpleHash(fmt.Sprintf("%s:%d", body.AgentID, time.Now().UnixMilli()))),
		RiskLevel:     "medium",
		SchemaHashSM3: SimpleHash(fmt.Sprintf("%s:%s", body.ToolName, body.Scope)),
	}

	return RequireToken{
		TokenPayload:  payload,
		SM2Signature: signPayload(payload),
	}
}

func VerifyRequireToken(token RequireToken, body VerifyRequestBody) Verification {
	recomputed := signPayload(TokenPayload{
		ToolName:      token.ToolName,
		Scope:         token.Scope,
		AgentID:       token.AgentID,
		SessionID:     token.SessionID,
		TaskID:        token.TaskID,
		ExpiresAt:     token.ExpiresAt,
		Nonce:         token.Nonce,
		RiskLevel:     token.RiskLevel,
		SchemaHashSM3: token.SchemaHashSM3,
	})

	expiresAt, err := time.Parse(time.RFC3339, token.ExpiresAt)
	notExpired := err == nil && expiresAt.After(time.Now())
	scopeAllowed := body.RequestedScope == token.Scope
	if !scopeAllowed && len(body.RequestedScope) >= len(token.Scope) {
		scopeAllowed = body.RequestedScope[:len(token.Scope)] == token.Scope
	}

	verification := Verification{
		SignatureValid: token.SM2Signature == recomputed,
		NotExpired:     notExpired,
		SessionMatch:   token.SessionID == body.SessionID,
		ScopeAllowed:   scopeAllowed,
		AgentMatch:     token.AgentID == body.AgentID,
		SchemaMatch:    token.SchemaHashSM3 == SimpleHash(fmt.Sprintf("%s:%s", token.ToolName, token.Scope)),
	}
	verification.AllPassed = verification.SignatureValid && verification.NotExpired && verification.SessionMatch && verification.ScopeAllowed && verification.AgentMatch && verification.SchemaMatch
	return verification
}
