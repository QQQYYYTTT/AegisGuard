package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aegisguard/internal/contract"
	"aegisguard/internal/gates"
	"aegisguard/internal/interfaces"
	"aegisguard/internal/vkey"

	"go.uber.org/zap"
)

type gateResult struct {
	Decision   gates.Decision
	Reason     string
	StatusCode int
	GateType   string
	RiskScore  int
	RiskLevel  string
	Rules      []string
}

type AegisProxy struct {
	target        *url.URL
	proxy         *httputil.ReverseProxy
	vkeyMgr       *vkey.Manager
	messageGate   MessageEvaluator
	actionGate    ActionEvaluator
	returnGate    ReturnEvaluator
	decisionStore *gates.DecisionStore
	tokenIssuer   TokenIssuer
	sandboxMgr    contract.SandboxManager
	transferMgr   contract.TransferManager
	contentFilter contract.ContentFilter
	logger        *zap.Logger
}

func NewAegisProxy(targetURL string, vkeyMgr *vkey.Manager, tokenIssuer TokenIssuer, logger *zap.Logger) (*AegisProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	ap := &AegisProxy{
		target:        target,
		vkeyMgr:       vkeyMgr,
		messageGate:   gates.NewMessageGate(),
		actionGate:    gates.NewActionGate(logger),
		returnGate:    gates.NewReturnGate(),
		decisionStore: gates.NewDecisionStore(1000),
		tokenIssuer:   tokenIssuer,
		logger:        logger,
	}

	ap.proxy = httputil.NewSingleHostReverseProxy(target)
	ap.proxy.Director = ap.director
	ap.proxy.ModifyResponse = ap.modifyResponse
	ap.proxy.ErrorHandler = ap.errorHandler

	return ap, nil
}

func (ap *AegisProxy) SetSandbox(sandboxMgr contract.SandboxManager, transferMgr contract.TransferManager, contentFilter contract.ContentFilter) {
	ap.sandboxMgr = sandboxMgr
	ap.transferMgr = transferMgr
	ap.contentFilter = contentFilter
}

func (ap *AegisProxy) director(req *http.Request) {
	originalAuth := req.Header.Get("Authorization")
	maskedAuth := originalAuth
	if len(originalAuth) > 30 {
		maskedAuth = originalAuth[:30] + "..."
	}

	req.Header.Set("Authorization", "Bearer "+ap.vkeyMgr.GetLLMAPIKey())
	req.Host = ap.target.Host
	req.URL.Scheme = ap.target.Scheme
	req.URL.Host = ap.target.Host

	ap.logger.Debug("gateway credential replaced",
		zap.String("original_auth", maskedAuth),
		zap.String("target_host", ap.target.Host),
		zap.String("target_path", req.URL.Path),
	)
}

