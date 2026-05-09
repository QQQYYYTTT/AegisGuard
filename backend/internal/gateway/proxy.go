package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"aegisguard/internal/gates"
	"aegisguard/internal/vkey"

	"go.uber.org/zap"
)

type AegisProxy struct {
	target      *url.URL
	proxy       *httputil.ReverseProxy
	vkeyMgr     *vkey.Manager
	messageGate MessageEvaluator
	actionGate  ActionEvaluator
	tokenIssuer TokenIssuer
	logger      *zap.Logger
}

func NewAegisProxy(targetURL string, vkeyMgr *vkey.Manager, tokenIssuer TokenIssuer, logger *zap.Logger) (*AegisProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	ap := &AegisProxy{
		target:      target,
		vkeyMgr:     vkeyMgr,
		messageGate: gates.NewMessageGate(),
		actionGate:  gates.NewActionGate(logger),
		tokenIssuer: tokenIssuer,
		logger:      logger,
	}

	ap.proxy = httputil.NewSingleHostReverseProxy(target)
	ap.proxy.Director = ap.director
	ap.proxy.ErrorHandler = ap.errorHandler

	return ap, nil
}

func (ap *AegisProxy) director(req *http.Request) {
	originalAuth := req.Header.Get("Authorization")
	maskedAuth := originalAuth
	if len(originalAuth) > 30 {
		maskedAuth = originalAuth[:30] + "..."
	}

	llmAPIKey := ap.vkeyMgr.GetLLMAPIKey()

	req.Header.Set("Authorization", "Bearer "+llmAPIKey)

	ap.logger.Debug("密钥替换完成",
		zap.String("original_auth", maskedAuth),
		zap.String("target_host", ap.target.Host),
		zap.String("target_path", req.URL.Path),
	)

	req.Host = ap.target.Host
	req.URL.Scheme = ap.target.Scheme
	req.URL.Host = ap.target.Host
}

func (ap *AegisProxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	ap.logger.Error("代理转发失败",
		zap.String("target", ap.target.String()),
		zap.Error(err),
	)
	w.WriteHeader(http.StatusBadGateway)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": "网关转发失败: " + err.Error(),
			"type":    "gateway_error",
		},
	})
}

func (ap *AegisProxy) isChatCompletion(path string) bool {
	return strings.Contains(path, "/chat/completions")
}

func (ap *AegisProxy) isToolCall(path string, body []byte) bool {
	var req struct {
		Messages []struct {
			ToolCalls []interface{} `json:"tool_calls"`
		} `json:"messages"`
	}
	json.Unmarshal(body, &req)
	return strings.Contains(path, "/tools") || len(req.Messages) > 0
}

func (ap *AegisProxy) handleChatRequest(req *http.Request, body []byte) (int, map[string]interface{}, bool) {
	decision, reason := ap.messageGate.Evaluate(body)
	switch decision {
	case gates.Block:
		return ap.blockRequest(req, reason)
	case gates.Degrade:
		ap.degradeRequest(req, body)
	case gates.Allow:
		ap.logger.Debug("Message Gate 放行")
	}
	return 0, nil, false
}

func (ap *AegisProxy) handleToolCall(req *http.Request, body []byte) (int, map[string]interface{}, bool) {
	toolName, params := ap.extractToolCall(body)
	if toolName == "" {
		return http.StatusBadRequest, map[string]interface{}{
			"error": map[string]interface{}{
				"message": "tool call detected but tool name is empty",
				"type":    "invalid_tool_call",
			},
		}, true
	}

	if err := ap.injectToken(req, toolName, params); err != nil {
		ap.logger.Error("签发或注入 RequireToken 失败", zap.String("tool", toolName), zap.Error(err))
		return http.StatusInternalServerError, map[string]interface{}{
			"error": map[string]interface{}{
				"message": "failed to issue require token: " + err.Error(),
				"type":    "token_issue_error",
			},
		}, true
	}

	decision, reason := ap.actionGate.Evaluate(toolName, params, req.Header)
	switch decision {
	case gates.Deny:
		return ap.denyToolCall(req, reason)
	case gates.HumanApproval:
		return ap.holdForApproval(req, toolName)
	case gates.Allow:
		return 0, nil, false
	default:
		ap.logger.Warn("未知的 Action Gate 决策状态",
			zap.Any("decision", decision),
			zap.String("tool", toolName),
		)
	}
	return 0, nil, false
}

