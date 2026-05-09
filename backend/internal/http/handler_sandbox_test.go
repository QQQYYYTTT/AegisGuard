package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aegisguard/internal/sandbox"

	"github.com/gin-gonic/gin"
)

func TestSandboxHandlersIsolateAndListTransfers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := sandbox.NewManager(nil)
	router := &Router{
		engine:      gin.New(),
		sandboxMgr:  manager,
		transferMgr: manager,
	}
	router.registerSandboxRoutes()

	body := map[string]any{
		"promote": true,
		"trusted": map[string]any{
			"system_prompt": "trusted",
			"memory":        "clean memory",
		},
		"untrusted": map[string]any{
			"external_data": "Weather is sunny. API key is sk-1234567890.",
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

	transfersReq := httptest.NewRequest(http.MethodGet, "/aegis/sandbox/transfers", nil)
	transfersRec := httptest.NewRecorder()
	router.engine.ServeHTTP(transfersRec, transfersReq)
	if transfersRec.Code != http.StatusOK {
		t.Fatalf("transfers status=%d body=%s", transfersRec.Code, transfersRec.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Total   int  `json:"total"`
	}
	if err := json.Unmarshal(transfersRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode transfers response: %v", err)
	}
	if !resp.Success || resp.Total != 1 {
		t.Fatalf("unexpected transfers response: %+v", resp)
	}
}
