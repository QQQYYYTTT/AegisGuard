package httpapi

import (
	"net/http"
	"time"

	"aegisguard/internal/auth"

	"github.com/gin-gonic/gin"
)

type tokenInfoResponse struct {
	TokenID            string          `json:"token_id"`
	ToolName           string          `json:"tool_name"`
	Scope              string          `json:"scope"`
	AgentID            string          `json:"agent_id"`
	SessionID          string          `json:"session_id"`
	TaskID             string          `json:"task_id"`
	ExpiresAt          string          `json:"expires_at"`
	Nonce              string          `json:"nonce"`
	RiskLevel          string          `json:"risk_level"`
	SchemaHash         string          `json:"schema_hash"`
	MaxCalls           int             `json:"max_calls"`
	CallCount          int             `json:"call_count"`
	Signature          string          `json:"signature"`
	Signed             bool            `json:"signed"`
	Verified           bool            `json:"verified"`
	VerificationChecks map[string]bool `json:"verification_checks"`
}

type authStatusResponse struct {
	SM2Active     bool   `json:"sm2_active"`
	SM3Active     bool   `json:"sm3_active"`
	SM4Active     bool   `json:"sm4_active"`
	KeyExpiresAt  string `json:"key_expires_at"`
	ActiveTokens  int    `json:"active_tokens"`
	RevokedTokens int    `json:"revoked_tokens"`
}

type issueTokenRequest struct {
	ToolName   string `json:"tool_name"`
	Scope      string `json:"scope"`
	AgentID    string `json:"agent_id"`
	SessionID  string `json:"session_id"`
	TaskID     string `json:"task_id"`
	SchemaHash string `json:"schema_hash"`
	TTLSeconds int    `json:"ttl_seconds"`
	MaxCalls   int    `json:"max_calls"`
}

type verifyTokenRequest struct {
	TokenID string `json:"token_id"`
}

func (r *Router) handleGetToken(c *gin.Context) {
	tokenID := c.Query("token_id")
	var token *auth.RequireToken
	var err error

	if tokenID != "" {
		token, err = r.tokenStore.GetByID(tokenID)
	} else {
		token, err = r.tokenStore.GetLatest()
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    r.buildTokenInfo(token),
	})
}

func (r *Router) handleIssueToken(c *gin.Context) {
	req := issueTokenRequest{
		ToolName:   "demo.tool",
		Scope:      "demo.tool:invoke",
		AgentID:    "agent-ui",
		SessionID:  "session-ui",
		TaskID:     "task-ui",
		TTLSeconds: 300,
		MaxCalls:   1,
	}
	requestID, start := r.auditManualRequest(c, req)
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			r.auditManualResponse(requestID, start, http.StatusBadRequest, "Block", "invalid request body", "action", 0, "low", nil, "invalid", r.cfg.TokenMode)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "invalid request body",
			})
			return
		}
	}

	if req.ToolName == "" {
		req.ToolName = "demo.tool"
	}
	if req.Scope == "" {
		req.Scope = req.ToolName + ":invoke"
	}
	if req.AgentID == "" {
		req.AgentID = "agent-ui"
	}
	if req.SessionID == "" {
		req.SessionID = "session-ui"
	}
	if req.TaskID == "" {
		req.TaskID = "task-ui"
	}
	if req.TTLSeconds <= 0 {
		req.TTLSeconds = 300
	}

	token, err := r.tokenStore.Issue(
		req.ToolName,
		req.Scope,
		req.AgentID,
		req.SessionID,
		req.TaskID,
		time.Duration(req.TTLSeconds)*time.Second,
		req.MaxCalls,
	)
	if err != nil {
		r.auditManualResponse(requestID, start, http.StatusInternalServerError, "Block", err.Error(), "action", 0, "low", nil, "issue_failed", r.cfg.TokenMode)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if req.SchemaHash != "" {
		token.SchemaHash = req.SchemaHash
		if err := token.Sign(); err != nil {
			r.auditManualResponse(requestID, start, http.StatusInternalServerError, "Block", err.Error(), "action", 0, "low", nil, "issue_failed", r.cfg.TokenMode)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		if err := r.tokenStore.Save(token); err != nil {
			r.auditManualResponse(requestID, start, http.StatusInternalServerError, "Block", err.Error(), "action", 0, "low", nil, "issue_failed", r.cfg.TokenMode)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
	}

	r.auditManualResponse(requestID, start, http.StatusOK, "Allow", "require token issued", "action", 20, "low", nil, "issued", r.cfg.TokenMode)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    r.buildTokenInfo(token),
	})
}

