package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"aegisguard/internal/interfaces"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type sandboxIsolateRequest struct {
	ContextID       string                      `json:"context_id"`
	AgentID         string                      `json:"agent_id"`
	SessionID       string                      `json:"session_id"`
	TaskID          string                      `json:"task_id"`
	Trusted         interfaces.TrustedContent   `json:"trusted"`
	Untrusted       interfaces.UntrustedContent `json:"untrusted"`
	Promote         bool                        `json:"promote"`
	PromotionReason string                      `json:"promotion_reason"`
}

type sandboxMemoryRequest struct {
	ToolName        string                      `json:"tool_name"`
	ContextID       string                      `json:"context_id"`
	AgentID         string                      `json:"agent_id"`
	SessionID       string                      `json:"session_id"`
	TaskID          string                      `json:"task_id"`
	Trusted         interfaces.TrustedContent   `json:"trusted"`
	Untrusted       interfaces.UntrustedContent `json:"untrusted"`
	MemorySource    string                      `json:"memory_source"`
	PromotionReason string                      `json:"promotion_reason"`
}

type sandboxMemoryOutcome struct {
	Context     *interfaces.SandboxContext `json:"context,omitempty"`
	Transfer    *interfaces.TransferRecord `json:"transfer,omitempty"`
	Action      interfaces.EvaluateResult  `json:"action"`
	TokenStatus string                     `json:"token_status,omitempty"`
}

type sandboxMetadataSetter interface {
	SetMetadata(contextID, agentID, sessionID string) error
}

type sandboxMemoryWriter interface {
	RecordMemoryWrite(contextID string, data interfaces.UntrustedContent, memorySource string) (*interfaces.TransferRecord, error)
}

type sandboxMemoryUpdater interface {
	UpdateMemoryCandidate(contextID string, data interfaces.UntrustedContent, memorySource string) (*interfaces.TransferRecord, error)
}

type sandboxMemoryPromoter interface {
	PromoteMemory(contextID string, data interfaces.UntrustedContent, memorySource, promotionReason string) (*interfaces.TransferRecord, error)
}

func (r *Router) registerSandboxRoutes() {
	group := r.engine.Group("/aegis/sandbox")
	{
		group.GET("/context", r.handleSandboxContext)
		group.GET("/transfers", r.handleSandboxTransfers)
		group.POST("/isolate", r.handleSandboxIsolate)
		group.POST("/memory", r.handleSandboxMemory)
	}
}

func (r *Router) handleSandboxContext(c *gin.Context) {
	if r.sandboxMgr == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": defaultSandboxContext()})
		return
	}

	contextID := c.Query("context_id")
	ctx, err := r.sandboxMgr.GetContext(contextID)
	if err != nil && contextID == "" {
		ctx, err = r.sandboxMgr.CreateContext(defaultSandboxTrusted(), defaultSandboxUntrusted())
	}
	if err != nil {
		sandboxLogger(r).Warn("sandbox context not found", zap.String("context_id", contextID), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "sandbox context not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": ctx})
}

func (r *Router) handleSandboxTransfers(c *gin.Context) {
	if r.transferMgr == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []interfaces.TransferRecord{}, "total": 0})
		return
	}

	limit := 50
	if n, err := parseInt(c.DefaultQuery("limit", "50")); err == nil && n > 0 && n <= 500 {
		limit = n
	}
	contextID := c.Query("context_id")

	records, err := r.transferMgr.GetRecords(contextID, limit)
	if err != nil {
		sandboxLogger(r).Warn("read sandbox transfers failed", zap.String("context_id", contextID), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "sandbox transfer records not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": records, "total": len(records)})
}

