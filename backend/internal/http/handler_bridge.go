package httpapi

import (
	"encoding/base64"
	"net/http"
	"strings"

	"aegisguard/internal/interfaces"
	"aegisguard/pkg/smcrypto"

	"github.com/gin-gonic/gin"
)

type bridgeEvaluateRequest struct {
	RequestID    string                 `json:"request_id"`
	ToolName     string                 `json:"tool_name"`
	AgentID      string                 `json:"agent_id"`
	SessionID    string                 `json:"session_id"`
	TaskID       string                 `json:"task_id"`
	Params       map[string]interface{} `json:"params"`
	Headers      map[string]string      `json:"headers"`
	ResponseBody string                 `json:"response_body"`
	Schema       string                 `json:"schema"`
	SchemaBase64 bool                   `json:"schema_base64"`
}

type bridgeEvaluateResponse struct {
	Success bool                 `json:"success"`
	Data    bridgeEvaluateResult `json:"data"`
}

type bridgeEvaluateResult struct {
	RequestID      string                     `json:"request_id"`
	Action         interfaces.EvaluateResult  `json:"action"`
	Return         *interfaces.EvaluateResult `json:"return,omitempty"`
	Token          string                     `json:"token,omitempty"`
	TokenStatus    string                     `json:"token_status,omitempty"`
	SchemaHash     string                     `json:"schema_hash,omitempty"`
	Filtered       bool                       `json:"filtered,omitempty"`
	FilteredFields []string                   `json:"filtered_fields,omitempty"`
	ResponseBody   string                     `json:"response_body,omitempty"`
}

func (r *Router) registerBridgeRoutes() {
	group := r.engine.Group("/aegis/bridge")
	{
		group.POST("/evaluate/action", r.handleBridgeEvaluateAction)
		group.POST("/evaluate/return", r.handleBridgeEvaluateReturn)
	}
}

func (r *Router) handleBridgeEvaluateAction(c *gin.Context) {
	var req bridgeEvaluateRequest
	requestID, start := r.auditManualRequest(c, req)
	if err := c.ShouldBindJSON(&req); err != nil {
		r.auditManualResponse(requestID, start, http.StatusBadRequest, "Block", "invalid request body", "action", 0, "low", nil, "invalid", r.cfg.TokenMode)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = requestID
	}
	if strings.TrimSpace(req.AgentID) == "" {
		req.AgentID = "agent-bridge"
	}
	if strings.TrimSpace(req.SessionID) == "" {
		req.SessionID = req.RequestID
	}
	if strings.TrimSpace(req.TaskID) == "" {
		req.TaskID = req.RequestID
	}

	header := make(http.Header)
	for k, v := range req.Headers {
		header.Set(k, v)
	}

	schemaBytes, schemaHash, schemaErr := decodeBridgeSchema(req)
	if schemaErr != nil {
		r.auditManualResponse(req.RequestID, start, http.StatusBadRequest, "Block", "invalid schema payload", "action", 0, "low", nil, "invalid", r.cfg.TokenMode)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid schema payload"})
		return
	}

	tokenSchema := ""
	if len(schemaBytes) > 0 {
		tokenSchema = base64.StdEncoding.EncodeToString(schemaBytes)
	}
	token, err := r.issueToolToken(req.ToolName, req.AgentID, req.SessionID, req.TaskID, tokenSchema)
	if err != nil {
		r.auditManualResponse(req.RequestID, start, http.StatusInternalServerError, "Block", "failed to issue require token", "action", 0, "low", nil, "issue_failed", r.cfg.TokenMode)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to issue require token"})
		return
	}

	header.Set("X-Aegis-Token", token)
	header.Set("X-Aegis-Token-Status", "issued")
	header.Set("X-Aegis-Agent-ID", req.AgentID)
	header.Set("X-Aegis-Session-ID", req.SessionID)
	header.Set("X-Aegis-Task-ID", req.TaskID)
	if len(schemaBytes) > 0 {
		header.Set("X-Aegis-Tool-Schema", base64.StdEncoding.EncodeToString(schemaBytes))
	}

	actionResult := r.gateEvaluator.EvaluateAction(req.RequestID, req.ToolName, req.Params, header, req.AgentID)
	statusCode := httpStatusForDecision(actionResult.Decision.String())
	if actionResult.Decision == interfaces.HumanApproval {
		statusCode = http.StatusAccepted
	}

	r.auditManualResponse(req.RequestID, start, statusCode, actionResult.Decision.String(), actionResult.Reason, "action", actionResult.RiskScore, actionResult.RiskLevel, actionResult.MatchedRules, "issued", r.cfg.TokenMode)
	c.JSON(statusCode, bridgeEvaluateResponse{
		Success: true,
		Data: bridgeEvaluateResult{
			RequestID:   req.RequestID,
			Action:      actionResult,
			Token:       token,
			TokenStatus: "issued",
			SchemaHash:  schemaHash,
		},
	})
}

func (r *Router) handleBridgeEvaluateReturn(c *gin.Context) {
	var req bridgeEvaluateRequest
	requestID, start := r.auditManualRequest(c, req)
	if err := c.ShouldBindJSON(&req); err != nil {
		r.auditManualResponse(requestID, start, http.StatusBadRequest, "Block", "invalid request body", "return", 0, "low", nil, "", r.cfg.TokenMode)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = requestID
	}
	if strings.TrimSpace(req.AgentID) == "" {
		req.AgentID = "agent-bridge"
	}

	body := []byte(req.ResponseBody)
	returnResult := r.gateEvaluator.EvaluateReturn(req.RequestID, body, req.AgentID)
	responseBody := body
	filteredFields := []string(nil)
	filtered := false

	if returnResult.Decision == interfaces.Degrade && r.contentFilter != nil {
		nextBody, removed := r.contentFilter.FilterToolResponse(body, req.ToolName)
		responseBody = nextBody
		filteredFields = removed
		filtered = len(removed) > 0 || string(nextBody) != string(body)
	}

	statusCode := httpStatusForDecision(returnResult.Decision.String())
	r.auditManualResponse(req.RequestID, start, statusCode, returnResult.Decision.String(), returnResult.Reason, "return", returnResult.RiskScore, returnResult.RiskLevel, returnResult.MatchedRules, "", r.cfg.TokenMode)
	c.JSON(statusCode, bridgeEvaluateResponse{
		Success: true,
		Data: bridgeEvaluateResult{
			RequestID:      req.RequestID,
			Action:         interfaces.EvaluateResult{Decision: interfaces.Allow, Reason: "not evaluated"},
			Return:         &returnResult,
			Filtered:       filtered,
			FilteredFields: filteredFields,
			ResponseBody:   string(responseBody),
		},
	})
}

func decodeBridgeSchema(req bridgeEvaluateRequest) ([]byte, string, error) {
	schema := strings.TrimSpace(req.Schema)
	if schema == "" {
		return nil, "", nil
	}
	if req.SchemaBase64 {
		data, err := base64.StdEncoding.DecodeString(schema)
		if err != nil {
			return nil, "", err
		}
		return data, smcrypto.SM3Hex(data), nil
	}
	data := []byte(schema)
	return data, smcrypto.SM3Hex(data), nil
}
