package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aegisguard/internal/auth"
	"aegisguard/internal/gates"

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
		expectedDecision gates.Decision
		expectedOK       bool
		expectedStatus   string
		expectedUnauth   bool
	}{
		{
			name:             "strict denies when token injection unavailable",
			mode:             "strict",
			expectedDecision: gates.Deny,
			expectedOK:       false,
			expectedStatus:   "skipped",
			expectedUnauth:   false,
		},
		{
			name:             "compat allows but marks unauthorized",
			mode:             "compat",
			expectedDecision: gates.Allow,
			expectedOK:       true,
			expectedStatus:   "skipped",
			expectedUnauth:   true,
		},
		{
			name:             "warn allows but marks unauthorized",
			mode:             "warn",
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
			if result.AuthMode != tt.mode {
				t.Fatalf("auth mode = %q, want %q", result.AuthMode, tt.mode)
			}
			if result.UnauthorizedAllow != tt.expectedUnauth {
				t.Fatalf("unauthorized_allow = %v, want %v", result.UnauthorizedAllow, tt.expectedUnauth)
			}

			header := make(http.Header)
			setGateHeaders(header, result)
			if got := header.Get("X-Aegis-Token-Status"); got != tt.expectedStatus {
				t.Fatalf("header token status = %q, want %q", got, tt.expectedStatus)
			}
			if got := header.Get("X-Aegis-Auth-Mode"); got != tt.mode {
				t.Fatalf("header auth mode = %q, want %q", got, tt.mode)
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
		AuthMode:          "compat",
		UnauthorizedAllow: true,
	}

	proxy.writeGateResponse(rec, result)

	if rec.Header().Get("X-Aegis-Token-Status") != "skipped" {
		t.Fatalf("missing token status header")
	}
	if rec.Header().Get("X-Aegis-Auth-Mode") != "compat" {
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