func (ap *AegisProxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	ap.logger.Error("proxy forwarding failed",
		zap.String("target", ap.target.String()),
		zap.Error(err),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": "gateway forwarding failed: " + err.Error(),
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
	_ = json.Unmarshal(body, &req)
	return strings.Contains(path, "/tools") || hasToolCalls(req.Messages)
}

func hasToolCalls(messages []struct {
	ToolCalls []interface{} `json:"tool_calls"`
}) bool {
	for _, message := range messages {
		if len(message.ToolCalls) > 0 {
			return true
		}
	}
	return false
}

func (ap *AegisProxy) handleChatRequest(req *http.Request, body []byte) (gateResult, bool) {
	decision, reason := ap.messageGate.Evaluate(body)
	result := ap.newGateResult("message", decision, reason, http.StatusOK)
	ap.recordDecision(req, result, "", "")

	switch decision {
	case gates.Block, gates.Deny:
		ap.blockRequest(req, reason)
		result.StatusCode = http.StatusForbidden
		return result, false
	case gates.HumanApproval:
		ap.blockRequest(req, reason)
		result.StatusCode = http.StatusAccepted
		return result, false
	case gates.Degrade:
		ap.degradeRequest(req, body)
		return result, true
	case gates.Allow:
		ap.logger.Debug("Message Gate allow", zap.String("reason", reason))
	}
	return result, true
}

func (ap *AegisProxy) handleToolCall(req *http.Request, body []byte) (gateResult, bool) {
	toolName, params := ap.extractToolCall(body)
	if toolName == "" {
		result := ap.newGateResult("action", gates.Deny, "tool call detected but tool name is empty", http.StatusBadRequest)
		ap.recordDecision(req, result, toolName, "")
		return result, false
	}

	decision, reason := ap.actionGate.Evaluate(toolName, params, req.Header)
	result := ap.newGateResult("action", decision, reason, http.StatusOK)
	ap.recordDecision(req, result, toolName, "")

	switch decision {
	case gates.Block, gates.Deny:
		ap.denyToolCall(req, reason)
		result.StatusCode = http.StatusForbidden
		return result, false
	case gates.HumanApproval:
		ap.holdForApproval(req, toolName)
		result.StatusCode = http.StatusAccepted
		return result, false
	case gates.Degrade:
		ap.degradeRequest(req, body)
		return result, true
	case gates.Allow:
		if err := ap.injectToken(req, toolName, params); err != nil {
			ap.logger.Error("failed to issue or inject RequireToken", zap.String("tool", toolName), zap.Error(err))
			result := ap.newGateResult("action", gates.Deny, "failed to issue require token: "+err.Error(), http.StatusInternalServerError)
			ap.recordDecision(req, result, toolName, "")
			return result, false
		}
		return result, true
	default:
		ap.logger.Warn("unknown Action Gate decision",
			zap.Any("decision", decision),
			zap.String("tool", toolName),
		)
	}
	return result, true
}

func (ap *AegisProxy) newGateResult(gateType string, decision gates.Decision, reason string, statusCode int) gateResult {
	score, level, rules := gates.ExtractReasonMetadata(reason)
	return gateResult{
		Decision:   decision,
		Reason:     reason,
		StatusCode: statusCode,
		GateType:   gateType,
		RiskScore:  score,
		RiskLevel:  level,
		Rules:      rules,
	}
}

func (ap *AegisProxy) recordDecision(req *http.Request, result gateResult, toolName, agentID string) {
	if ap.decisionStore == nil {
		return
	}
	ap.decisionStore.Add(interfaces.GateDecision{
		RequestID:    requestIDFromContext(req),
		Timestamp:    time.Now(),
		GateType:     result.GateType,
		Decision:     result.Decision.String(),
		RiskScore:    result.RiskScore,
		RiskLevel:    result.RiskLevel,
		MatchedRules: result.Rules,
		Reason:       result.Reason,
		ToolName:     toolName,
		AgentID:      agentID,
	})
}

func requestIDFromContext(req *http.Request) string {
	if req == nil {
		return "unknown"
	}
	if requestID, ok := req.Context().Value("request_id").(string); ok && requestID != "" {
		return requestID
	}
	return "unknown"
}

func (ap *AegisProxy) blockRequest(req *http.Request, reason string) {
	ap.logger.Warn("request blocked",
		zap.String("reason", reason),
		zap.String("path", req.URL.Path),
	)
}

func (ap *AegisProxy) degradeRequest(req *http.Request, body []byte) {
	ap.logger.Info("request degraded", zap.String("path", req.URL.Path))

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		req.Header.Set("X-Aegis-Degraded", "true")
		return
	}

	payload["tool_choice"] = "none"
	payload["tools"] = []interface{}{}
	if messages, ok := payload["messages"].([]interface{}); ok {
		for _, item := range messages {
			message, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if content, ok := message["content"].(string); ok {
				message["content"] = redactGatewayText(content)
			}
		}
	}

	nextBody, err := json.Marshal(payload)
	if err != nil {
		req.Header.Set("X-Aegis-Degraded", "true")
		return
	}
	req.Body = io.NopCloser(bytes.NewBuffer(nextBody))
	req.ContentLength = int64(len(nextBody))
	req.Header.Set("Content-Length", strconv.Itoa(len(nextBody)))
	req.Header.Set("X-Aegis-Degraded", "true")
}

