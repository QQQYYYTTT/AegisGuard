package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"aegisguard/internal/auth"
	"aegisguard/internal/gates"
	"aegisguard/internal/vkey"

	"go.uber.org/zap"
)

func TestHandleToolCallInjectsRealToken(t *testing.T) {
	auth.ResetNonces()
	if err := auth.InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}

	proxy := &AegisProxy{
		actionGate:  gates.NewActionGateWithMode(zap.NewNop(), "strict"),
		tokenIssuer: auth.NewTokenStore(),
		tokenMode:   "strict",
		logger:      zap.NewNop(),
	}

	body := []byte(`{
		"tools": [{
			"type": "function",
			"function": {
				"name": "read_file",
				"description": "Read a file from the workspace",
				"parameters": {
					"type": "object",
					"properties": {
						"path": {"type": "string"}
					},
					"required": ["path"]
				}
			}
		}],
		"messages": [{
			"tool_calls": [{
				"function": {
					"name": "read_file",
					"arguments": {"path": "D:/workspace/demo.txt"}
				}
			}]
		}]
	}`)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), "request_id", "req-001"))
	req = req.WithContext(context.WithValue(req.Context(), "gateway_key", "agk-test-001"))

	result, ok := proxy.handleToolCall(req, body)
	if !ok {
		t.Fatalf("tool call should be allowed, got decision=%s status=%d reason=%s", result.Decision.String(), result.StatusCode, result.Reason)
	}

	tokenHeader := req.Header.Get("X-Aegis-Token")
	if tokenHeader == "" || tokenHeader == "placeholder-token" {
		t.Fatalf("expected injected real token, got %q", tokenHeader)
	}
	if !bytes.Contains([]byte(tokenHeader), []byte(`"tool_name":"read_file"`)) {
		t.Fatalf("expected token payload to contain tool name, got %q", tokenHeader)
	}
	if !bytes.Contains([]byte(tokenHeader), []byte(`"schema_hash":"`)) {
		t.Fatalf("expected token payload to contain schema hash, got %q", tokenHeader)
	}
	if req.Header.Get("X-Aegis-Session-ID") != "req-001" {
		t.Fatalf("expected session header to be injected")
	}
	if req.Header.Get("X-Aegis-Task-ID") != "req-001" {
		t.Fatalf("expected task header to be injected")
	}
	if req.Header.Get("X-Aegis-Tool-Schema") == "" {
		t.Fatalf("expected tool schema header to be injected")
	}
}

func TestHandleToolCallTokenModes(t *testing.T) {
	body := []byte(`{
		"messages": [{
			"tool_calls": [{
				"function": {
					"name": "read_file",
					"arguments": {"path": "D:/workspace/demo.txt"}
				}
			}]
		}]
	}`)

	tests := []struct {
		name             string
		mode             string
		authMode         string
		expectedDecision gates.Decision
		expectedOK       bool
		expectedStatus   string
		expectedUnauth   bool
	}{
		{
			name:             "strict denies when token injection unavailable",
			mode:             "strict",
			authMode:         "gateway_managed",
			expectedDecision: gates.Deny,
			expectedOK:       false,
			expectedStatus:   "skipped",
			expectedUnauth:   false,
		},
		{
			name:             "compat allows but marks unauthorized",
			mode:             "compat",
			authMode:         "gateway_managed",
			expectedDecision: gates.Allow,
			expectedOK:       true,
			expectedStatus:   "skipped",
			expectedUnauth:   true,
		},
		{
			name:             "warn allows but marks unauthorized",
			mode:             "warn",
			authMode:         "gateway_managed",
			expectedDecision: gates.Allow,
			expectedOK:       true,
			expectedStatus:   "skipped",
			expectedUnauth:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := &AegisProxy{
				actionGate:  gates.NewActionGateWithMode(zap.NewNop(), tt.mode),
				tokenIssuer: nil,
				tokenMode:   tt.mode,
				authMode:    tt.authMode,
				logger:      zap.NewNop(),
			}

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			req = req.WithContext(context.WithValue(req.Context(), "request_id", "req-mode"))
			req = req.WithContext(context.WithValue(req.Context(), "gateway_key", "agk-test-001"))

			result, ok := proxy.handleToolCall(req, body)
			if ok != tt.expectedOK {
				t.Fatalf("ok = %v, want %v (decision=%s reason=%s)", ok, tt.expectedOK, result.Decision.String(), result.Reason)
			}
			if result.Decision != tt.expectedDecision {
				t.Fatalf("decision = %s, want %s", result.Decision.String(), tt.expectedDecision.String())
			}
			if result.TokenStatus != tt.expectedStatus {
				t.Fatalf("token status = %q, want %q", result.TokenStatus, tt.expectedStatus)
			}
			if result.AuthMode != tt.authMode {
				t.Fatalf("auth mode = %q, want %q", result.AuthMode, tt.authMode)
			}
			if result.UnauthorizedAllow != tt.expectedUnauth {
				t.Fatalf("unauthorized_allow = %v, want %v", result.UnauthorizedAllow, tt.expectedUnauth)
			}

			header := make(http.Header)
			setGateHeaders(header, result)
			if got := header.Get("X-Aegis-Token-Status"); got != tt.expectedStatus {
				t.Fatalf("header token status = %q, want %q", got, tt.expectedStatus)
			}
			if got := header.Get("X-Aegis-Auth-Mode"); got != tt.authMode {
				t.Fatalf("header auth mode = %q, want %q", got, tt.authMode)
			}
			if tt.expectedUnauth && header.Get("X-Aegis-Unauthorized-Allow") != "true" {
				t.Fatalf("expected unauthorized allow header to be true")
			}
		})
	}
}