func (ap *AegisProxy) blockRequest(req *http.Request, reason string) (int, map[string]interface{}, bool) {
	ap.logger.Warn("请求被阻断",
		zap.String("reason", reason),
		zap.String("path", req.URL.Path),
	)
	return http.StatusForbidden, map[string]interface{}{
		"error": map[string]interface{}{
			"message": reason,
			"type":    "message_gate_blocked",
		},
	}, true
}

func (ap *AegisProxy) degradeRequest(req *http.Request, body []byte) {
	ap.logger.Info("请求降级：限制工具权限",
		zap.String("path", req.URL.Path),
	)
}

func (ap *AegisProxy) denyToolCall(req *http.Request, reason string) (int, map[string]interface{}, bool) {
	ap.logger.Warn("工具调用被拒绝",
		zap.String("reason", reason),
		zap.String("path", req.URL.Path),
	)
	return http.StatusForbidden, map[string]interface{}{
		"error": map[string]interface{}{
			"message": reason,
			"type":    "tool_call_denied",
		},
	}, true
}

func (ap *AegisProxy) holdForApproval(req *http.Request, toolName string) (int, map[string]interface{}, bool) {
	ap.logger.Info("工具调用等待人工审批",
		zap.String("tool", toolName),
		zap.String("path", req.URL.Path),
	)
	return http.StatusForbidden, map[string]interface{}{
		"error": map[string]interface{}{
			"message": "tool call requires human approval",
			"type":    "human_approval_required",
		},
	}, true
}

func (ap *AegisProxy) injectToken(req *http.Request, toolName string, params map[string]interface{}) error {
	if ap.tokenIssuer == nil {
		return nil
	}

	requestID, _ := req.Context().Value("request_id").(string)
	gatewayKey, _ := req.Context().Value("gateway_key").(string)
	if requestID == "" {
		requestID = "request-anonymous"
	}
	if gatewayKey == "" {
		gatewayKey = "agent-anonymous"
	}

	scope := toolName + ":invoke"
	token, err := ap.tokenIssuer.Issue(toolName, scope, gatewayKey, requestID, requestID, 5*time.Minute, 1)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(token)
	if err != nil {
		return err
	}
	req.Header.Set("X-Aegis-Token", string(payload))

	ap.logger.Debug("注入 RequireToken",
		zap.String("tool", toolName),
		zap.String("token_id", token.Nonce),
	)
	return nil
}

func (ap *AegisProxy) extractToolCall(body []byte) (string, map[string]interface{}) {
	var req struct {
		Messages []struct {
			ToolCalls []struct {
				Function struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	json.Unmarshal(body, &req)

	if len(req.Messages) > 0 && len(req.Messages[0].ToolCalls) > 0 {
		tc := req.Messages[0].ToolCalls[0]
		return tc.Function.Name, tc.Function.Arguments
	}
	return "", nil
}

func (ap *AegisProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ap.logger.Error("读取请求体失败", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "failed to read request body",
				"type":    "bad_request",
			},
		})
		return
	}

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	if ap.isChatCompletion(r.URL.Path) {
		if status, payload, blocked := ap.handleChatRequest(r, body); blocked {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(payload)
			return
		}
	} else if ap.isToolCall(r.URL.Path, body) {
		if status, payload, blocked := ap.handleToolCall(r, body); blocked {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(payload)
			return
		}
	}

	r.Body = io.NopCloser(bytes.NewBuffer(body))
	ap.proxy.ServeHTTP(w, r)
}

func (ap *AegisProxy) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ap.ServeHTTP(w, r)
	}
}