func redactGatewayText(text string) string {
	replacements := []string{
		`(?i)\bsk-[A-Za-z0-9_-]{8,}\b`,
		`(?i)\bBearer\s+[A-Za-z0-9._-]{12,}\b`,
		`(?i)\b(password|passwd|api[_-]?key|secret|token)\s*[:=]\s*("[^"]*"|'[^']*'|[^\s,;}]+)`,
	}
	safe := text
	for _, pattern := range replacements {
		safe = regexpReplace(pattern, safe, "[REDACTED]")
	}
	safe = regexpReplace(`(?i)\b(ignore|bypass|override)\b.{0,80}\b(system|developer|previous|prior|policy|instruction)s?\b`, safe, "[FILTERED_POLICY_TEXT]")
	return safe
}

func regexpReplace(pattern, text, replacement string) string {
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(text, replacement)
}

func (ap *AegisProxy) denyToolCall(req *http.Request, reason string) {
	ap.logger.Warn("tool call denied",
		zap.String("reason", reason),
		zap.String("path", req.URL.Path),
	)
}

func (ap *AegisProxy) holdForApproval(req *http.Request, toolName string) {
	ap.logger.Info("tool call waiting for human approval",
		zap.String("tool", toolName),
		zap.String("path", req.URL.Path),
	)
}