func (r *Router) handleSandboxIsolate(c *gin.Context) {
	if r.sandboxMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "sandbox manager unavailable"})
		return
	}

	var req sandboxIsolateRequest
	requestID, start := r.auditManualRequest(c, req)

	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			r.auditManualResponse(requestID, start, http.StatusBadRequest, "Block", "invalid request body", "action", 0, "low", nil, "", "")
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
			return
		}
	}

	hasContextID := strings.TrimSpace(req.ContextID) != ""
	hasUntrustedPayload := !isEmptyUntrusted(req.Untrusted)
	if !hasContextID && isEmptyTrusted(req.Trusted) {
		req.Trusted = defaultSandboxTrusted()
	}
	if !hasContextID && !hasUntrustedPayload {
		req.Untrusted = defaultSandboxUntrusted()
		hasUntrustedPayload = true
	}

	var (
		outcome    *sandboxMemoryOutcome
		statusCode int
		err        error
	)
	if hasContextID {
		ctx, getErr := r.sandboxMgr.GetContext(req.ContextID)
		if getErr != nil {
			r.auditManualResponse(requestID, start, http.StatusNotFound, "Block", "sandbox context not found", "action", 0, "low", nil, "", "")
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "sandbox context not found"})
			return
		}
		outcome = &sandboxMemoryOutcome{
			Context: ctx,
			Action: interfaces.EvaluateResult{
				Decision: interfaces.Allow,
				Reason:   "sandbox context loaded",
			},
		}
	}
	if !hasContextID || hasUntrustedPayload {
		writeTool := "memory.write"
		if hasContextID {
			writeTool = "memory.update"
		}
		writeReq := sandboxMemoryRequest{
			ToolName:     writeTool,
			ContextID:    req.ContextID,
			AgentID:      req.AgentID,
			SessionID:    req.SessionID,
			TaskID:       req.TaskID,
			Trusted:      req.Trusted,
			Untrusted:    req.Untrusted,
			MemorySource: firstNonEmpty(req.Untrusted.Source, "sandbox_isolate"),
		}

		outcome, statusCode, err = r.executeSandboxMemoryAction(requestID, writeReq)
		if err != nil {
			sandboxLogger(r).Warn("sandbox memory write failed", zap.Error(err))
			r.auditManualResponse(requestID, start, statusCode, outcome.Action.Decision.String(), outcome.Action.Reason, "action", outcome.Action.RiskScore, outcome.Action.RiskLevel, outcome.Action.MatchedRules, outcome.TokenStatus, r.cfg.TokenMode)
			c.JSON(statusCode, gin.H{"success": false, "error": err.Error(), "data": outcome.Context, "transfer": outcome.Transfer, "action": outcome.Action})
			return
		}
	}

	ctx := outcome.Context
	transfer := outcome.Transfer
	finalAction := outcome.Action
	finalTokenStatus := outcome.TokenStatus

	if req.Promote {
		promoteReq := sandboxMemoryRequest{
			ToolName:        "memory.promote",
			ContextID:       ctx.ContextID,
			AgentID:         req.AgentID,
			SessionID:       req.SessionID,
			TaskID:          req.TaskID,
			Untrusted:       req.Untrusted,
			MemorySource:    "safe_summary",
			PromotionReason: firstNonEmpty(req.PromotionReason, "safe_summary approved for trusted memory"),
		}
		promoteOutcome, promoteStatus, promoteErr := r.executeSandboxMemoryAction(requestID, promoteReq)
		if promoteOutcome.Context != nil {
			ctx = promoteOutcome.Context
		}
		if promoteOutcome.Transfer != nil {
			transfer = promoteOutcome.Transfer
		}
		finalAction = promoteOutcome.Action
		finalTokenStatus = promoteOutcome.TokenStatus
		if promoteErr != nil {
			sandboxLogger(r).Warn("sandbox promote failed", zap.String("context_id", ctx.ContextID), zap.Error(promoteErr))
			r.auditManualResponse(requestID, start, promoteStatus, promoteOutcome.Action.Decision.String(), promoteOutcome.Action.Reason, "action", promoteOutcome.Action.RiskScore, promoteOutcome.Action.RiskLevel, promoteOutcome.Action.MatchedRules, promoteOutcome.TokenStatus, r.cfg.TokenMode)
			c.JSON(promoteStatus, gin.H{"success": false, "error": promoteErr.Error(), "data": ctx, "transfer": transfer, "action": promoteOutcome.Action})
			return
		}
	}

	decision := "Allow"
	reason := "sandbox context isolated"
	riskScore := ctx.RiskScore
	riskLevel := ctx.RiskLevel
	if transfer != nil && !transfer.Approved {
		decision = "Block"
		reason = firstNonEmpty(transfer.Reason, "sandbox content remains isolated")
		riskScore = transfer.RiskScore
		riskLevel = transfer.RiskLevel
	} else if ctx.Status == "quarantined" {
		decision = "Block"
		reason = "sandbox context quarantined"
	}
	r.auditManualResponse(requestID, start, http.StatusOK, decision, reason, "action", riskScore, riskLevel, nil, finalTokenStatus, r.cfg.TokenMode)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"data":     ctx,
		"transfer": transfer,
		"action":   finalAction,
	})
}

