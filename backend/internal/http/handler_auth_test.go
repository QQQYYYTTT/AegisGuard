package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aegisguard/internal/auth"

	"github.com/gin-gonic/gin"
)

func TestAuthHandlersIssueVerifyAndStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth.ResetNonces()
	if err := auth.InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}

	router := &Router{
		engine:     gin.New(),
		tokenStore: auth.NewTokenStore(),
		verifier:   auth.NewVerifier(),
	}
	router.registerAuthRoutes()

	issueBody := map[string]interface{}{
		"tool_name":   "read_file",
		"scope":       "read_file:invoke",
		"agent_id":    "agent-test",
		"session_id":  "session-test",
		"task_id":     "task-test",
		"ttl_seconds": 60,
		"max_calls":   2,
	}
	payload, _ := json.Marshal(issueBody)

	issueReq := httptest.NewRequest(http.MethodPost, "/aegis/auth/token", bytes.NewReader(payload))
	issueReq.Header.Set("Content-Type", "application/json")
	issueRec := httptest.NewRecorder()
	router.engine.ServeHTTP(issueRec, issueReq)
	if issueRec.Code != http.StatusOK {
		t.Fatalf("issue token status = %d, body = %s", issueRec.Code, issueRec.Body.String())
	}

	var issueResp struct {
		Success bool `json:"success"`
		Data    struct {
			TokenID  string `json:"token_id"`
			ToolName string `json:"tool_name"`
			Signed   bool   `json:"signed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(issueRec.Body.Bytes(), &issueResp); err != nil {
		t.Fatalf("decode issue response: %v", err)
	}
	if !issueResp.Success || issueResp.Data.TokenID == "" || !issueResp.Data.Signed {
		t.Fatalf("unexpected issue response: %+v", issueResp)
	}

	verifyReq := httptest.NewRequest(http.MethodPost, "/aegis/auth/verify", nil)
	verifyRec := httptest.NewRecorder()
	router.engine.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify token status = %d, body = %s", verifyRec.Code, verifyRec.Body.String())
	}

	var verifyResp struct {
		Success bool `json:"success"`
		Data    struct {
			Valid  bool            `json:"valid"`
			Checks map[string]bool `json:"checks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(verifyRec.Body.Bytes(), &verifyResp); err != nil {
		t.Fatalf("decode verify response: %v", err)
	}
	if !verifyResp.Success || !verifyResp.Data.Valid || !verifyResp.Data.Checks["signature_valid"] {
		t.Fatalf("unexpected verify response: %+v", verifyResp)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/aegis/auth/status", nil)
	statusRec := httptest.NewRecorder()
	router.engine.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("auth status status = %d, body = %s", statusRec.Code, statusRec.Body.String())
	}

	var statusResp struct {
		Success bool `json:"success"`
		Data    struct {
			SM2Active    bool `json:"sm2_active"`
			ActiveTokens int  `json:"active_tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if !statusResp.Success || !statusResp.Data.SM2Active || statusResp.Data.ActiveTokens != 1 {
		t.Fatalf("unexpected status response: %+v", statusResp)
	}
}
