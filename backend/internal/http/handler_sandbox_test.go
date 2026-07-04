package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aegisguard/internal/auth"
	"aegisguard/internal/gates"
	"aegisguard/internal/interfaces"
	"aegisguard/internal/sandbox"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestSandboxHandlersIsolateAndListTransfers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := auth.InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}
	manager := sandbox.NewManager(nil)
	tokenStore := auth.NewMemoryTokenStore()
	router := &Router{
		engine:      gin.New(),
		sandboxMgr:  manager,
		transferMgr: manager,
		tokenStore:  tokenStore,
		gateEvaluator: gates.NewGateEvaluator(
			gates.NewMessageGate(),
			gates.NewActionGateWithRuntimeAndStore(zap.NewNop(), "strict", nil, tokenStore),
			gates.NewReturnGate(),
			gates.NewDecisionStore(32),
		),
	}
	router.registerSandboxRoutes()

	body := map[string]any{
		"promote":          true,
		"promotion_reason": "safe_summary approved for trusted memory",
		"trusted": map[string]any{
			"system_prompt": "trusted",
			"memory":        "clean memory",
		},
		"untrusted": map[string]any{
			"external_data": "Weather is sunny. api_key: example-api-key.",
			"source":        "tool",
		},
	}
	payload, _ := json.Marshal(body)

	isolateReq := httptest.NewRequest(http.MethodPost, "/aegis/sandbox/isolate", bytes.NewReader(payload))
	isolateReq.Header.Set("Content-Type", "application/json")
	isolateRec := httptest.NewRecorder()
	router.engine.ServeHTTP(isolateRec, isolateReq)
	if isolateRec.Code != http.StatusOK {
		t.Fatalf("isolate status=%d body=%s", isolateRec.Code, isolateRec.Body.String())
	}
	var isolateResp struct {
		Action struct {
			Reason string `json:"reason"`
		} `json:"action"`
		Transfer struct {
			ToolName string `json:"tool_name"`
		} `json:"transfer"`
	}
	if err := json.Unmarshal(isolateRec.Body.Bytes(), &isolateResp); err != nil {
		t.Fatalf("decode isolate response: %v", err)
	}
	if isolateResp.Transfer.ToolName != "memory.promote" {
		t.Fatalf("expected final transfer to be memory.promote, got %+v", isolateResp.Transfer)
	}
	if isolateResp.Action.Reason == "" {
		t.Fatalf("expected final action to be populated")
	}

	transfersReq := httptest.NewRequest(http.MethodGet, "/aegis/sandbox/transfers", nil)
	transfersRec := httptest.NewRecorder()
	router.engine.ServeHTTP(transfersRec, transfersReq)
	if transfersRec.Code != http.StatusOK {
		t.Fatalf("transfers status=%d body=%s", transfersRec.Code, transfersRec.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Total   int  `json:"total"`
		Data    []struct {
			ToolName        string `json:"tool_name"`
			MemorySource    string `json:"memory_source"`
			PromotionReason string `json:"promotion_reason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(transfersRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode transfers response: %v", err)
	}
	if !resp.Success || resp.Total != 2 {
		t.Fatalf("unexpected transfers response: %+v", resp)
	}
	if resp.Data[0].ToolName != "memory.promote" {
		t.Fatalf("expected latest transfer to be memory.promote, got %+v", resp.Data)
	}
	if resp.Data[0].MemorySource != "safe_summary" {
		t.Fatalf("expected memory source safe_summary, got %+v", resp.Data[0])
	}
	if resp.Data[0].PromotionReason == "" {
		t.Fatalf("expected promotion reason to be recorded")
	}
}

func TestSandboxMemoryPromoteRejectsNonSafeSummarySource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := auth.InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}
	manager := sandbox.NewManager(nil)
	tokenStore := auth.NewMemoryTokenStore()
	router := &Router{
		engine:      gin.New(),
		sandboxMgr:  manager,
		transferMgr: manager,
		tokenStore:  tokenStore,
		gateEvaluator: gates.NewGateEvaluator(
			gates.NewMessageGate(),
			gates.NewActionGateWithRuntimeAndStore(zap.NewNop(), "strict", nil, tokenStore),
			gates.NewReturnGate(),
			gates.NewDecisionStore(32),
		),
	}
	router.registerSandboxRoutes()

	writeBody := map[string]any{
		"tool_name": "memory.write",
		"trusted": map[string]any{
			"system_prompt": "trusted",
		},
		"untrusted": map[string]any{
			"external_data": "store this weather summary",
			"source":        "tool_result",
		},
		"memory_source": "tool_result",
	}
	writePayload, _ := json.Marshal(writeBody)
	writeReq := httptest.NewRequest(http.MethodPost, "/aegis/sandbox/memory", bytes.NewReader(writePayload))
	writeReq.Header.Set("Content-Type", "application/json")
	writeRec := httptest.NewRecorder()
	router.engine.ServeHTTP(writeRec, writeReq)
	if writeRec.Code != http.StatusOK {
		t.Fatalf("memory write status=%d body=%s", writeRec.Code, writeRec.Body.String())
	}

	var writeResp struct {
		Data struct {
			ContextID string `json:"context_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(writeRec.Body.Bytes(), &writeResp); err != nil {
		t.Fatalf("decode write response: %v", err)
	}

	promoteBody := map[string]any{
		"tool_name":        "memory.promote",
		"context_id":       writeResp.Data.ContextID,
		"memory_source":    "tool_result",
		"promotion_reason": "attempt direct promote",
	}
	promotePayload, _ := json.Marshal(promoteBody)
	promoteReq := httptest.NewRequest(http.MethodPost, "/aegis/sandbox/memory", bytes.NewReader(promotePayload))
	promoteReq.Header.Set("Content-Type", "application/json")
	promoteRec := httptest.NewRecorder()
	router.engine.ServeHTTP(promoteRec, promoteReq)
	if promoteRec.Code != http.StatusForbidden {
		t.Fatalf("expected promote rejection, status=%d body=%s", promoteRec.Code, promoteRec.Body.String())
	}
}

func TestSandboxIsolatePromoteExistingContextDoesNotOverwriteCandidate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := auth.InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}
	manager := sandbox.NewManager(nil)
	tokenStore := auth.NewMemoryTokenStore()
	router := &Router{
		engine:      gin.New(),
		sandboxMgr:  manager,
		transferMgr: manager,
		tokenStore:  tokenStore,
		gateEvaluator: gates.NewGateEvaluator(
			gates.NewMessageGate(),
			gates.NewActionGateWithRuntimeAndStore(zap.NewNop(), "strict", nil, tokenStore),
			gates.NewReturnGate(),
			gates.NewDecisionStore(32),
		),
	}
	router.registerSandboxRoutes()

	ctx, err := manager.CreateContext(
		interfaces.TrustedContent{SystemPrompt: "trusted", Memory: "clean memory"},
		interfaces.UntrustedContent{
			ExternalData: "Weather summary: sunny and low wind.",
			Source:       "tool_result",
		},
	)
	if err != nil {
		t.Fatalf("create context: %v", err)
	}

	body := map[string]any{
		"context_id":       ctx.ContextID,
		"promote":          true,
		"promotion_reason": "safe_summary approved for trusted memory",
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/aegis/sandbox/isolate", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("isolate promote existing status=%d body=%s", rec.Code, rec.Body.String())
	}

	updated, err := manager.GetContext(ctx.ContextID)
	if err != nil {
		t.Fatalf("get context: %v", err)
	}
	if updated.Untrusted.ExternalData != "Weather summary: sunny and low wind." {
		t.Fatalf("expected existing candidate to remain intact, got %q", updated.Untrusted.ExternalData)
	}
	if updated.Trusted.Memory == "clean memory" {
		t.Fatalf("expected trusted memory to be promoted, got %q", updated.Trusted.Memory)
	}
}