func (r *Router) handleSandboxMemory(c *gin.Context) {
	if r.sandboxMgr == nil || r.transferMgr == nil || r.gateEvaluator == nil || r.tokenStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "sandbox memory actions unavailable"})
		return
	}

	var req sandboxMemoryRequest
	requestID, start := r.auditManualRequest(c, req)
	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			r.auditManualResponse(requestID, start, http.StatusBadRequest, "Block", "invalid request body", "action", 0, "low", nil, "", "")
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
			return
		}
	}

	outcome, statusCode, err := r.executeSandboxMemoryAction(requestID, req)
	r.auditManualResponse(requestID, start, statusCode, outcome.Action.Decision.String(), outcome.Action.Reason, "action", outcome.Action.RiskScore, outcome.Action.RiskLevel, outcome.Action.MatchedRules, outcome.TokenStatus, r.cfg.TokenMode)
	if err != nil {
		c.JSON(statusCode, gin.H{"success": false, "error": err.Error(), "data": outcome.Context, "transfer": outcome.Transfer, "action": outcome.Action})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"data":     outcome.Context,
		"transfer": outcome.Transfer,
		"action":   outcome.Action,
	})
}

func (r *Router) executeSandboxMemoryAction(requestID string, req sandboxMemoryRequest) (*sandboxMemoryOutcome, int, error) {
	outcome := &sandboxMemoryOutcome{}
	if r == nil || r.sandboxMgr == nil || r.transferMgr == nil || r.gateEvaluator == nil || r.tokenStore == nil {
		outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "sandbox memory actions unavailable"}
		return outcome, http.StatusServiceUnavailable, errors.New("sandbox memory actions unavailable")
	}

	toolName := strings.TrimSpace(req.ToolName)
	if toolName == "" {
		outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "memory tool name is required"}
		return outcome, http.StatusBadRequest, errors.New("memory tool name is required")
	}

	agentID := firstNonEmptyMany(req.AgentID, "agent-sandbox")
	sessionID := firstNonEmptyMany(req.SessionID, requestID, "session-sandbox")
	taskID := firstNonEmptyMany(req.TaskID, req.ContextID, requestID, "task-sandbox")
	memorySource := firstNonEmptyMany(req.MemorySource, req.Untrusted.Source, "sandbox")
	promotionReason := strings.TrimSpace(req.PromotionReason)

	params := map[string]interface{}{
		"context_id":       req.ContextID,
		"memory_source":    memorySource,
		"promotion_reason": promotionReason,
		"user_input":       req.Untrusted.UserInput,
		"external_data":    req.Untrusted.ExternalData,
		"injected_content": req.Untrusted.InjectedContent,
		"content_type":     req.Untrusted.ContentType,
		"source":           req.Untrusted.Source,
	}

	token, err := r.tokenStore.Issue(toolName, toolName+":invoke", agentID, sessionID, taskID, 5*time.Minute, 1)
	if err != nil {
		outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: err.Error()}
		return outcome, http.StatusInternalServerError, err
	}
	payload, err := json.Marshal(token)
	if err != nil {
		outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: err.Error()}
		return outcome, http.StatusInternalServerError, err
	}

	headers := make(http.Header)
	headers.Set("X-Aegis-Token", string(payload))
	headers.Set("X-Aegis-Token-Status", "issued")
	headers.Set("X-Aegis-Agent-ID", agentID)
	headers.Set("X-Aegis-Session-ID", sessionID)
	headers.Set("X-Aegis-Task-ID", taskID)

	outcome.TokenStatus = "issued"
	outcome.Action = r.gateEvaluator.EvaluateAction(requestID, toolName, params, headers, agentID)
	switch outcome.Action.Decision {
	case interfaces.Allow:
	case interfaces.HumanApproval:
		return outcome, http.StatusAccepted, errors.New(outcome.Action.Reason)
	default:
		return outcome, http.StatusForbidden, errors.New(outcome.Action.Reason)
	}

	switch toolName {
	case "memory.write":
		if isEmptyTrusted(req.Trusted) {
			req.Trusted = defaultSandboxTrusted()
		}
		if isEmptyUntrusted(req.Untrusted) {
			req.Untrusted = defaultSandboxUntrusted()
		}
		ctx, err := r.sandboxMgr.CreateContext(req.Trusted, req.Untrusted)
		if err != nil {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "create sandbox context failed"}
			return outcome, http.StatusInternalServerError, err
		}
		outcome.Context = ctx
		if setter, ok := r.sandboxMgr.(sandboxMetadataSetter); ok {
			if err := setter.SetMetadata(ctx.ContextID, agentID, sessionID); err == nil {
				if refreshed, getErr := r.sandboxMgr.GetContext(ctx.ContextID); getErr == nil {
					outcome.Context = refreshed
				}
			}
		}
		writer, ok := r.transferMgr.(sandboxMemoryWriter)
		if !ok {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "sandbox memory writer unavailable"}
			return outcome, http.StatusServiceUnavailable, errors.New("sandbox memory writer unavailable")
		}
		transfer, err := writer.RecordMemoryWrite(outcome.Context.ContextID, req.Untrusted, memorySource)
		if err != nil {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "record memory write failed"}
			return outcome, http.StatusInternalServerError, err
		}
		outcome.Transfer = transfer
		if refreshed, getErr := r.sandboxMgr.GetContext(outcome.Context.ContextID); getErr == nil {
			outcome.Context = refreshed
		}
		return outcome, http.StatusOK, nil
	case "memory.update":
		if strings.TrimSpace(req.ContextID) == "" {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "memory.update requires context_id"}
			return outcome, http.StatusBadRequest, errors.New("memory.update requires context_id")
		}
		updater, ok := r.transferMgr.(sandboxMemoryUpdater)
		if !ok {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "sandbox memory updater unavailable"}
			return outcome, http.StatusServiceUnavailable, errors.New("sandbox memory updater unavailable")
		}
		transfer, err := updater.UpdateMemoryCandidate(req.ContextID, req.Untrusted, memorySource)
		if err != nil {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "update memory candidate failed"}
			return outcome, http.StatusInternalServerError, err
		}
		outcome.Transfer = transfer
		ctx, err := r.sandboxMgr.GetContext(req.ContextID)
		if err != nil {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "sandbox context not found"}
			return outcome, http.StatusInternalServerError, err
		}
		outcome.Context = ctx
		return outcome, http.StatusOK, nil
	case "memory.promote":
		if strings.TrimSpace(req.ContextID) == "" {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "memory.promote requires context_id"}
			return outcome, http.StatusBadRequest, errors.New("memory.promote requires context_id")
		}
		promoter, ok := r.transferMgr.(sandboxMemoryPromoter)
		if !ok {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "sandbox memory promoter unavailable"}
			return outcome, http.StatusServiceUnavailable, errors.New("sandbox memory promoter unavailable")
		}
		transfer, err := promoter.PromoteMemory(req.ContextID, req.Untrusted, memorySource, firstNonEmpty(promotionReason, "safe_summary approved for trusted memory"))
		if err != nil {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "promote memory failed"}
			return outcome, http.StatusInternalServerError, err
		}
		outcome.Transfer = transfer
		ctx, err := r.sandboxMgr.GetContext(req.ContextID)
		if err != nil {
			outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "sandbox context not found"}
			return outcome, http.StatusInternalServerError, err
		}
		outcome.Context = ctx
		return outcome, http.StatusOK, nil
	default:
		outcome.Action = interfaces.EvaluateResult{Decision: interfaces.Block, Reason: "unsupported memory tool"}
		return outcome, http.StatusBadRequest, errors.New("unsupported memory tool")
	}
}