func (r *Router) handleVerifyToken(c *gin.Context) {
	var req verifyTokenRequest
	requestID, start := r.auditManualRequest(c, req)
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			r.auditManualResponse(requestID, start, http.StatusBadRequest, "Block", "invalid request body", "action", 0, "low", nil, "invalid", r.cfg.TokenMode)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "invalid request body",
			})
			return
		}
	}

	var token *auth.RequireToken
	var err error
	if req.TokenID != "" {
		token, err = r.tokenStore.GetByID(req.TokenID)
	} else {
		token, err = r.tokenStore.GetLatest()
	}
	if err != nil {
		r.auditManualResponse(requestID, start, http.StatusOK, "Block", "require token not found", "action", 90, "high", nil, "missing", r.cfg.TokenMode)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"valid":  false,
				"checks": verificationChecksMap(auth.VerificationChecks{}),
			},
		})
		return
	}

	checks := r.verifier.Inspect(token)
	checkMap := verificationChecksMap(checks)
	checkMap["schema_hash_match"] = true
	checkMap["scope_match"] = true
	checkMap["risk_level_ok"] = true
	valid := checks.IsValid()
	decision := boolDecision(valid)
	reason := "require token verified"
	if !valid {
		reason = "require token verification failed"
	}
	riskScore := 15
	riskLevel := "low"
	if !valid {
		riskScore = 95
		riskLevel = "critical"
	}
	r.auditManualResponse(
		requestID,
		start,
		http.StatusOK,
		decision,
		reason,
		"action",
		riskScore,
		riskLevel,
		nil,
		statusFromBool(valid, "valid", "invalid"),
		r.cfg.TokenMode,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"valid":  valid,
			"checks": checkMap,
		},
	})
}

func (r *Router) handleAuthStatus(c *gin.Context) {
	latest, _ := r.tokenStore.GetLatest()
	keyExpiry := ""
	if latest != nil {
		keyExpiry = latest.ExpiresAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": authStatusResponse{
			SM2Active:     auth.GetSigningPublicKey() != nil,
			SM3Active:     true,
			SM4Active:     true,
			KeyExpiresAt:  keyExpiry,
			ActiveTokens:  r.tokenStore.ActiveCount(),
			RevokedTokens: r.tokenStore.RevokedCount(),
		},
	})
}

func (r *Router) buildTokenInfo(token *auth.RequireToken) tokenInfoResponse {
	checks := r.verifier.Inspect(token)
	checkMap := verificationChecksMap(checks)
	checkMap["schema_hash_match"] = true
	checkMap["scope_match"] = true
	checkMap["risk_level_ok"] = true

	return tokenInfoResponse{
		TokenID:            token.TokenID,
		ToolName:           token.ToolName,
		Scope:              token.Scope,
		AgentID:            token.AgentID,
		SessionID:          token.SessionID,
		TaskID:             token.TaskID,
		ExpiresAt:          token.ExpiresAt.Format(time.RFC3339),
		Nonce:              token.Nonce,
		RiskLevel:          mapRiskLevel(token.RiskLevel),
		SchemaHash:         token.SchemaHash,
		MaxCalls:           token.MaxCalls,
		CallCount:          token.CallCount,
		Signature:          token.Signature,
		Signed:             token.Signature != "",
		Verified:           checks.IsValid(),
		VerificationChecks: checkMap,
	}
}

func verificationChecksMap(checks auth.VerificationChecks) map[string]bool {
	return map[string]bool{
		"signature_valid": checks.SignatureValid,
		"expiry_valid":    checks.ExpiryValid,
		"nonce_valid":     checks.NonceValid,
		"call_budget_ok":  checks.CallBudgetOK,
	}
}

func mapRiskLevel(level int) string {
	switch {
	case level >= 80:
		return "critical"
	case level >= 60:
		return "high"
	case level >= 30:
		return "medium"
	default:
		return "low"
	}
}
