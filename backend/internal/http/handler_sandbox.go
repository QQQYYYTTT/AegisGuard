package httpapi

import (
	"errors"
	"io"
	"net/http"

	"aegisguard/internal/interfaces"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (r *Router) registerSandboxRoutes() {
	group := r.engine.Group("/aegis/sandbox")
	{
		group.GET("/context", r.handleSandboxContext)
		group.GET("/transfers", r.handleSandboxTransfers)
		group.POST("/isolate", r.handleSandboxIsolate)
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

	var req struct {
		AgentID   string                      `json:"agent_id"`
		SessionID string                      `json:"session_id"`
		Trusted   interfaces.TrustedContent   `json:"trusted"`
		Untrusted interfaces.UntrustedContent `json:"untrusted"`
		Promote   bool                        `json:"promote"`
	}

	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
			return
		}
	}

	if isEmptyTrusted(req.Trusted) {
		req.Trusted = defaultSandboxTrusted()
	}
	if isEmptyUntrusted(req.Untrusted) {
		req.Untrusted = defaultSandboxUntrusted()
	}

	ctx, err := r.sandboxMgr.CreateContext(req.Trusted, req.Untrusted)
	if err != nil {
		sandboxLogger(r).Error("create sandbox context failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "create sandbox context failed"})
		return
	}
	ctx.AgentID = req.AgentID
	ctx.SessionID = req.SessionID
	if metadataSetter, ok := r.sandboxMgr.(interface {
		SetMetadata(contextID, agentID, sessionID string) error
	}); ok {
		if err := metadataSetter.SetMetadata(ctx.ContextID, req.AgentID, req.SessionID); err == nil {
			if refreshed, getErr := r.sandboxMgr.GetContext(ctx.ContextID); getErr == nil {
				ctx = refreshed
			}
		}
	}

	var transfer *interfaces.TransferRecord
	if req.Promote && r.transferMgr != nil {
		transfer, err = r.transferMgr.UntrustedToTrusted(ctx.ContextID, req.Untrusted)
		if err != nil {
			sandboxLogger(r).Warn("sandbox promote failed", zap.String("context_id", ctx.ContextID), zap.Error(err))
		}
		if refreshed, getErr := r.sandboxMgr.GetContext(ctx.ContextID); getErr == nil {
			ctx = refreshed
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"data":     ctx,
		"transfer": transfer,
	})
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
