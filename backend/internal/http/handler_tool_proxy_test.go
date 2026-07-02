package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aegisguard/internal/audit"
	"aegisguard/internal/auth"
	"aegisguard/internal/config"
	"aegisguard/internal/gates"
	"aegisguard/internal/sandbox"
	"aegisguard/internal/vkey"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestHTTPToolProxyForwardsAllowedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newToolProxyTestRouter(t)

	var upstreamBody map[string]any
	var upstreamAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/proxy/weather?upstream="+upstream.URL, bytes.NewBufferString(`{"city":"beijing"}`))
	req.Header.Set("Authorization", "Bearer agk-test-001")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if upstreamBody["city"] != "beijing" {
		t.Fatalf("unexpected upstream body: %+v", upstreamBody)
	}
	if upstreamAuth != "" {
		t.Fatalf("tool proxy should not forward authorization, got %q", upstreamAuth)
	}
	if got := rec.Header().Get("X-Aegis-Decision"); got != "Allow" {
		t.Fatalf("expected allow decision header, got %q", got)
	}
}

func TestHTTPToolProxyAcceptsXGatewayKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newToolProxyTestRouter(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/proxy/weather?upstream="+upstream.URL, bytes.NewBufferString(`{"city":"beijing"}`))
	req.Header.Set("X-Gateway-Key", "agk-test-001")
	req.Header.Set("Authorization", "Bearer sk-real-upstream")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPToolProxyBlocksHighRiskAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newToolProxyTestRouter(t)

	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/proxy/exec_shell?upstream="+upstream.URL, bytes.NewBufferString(`{"command":"rm -rf /prod"}`))
	req.Header.Set("Authorization", "Bearer agk-test-001")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for human approval, got %d body=%s", rec.Code, rec.Body.String())
	}
	if upstreamCalled {
		t.Fatal("upstream should not be called when action gate blocks request")
	}
}

func TestHTTPToolProxyFiltersSensitiveResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newToolProxyTestRouter(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":"api_key: example-api-key","password":"example-password"}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/mcp/proxy/search?upstream="+upstream.URL, bytes.NewBufferString(`{"query":"weather"}`))
	req.Header.Set("Authorization", "Bearer agk-test-001")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("example-api-key")) || bytes.Contains(rec.Body.Bytes(), []byte("example-password")) {
		t.Fatalf("response should be filtered, got %s", rec.Body.String())
	}
	if rec.Header().Get("X-Aegis-Filtered") != "true" {
		t.Fatalf("expected filtered header")
	}
}

func newToolProxyTestRouter(t *testing.T) *Router {
	t.Helper()

	auth.ResetNonces()
	if err := auth.InitSigningKey(""); err != nil {
		t.Fatalf("init signing key: %v", err)
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gateway.yaml")
	if err := os.WriteFile(cfgPath, []byte("gateway_key: agk-test-001\ntarget_url: https://api.openai.com\nllm_api_key: sk-test-key\n"), 0o644); err != nil {
		t.Fatalf("write gateway config: %v", err)
	}

	vkeyMgr, err := vkey.NewManager(zap.NewNop(), cfgPath)
	if err != nil {
		t.Fatalf("new vkey manager: %v", err)
	}

	engine := gin.New()
	tokenStore := auth.NewTokenStore()
	sandboxMgr := sandbox.NewManager(zap.NewNop())
	actionGate := gates.NewActionGateWithMode(zap.NewNop(), "strict")
	gateEvaluator := gates.NewGateEvaluator(
		gates.NewMessageGate(),
		actionGate,
		gates.NewReturnGate(),
		gates.NewDecisionStore(100),
	)

	router := &Router{
		engine:        engine,
		vkeyMgr:       vkeyMgr,
		auditor:       audit.NewLogger(&memoryAuditStore{}),
		tokenStore:    tokenStore,
		gateEvaluator: gateEvaluator,
		contentFilter: sandboxMgr,
		logger:        zap.NewNop(),
		cfg: config.Config{
			TokenMode: "strict",
		},
	}
	router.registerToolProxyRoutes()
	return router
}

type memoryAuditStore struct {
	events []audit.AuditEvent
}

func (m *memoryAuditStore) Append(event audit.AuditEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *memoryAuditStore) ReadAll() ([]audit.AuditEvent, error) {
	return append([]audit.AuditEvent(nil), m.events...), nil
}

func (m *memoryAuditStore) QuerySince(since time.Time) ([]audit.AuditEvent, error) {
	var filtered []audit.AuditEvent
	for _, ev := range m.events {
		if !ev.Timestamp.Before(since) {
			filtered = append(filtered, ev)
		}
	}
	return filtered, nil
}

func (m *memoryAuditStore) AggregateThreatSources(time.Time) ([]audit.ThreatSourceRow, error) {
	return nil, nil
}

func (m *memoryAuditStore) Close() error {
	return nil
}
