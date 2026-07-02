package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aegisguard/internal/auth"
	"aegisguard/internal/config"
	"aegisguard/internal/gates"
	"aegisguard/internal/sandbox"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestBridgeActionEvaluateAllow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth.ResetNonces()
	if err := auth.InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}
	store := auth.NewTokenStore()

	router := &Router{
		engine:     gin.New(),
		tokenStore: store,
		verifier:   auth.NewVerifierWithStore(store),
		gateEvaluator: gates.NewGateEvaluator(
			gates.NewMessageGate(),
			gates.NewActionGateWithRuntimeAndStore(zap.NewNop(), "strict", nil, store),
			gates.NewReturnGate(),
			gates.NewDecisionStore(100),
		),
		logger: zap.NewNop(),
		cfg:    testHTTPConfig(),
	}
	router.registerBridgeRoutes()

	body := map[string]any{
		"request_id": "req-bridge-1",
		"tool_name":  "weather.query",
		"agent_id":   "agent-test",
		"session_id": "session-test",
		"task_id":    "task-test",
		"params": map[string]any{
			"city": "beijing",
		},
		"schema": `{"name":"weather.query","inputSchema":{"type":"object"}}`,
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/aegis/bridge/evaluate/action", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Aegis-Bridge-Key", "bridge-test-key")
	rec := httptest.NewRecorder()

	router.engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp bridgeEvaluateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.Action.Decision.String() != "Allow" {
		t.Fatalf("expected allow, got %+v", resp.Data.Action)
	}
	if resp.Data.Token == "" || resp.Data.SchemaHash == "" {
		t.Fatalf("expected token and schema hash, got %+v", resp.Data)
	}
}

func testHTTPConfig() config.Config {
	return config.Config{
		TokenMode:       "strict",
		BridgeSharedKey: "bridge-test-key",
	}
}

func TestBridgeReturnEvaluateFiltersSensitiveResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := &Router{
		engine:        gin.New(),
		gateEvaluator: gates.NewGateEvaluator(gates.NewMessageGate(), gates.NewActionGateWithMode(zap.NewNop(), "strict"), gates.NewReturnGate(), gates.NewDecisionStore(100)),
		contentFilter: sandbox.NewManager(zap.NewNop()),
		logger:        zap.NewNop(),
		cfg:           testHTTPConfig(),
	}
	router.registerBridgeRoutes()

	body := map[string]any{
		"request_id":    "req-bridge-2",
		"tool_name":     "search.result",
		"agent_id":      "agent-test",
		"response_body": `{"content":"api_key: example-api-key","password":"example-password"}`,
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/aegis/bridge/evaluate/return", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Aegis-Bridge-Key", "bridge-test-key")
	rec := httptest.NewRecorder()

	router.engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp bridgeEvaluateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.Return == nil {
		t.Fatalf("expected return result")
	}
	if bytes.Contains([]byte(resp.Data.ResponseBody), []byte("example-api-key")) || bytes.Contains([]byte(resp.Data.ResponseBody), []byte("example-password")) {
		t.Fatalf("expected filtered response, got %s", resp.Data.ResponseBody)
	}
}
