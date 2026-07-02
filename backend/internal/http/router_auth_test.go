package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aegisguard/internal/audit"
	"aegisguard/internal/config"
	"aegisguard/internal/gates"
	"aegisguard/internal/gateway"
	"aegisguard/internal/vkey"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestHandleProxyAcceptsXGatewayKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer upstream.Close()

	router := newV1ProxyTestRouter(t, upstream.URL, "gateway_managed", "sk-managed")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("X-Gateway-Key", "agk-test-001")
	req.Header.Set("Authorization", "Bearer sk-client")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	closeRec := &closeNotifyRecorder{ResponseRecorder: rec}

	router.engine.ServeHTTP(closeRec, req)

	if closeRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", closeRec.Code, closeRec.Body.String())
	}
	if upstreamAuth != "Bearer sk-managed" {
		t.Fatalf("upstream authorization = %q, want managed key", upstreamAuth)
	}
	if closeRec.Header().Get("X-Aegis-Auth-Mode") != "gateway_managed" {
		t.Fatalf("auth mode header = %q", closeRec.Header().Get("X-Aegis-Auth-Mode"))
	}
}

func TestHandleProxyAcceptsLegacyBearerGatewayKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer upstream.Close()

	router := newV1ProxyTestRouter(t, upstream.URL, "gateway_managed", "sk-managed")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer agk-test-001")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	closeRec := &closeNotifyRecorder{ResponseRecorder: rec}

	router.engine.ServeHTTP(closeRec, req)

	if closeRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", closeRec.Code, closeRec.Body.String())
	}
}

func TestHandleProxyPassthroughForwardsClientAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer upstream.Close()

	router := newV1ProxyTestRouter(t, upstream.URL, "passthrough", "")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("X-Gateway-Key", "agk-test-001")
	req.Header.Set("Authorization", "Bearer sk-client")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	closeRec := &closeNotifyRecorder{ResponseRecorder: rec}

	router.engine.ServeHTTP(closeRec, req)

	if closeRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", closeRec.Code, closeRec.Body.String())
	}
	if upstreamAuth != "Bearer sk-client" {
		t.Fatalf("upstream authorization = %q, want client authorization", upstreamAuth)
	}
	if closeRec.Header().Get("X-Aegis-Auth-Mode") != "passthrough" {
		t.Fatalf("auth mode header = %q", closeRec.Header().Get("X-Aegis-Auth-Mode"))
	}
}

func TestHandleProxyPassthroughWithoutUpstreamAuthorizationReturnsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	router := newV1ProxyTestRouter(t, upstream.URL, "passthrough", "")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("X-Gateway-Key", "agk-test-001")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	closeRec := &closeNotifyRecorder{ResponseRecorder: rec}

	router.engine.ServeHTTP(closeRec, req)

	if closeRec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", closeRec.Code, closeRec.Body.String())
	}
	if upstreamCalled {
		t.Fatal("upstream should not be called")
	}
	if !bytes.Contains(closeRec.Body.Bytes(), []byte("missing upstream Authorization")) {
		t.Fatalf("unexpected body: %s", closeRec.Body.String())
	}
}

func newV1ProxyTestRouter(t *testing.T, targetURL, authMode, llmAPIKey string) *Router {
	t.Helper()

	cfgPath := writeGatewayConfigForTest(t, targetURL, llmAPIKey)
	vkeyMgr, err := vkey.NewManager(zap.NewNop(), cfgPath)
	if err != nil {
		t.Fatalf("new vkey manager: %v", err)
	}

	proxy, err := gateway.NewAegisProxy(targetURL, vkeyMgr, nil, "strict", authMode, false, gates.TDGSettings{}, gates.ProvenanceSettings{}, false, zap.NewNop())
	if err != nil {
		t.Fatalf("new proxy: %v", err)
	}

	engine := gin.New()
	router := &Router{
		engine:    engine,
		proxy:     proxy,
		vkeyMgr:   vkeyMgr,
		auditor:   audit.NewLogger(&memoryAuditStore{}),
		logger:    zap.NewNop(),
		cfg:       config.Config{AuthMode: authMode, TokenMode: "strict"},
		targetURL: targetURL,
	}
	engine.Any("/v1/*path", router.handleProxy)
	return router
}

func writeGatewayConfigForTest(t *testing.T, targetURL, llmAPIKey string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gateway.yaml")
	content := "gateway_key: agk-test-001\n" +
		"target_url: " + targetURL + "\n" +
		"llm_api_key: " + llmAPIKey + "\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write gateway config: %v", err)
	}
	return cfgPath
}

type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
}

func (r *closeNotifyRecorder) CloseNotify() <-chan bool {
	ch := make(chan bool, 1)
	return ch
}