func TestWriteGateResponseIncludesTokenHeaders(t *testing.T) {
	proxy := &AegisProxy{logger: zap.NewNop()}
	rec := httptest.NewRecorder()
	result := gateResult{
		Decision:          gates.Allow,
		Reason:            "compat mode allow",
		StatusCode:        http.StatusOK,
		GateType:          "action",
		RiskScore:         12,
		RiskLevel:         "low",
		TokenStatus:       "skipped",
		AuthMode:          "gateway_managed",
		UnauthorizedAllow: true,
	}

	proxy.writeGateResponse(rec, result)

	if rec.Header().Get("X-Aegis-Token-Status") != "skipped" {
		t.Fatalf("missing token status header")
	}
	if rec.Header().Get("X-Aegis-Auth-Mode") != "gateway_managed" {
		t.Fatalf("missing auth mode header")
	}
	if rec.Header().Get("X-Aegis-Unauthorized-Allow") != "true" {
		t.Fatalf("missing unauthorized allow header")
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

func TestDirectorGatewayManagedOverridesAuthorization(t *testing.T) {
	proxy := &AegisProxy{
		target:   mustParseURL(t, "https://upstream.example.com"),
		vkeyMgr:  testVKeyManager(t, "agk-test-001", "https://upstream.example.com", "sk-managed"),
		authMode: "gateway_managed",
		logger:   zap.NewNop(),
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-client")

	proxy.director(req)

	if got := req.Header.Get("Authorization"); got != "Bearer sk-managed" {
		t.Fatalf("authorization = %q, want managed key", got)
	}
}

func TestDirectorPassthroughPreservesClientAuthorization(t *testing.T) {
	proxy := &AegisProxy{
		target:   mustParseURL(t, "https://upstream.example.com"),
		vkeyMgr:  testVKeyManager(t, "agk-test-001", "https://upstream.example.com", ""),
		authMode: "passthrough",
		logger:   zap.NewNop(),
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-client")

	proxy.director(req)

	if got := req.Header.Get("Authorization"); got != "Bearer sk-client" {
		t.Fatalf("authorization = %q, want client authorization preserved", got)
	}
}

func TestDirectorPassthroughDoesNotForwardLegacyGatewayBearer(t *testing.T) {
	proxy := &AegisProxy{
		target:   mustParseURL(t, "https://upstream.example.com"),
		vkeyMgr:  testVKeyManager(t, "agk-test-001", "https://upstream.example.com", ""),
		authMode: "passthrough",
		logger:   zap.NewNop(),
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer agk-test-001")

	proxy.director(req)

	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("authorization = %q, want legacy gateway bearer stripped", got)
	}
}

func TestHandleChatRequestRequiresUpstreamAuthorizationWhenNeeded(t *testing.T) {
	proxy := &AegisProxy{
		messageGate: gates.NewMessageGateWithRuntime(nil),
		vkeyMgr:     testVKeyManager(t, "agk-test-001", "https://upstream.example.com", ""),
		authMode:    "passthrough",
		logger:      zap.NewNop(),
	}

	body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("X-Gateway-Key", "agk-test-001")

	result, ok := proxy.handleChatRequest(req, body)
	if ok {
		t.Fatalf("expected request to be rejected")
	}
	if result.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", result.StatusCode)
	}
	if result.Reason != "missing upstream Authorization" {
		t.Fatalf("reason = %q", result.Reason)
	}
}

func TestHandleChatRequestAllowsPassthroughWithClientAuthorization(t *testing.T) {
	proxy := &AegisProxy{
		messageGate: gates.NewMessageGateWithRuntime(nil),
		vkeyMgr:     testVKeyManager(t, "agk-test-001", "https://upstream.example.com", ""),
		authMode:    "passthrough",
		logger:      zap.NewNop(),
	}

	body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer sk-client")

	result, ok := proxy.handleChatRequest(req, body)
	if !ok {
		t.Fatalf("expected request to continue, got reason=%s", result.Reason)
	}
	if result.AuthMode != "passthrough" {
		t.Fatalf("auth mode = %q, want passthrough", result.AuthMode)
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return parsed
}

func testVKeyManager(t *testing.T, gatewayKey, targetURL, llmAPIKey string) *vkey.Manager {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gateway.yaml")
	content := "gateway_key: " + gatewayKey + "\n" +
		"target_url: " + targetURL + "\n" +
		"llm_api_key: " + llmAPIKey + "\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write gateway config: %v", err)
	}
	manager, err := vkey.NewManager(zap.NewNop(), cfgPath)
	if err != nil {
		t.Fatalf("new vkey manager: %v", err)
	}
	return manager
}
