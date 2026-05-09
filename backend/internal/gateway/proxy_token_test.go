package gateway

import (
	"bytes"
	"context"
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
		actionGate:  gates.NewActionGate(zap.NewNop()),
		tokenIssuer: auth.NewTokenStore(),
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
		t.Fatalf("tool call should be allowed, result=%+v", result)
	}

	tokenHeader := req.Header.Get("X-Aegis-Token")
	if tokenHeader == "" || tokenHeader == "placeholder-token" {
		t.Fatalf("expected injected real token, got %q", tokenHeader)
	}
	if !bytes.Contains([]byte(tokenHeader), []byte(`"tool_name":"read_file"`)) {
		t.Fatalf("expected token payload to contain tool name, got %q", tokenHeader)
	}
}