func firstNonEmptyMany(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultSandboxContext() *interfaces.SandboxContext {
	return &interfaces.SandboxContext{
		ContextID: "sandbox-bootstrap",
		Trusted:   defaultSandboxTrusted(),
		Untrusted: defaultSandboxUntrusted(),
		Status:    "isolated",
	}
}

func defaultSandboxTrusted() interfaces.TrustedContent {
	return interfaces.TrustedContent{
		SystemPrompt: "AegisGuard Trusted Core Context",
		ToolDefinitions: []string{
			"MessageGate: inspect inbound prompts",
			"ActionGate: authorize tool calls",
			"ReturnGate: inspect external results",
		},
		Memory:    "Only sanitized summaries approved by Transfer Gate may enter trusted memory.",
		TaskState: "sandbox initialized",
	}
}

func defaultSandboxUntrusted() interfaces.UntrustedContent {
	return interfaces.UntrustedContent{
		UserInput:       "No active user input captured yet.",
		ExternalData:    "No external tool/RAG/web result captured yet.",
		InjectedContent: "Untrusted content is isolated until scanned, summarized and approved.",
		Source:          "bootstrap",
		ContentType:     "text/plain",
	}
}

func isEmptyTrusted(value interfaces.TrustedContent) bool {
	return value.SystemPrompt == "" && len(value.ToolDefinitions) == 0 && value.Memory == "" && value.TaskState == ""
}

func isEmptyUntrusted(value interfaces.UntrustedContent) bool {
	return value.UserInput == "" && value.ExternalData == "" && value.InjectedContent == ""
}

func sandboxLogger(r *Router) *zap.Logger {
	if r == nil || r.logger == nil {
		return zap.NewNop()
	}
	return r.logger
}
