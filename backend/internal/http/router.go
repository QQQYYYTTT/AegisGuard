package http

import (
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"aegisguard/backend/internal/attacks"
	"aegisguard/backend/internal/audit"
	"aegisguard/backend/internal/catalog"
	"aegisguard/backend/internal/config"
	"aegisguard/backend/internal/runtime"
	"aegisguard/backend/internal/security"
)

func NewRouter(cfg config.Config) (stdhttp.Handler, error) {
	auditStore, err := audit.NewStore(cfg.AuditFile)
	if err != nil {
		return nil, err
	}
	runtimeService := runtime.NewService(auditStore)

	mux := stdhttp.NewServeMux()

	mux.HandleFunc("/api/health", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		writeJSON(w, stdhttp.StatusOK, map[string]any{"ok": true, "service": "aegisguard-backend", "stage": "go-modular"})
	})
	mux.HandleFunc("/api/agents", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		writeJSON(w, stdhttp.StatusOK, map[string]any{"items": catalog.Agents})
	})
	mux.HandleFunc("/api/attack-families", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		writeJSON(w, stdhttp.StatusOK, map[string]any{"items": catalog.AttackFamilies})
	})
	mux.HandleFunc("/api/attack-library", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		writeJSON(w, stdhttp.StatusOK, attacks.GetLibrary())
	})
	mux.HandleFunc("/api/experiment-layers", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		writeJSON(w, stdhttp.StatusOK, map[string]any{"items": catalog.Layers})
	})
	mux.HandleFunc("/api/experiment-plan", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		writeJSON(w, stdhttp.StatusOK, catalog.BuildPlan())
	})
	mux.HandleFunc("/api/scenarios", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		writeJSON(w, stdhttp.StatusOK, map[string]any{"scenarios": catalog.ScenarioTemplates})
	})
	mux.HandleFunc("/api/experiment-assets", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		writeJSON(w, stdhttp.StatusOK, catalog.Assets)
	})
	mux.HandleFunc("/api/audit", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			items, err := runtimeService.ListAudit()
			if err != nil {
				writeError(w, stdhttp.StatusInternalServerError, err)
				return
			}
			writeJSON(w, stdhttp.StatusOK, map[string]any{"items": items})
		case stdhttp.MethodDelete:
			if err := runtimeService.ClearAudit(); err != nil {
				writeError(w, stdhttp.StatusInternalServerError, err)
				return
			}
			writeJSON(w, stdhttp.StatusOK, map[string]any{"ok": true})
		default:
			w.WriteHeader(stdhttp.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/issue-token", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			w.WriteHeader(stdhttp.StatusMethodNotAllowed)
			return
		}
		var body security.IssueTokenRequest
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, stdhttp.StatusBadRequest, err)
			return
		}
		writeJSON(w, stdhttp.StatusOK, map[string]any{"token": security.IssueRequireToken(body)})
	})
	mux.HandleFunc("/api/verify-request", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			w.WriteHeader(stdhttp.StatusMethodNotAllowed)
			return
		}
		var body security.VerifyRequestBody
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, stdhttp.StatusBadRequest, err)
			return
		}
		result, err := runtimeService.EvaluateProtectedRequest(body)
		if err != nil {
			writeError(w, stdhttp.StatusInternalServerError, err)
			return
		}
		writeJSON(w, stdhttp.StatusOK, result)
	})

	fileServer := stdhttp.FileServer(stdhttp.Dir(cfg.FrontendDir))
	mux.HandleFunc("/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.WriteHeader(stdhttp.StatusNotFound)
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		requested := filepath.Join(cfg.FrontendDir, cleanPath)
		indexFile := filepath.Join(cfg.FrontendDir, "index.html")
		if r.URL.Path == "/" {
			stdhttp.ServeFile(w, r, indexFile)
			return
		}

		info, err := os.Stat(requested)
		if err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		stdhttp.ServeFile(w, r, indexFile)
	})

	return mux, nil
}

func decodeJSON(r *stdhttp.Request, dest any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("empty request body")
	}
	return json.Unmarshal(body, dest)
}

func writeJSON(w stdhttp.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w stdhttp.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}