func (ap *AegisProxy) injectToken(req *http.Request, toolName string, params map[string]interface{}) error {
	if ap.tokenIssuer == nil {
		req.Header.Set("X-Aegis-Token-Status", "skipped")
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

	ap.logger.Debug("RequireToken injected",
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
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	_ = json.Unmarshal(body, &req)

	for _, message := range req.Messages {
		if len(message.ToolCalls) == 0 {
			continue
		}
		tc := message.ToolCalls[0]
		params := map[string]interface{}{}
		if len(tc.Function.Arguments) > 0 {
			if err := json.Unmarshal(tc.Function.Arguments, &params); err != nil {
				var argString string
				if stringErr := json.Unmarshal(tc.Function.Arguments, &argString); stringErr == nil {
					if mapErr := json.Unmarshal([]byte(argString), &params); mapErr != nil {
						params["raw_arguments"] = argString
					}
				} else {
					params["raw_arguments"] = string(tc.Function.Arguments)
				}
			}
		}
		return tc.Function.Name, params
	}
	return "", nil
}

func (ap *AegisProxy) modifyResponse(resp *http.Response) error {
	if resp.Body == nil || ap.returnGate == nil {
		return nil
	}
	if encoding := resp.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		ap.logger.Debug("Return Gate skipped encoded response", zap.String("content_encoding", encoding))
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	decision, reason := ap.returnGate.Evaluate(body)
	result := ap.newGateResult("return", decision, reason, resp.StatusCode)
	ap.recordDecision(resp.Request, result, "", "")
	setGateHeaders(resp.Header, result)

	switch decision {
	case gates.Block, gates.Deny:
		ap.logger.Warn("response blocked by Return Gate", zap.String("reason", reason))
		ap.captureSandboxResponse(resp, body, result, false)
		resp.StatusCode = http.StatusForbidden
		resp.Status = "403 Forbidden"
		body = gateResponseBody(decision, reason)
	case gates.Degrade:
		ap.logger.Info("response filtered by Return Gate", zap.String("reason", reason))
		ap.captureSandboxResponse(resp, body, result, true)
		if ap.contentFilter != nil {
			filtered, removed := ap.contentFilter.FilterToolResponse(body)
			body = filtered
			if len(removed) > 0 {
				resp.Header.Set("X-Aegis-Filtered-Fields", strings.Join(removed, ","))
			}
		} else {
			body = ap.returnGate.Filter(body)
		}
		resp.Header.Set("X-Aegis-Filtered", "true")
	default:
		ap.logger.Debug("Return Gate allow", zap.String("reason", reason))
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

func (ap *AegisProxy) captureSandboxResponse(resp *http.Response, body []byte, result gateResult, promoteSummary bool) {
	if ap.sandboxMgr == nil || resp == nil {
		return
	}

	requestID := requestIDFromContext(resp.Request)
	trusted := interfaces.TrustedContent{
		SystemPrompt: "AegisGuard trusted core context",
		Memory:       "request_id=" + requestID,
		TaskState:    "return_gate_decision=" + result.Decision.String(),
	}
	untrusted := interfaces.UntrustedContent{
		ExternalData:    string(body),
		InjectedContent: result.Reason,
		Source:          "return_gate",
		ContentType:     contentTypeFromHeader(resp.Header.Get("Content-Type")),
	}

	ctx, err := ap.sandboxMgr.CreateContext(trusted, untrusted)
	if err != nil {
		ap.logger.Warn("failed to create sandbox context", zap.Error(err))
		return
	}

	resp.Header.Set("X-Aegis-Sandbox-Context-ID", ctx.ContextID)
	resp.Header.Set("X-Aegis-Sandbox-Status", ctx.Status)
	resp.Header.Set("X-Aegis-Sandbox-Fingerprint", ctx.SM3Fingerprint)

	if !promoteSummary {
		if ap.transferMgr != nil {
			if quarantineRecorder, ok := ap.transferMgr.(interface {
				RecordQuarantine(contextID string, data interfaces.UntrustedContent, reason string) (*interfaces.TransferRecord, error)
			}); ok {
				if record, err := quarantineRecorder.RecordQuarantine(ctx.ContextID, untrusted, result.Reason); err == nil {
					resp.Header.Set("X-Aegis-Sandbox-Transfer-ID", record.ID)
					resp.Header.Set("X-Aegis-Sandbox-Approved", strconv.FormatBool(record.Approved))
				}
			}
		}
		return
	}
	if ap.transferMgr == nil {
		return
	}

	record, err := ap.transferMgr.UntrustedToTrusted(ctx.ContextID, untrusted)
	if err != nil {
		ap.logger.Warn("failed to record sandbox transfer",
			zap.String("context_id", ctx.ContextID),
			zap.Error(err),
		)
		return
	}
	resp.Header.Set("X-Aegis-Sandbox-Transfer-ID", record.ID)
	resp.Header.Set("X-Aegis-Sandbox-Approved", strconv.FormatBool(record.Approved))
}

func contentTypeFromHeader(value string) string {
	if value == "" {
		return "application/octet-stream"
	}
	return strings.Split(value, ";")[0]
}

func (ap *AegisProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		result := ap.newGateResult("message", gates.Block, "failed to read request body", http.StatusBadRequest)
		ap.writeGateResponse(w, result)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	if ap.isChatCompletion(r.URL.Path) {
		result, ok := ap.handleChatRequest(r, body)
		setGateHeaders(w.Header(), result)
		if !ok {
			ap.writeGateResponse(w, result)
			return
		}
	} else if ap.isToolCall(r.URL.Path, body) {
		result, ok := ap.handleToolCall(r, body)
		setGateHeaders(w.Header(), result)
		if !ok {
			ap.writeGateResponse(w, result)
			return
		}
	}

	ap.proxy.ServeHTTP(w, r)
}

func (ap *AegisProxy) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ap.ServeHTTP(w, r)
	}
}

func (ap *AegisProxy) writeGateResponse(w http.ResponseWriter, result gateResult) {
	w.Header().Set("Content-Type", "application/json")
	setGateHeaders(w.Header(), result)
	w.WriteHeader(result.StatusCode)
	_, _ = w.Write(gateResponseBody(result.Decision, result.Reason))
}

func setGateHeaders(header http.Header, result gateResult) {
	header.Set("X-Aegis-Decision", result.Decision.String())
	header.Set("X-Aegis-Gate-Type", result.GateType)
	header.Set("X-Aegis-Reason", result.Reason)
	header.Set("X-Aegis-Risk-Score", strconv.Itoa(result.RiskScore))
	header.Set("X-Aegis-Risk-Level", result.RiskLevel)
	header.Set("X-Aegis-Matched-Rules", strings.Join(result.Rules, ","))
}

func gateResponseBody(decision gates.Decision, reason string) []byte {
	body, _ := json.Marshal(map[string]interface{}{
		"error": map[string]interface{}{
			"message":  "request stopped by AegisGuard",
			"type":     "aegis_gate_decision",
			"decision": decision.String(),
			"reason":   reason,
		},
	})
	return body
}

func (ap *AegisProxy) GetDecisionStore() *gates.DecisionStore {
	return ap.decisionStore
}
