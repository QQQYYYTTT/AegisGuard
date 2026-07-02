package gateway

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	"aegisguard/pkg/smcrypto"

	"go.uber.org/zap"
)

type gateResult struct {
	Decision          gates.Decision
	Reason            string
	StatusCode        int
	GateType          string
	RiskScore         int
	RiskLevel         string
	Rules             []string
	TokenStatus       string
	AuthMode          string
	UnauthorizedAllow bool
}

type AegisProxy struct {
	target        *url.URL
	proxy         *httputil.ReverseProxy
	vkeyMgr       *vkey.Manager
	messageGate   MessageEvaluator
	actionGate    ActionEvaluator
	returnGate    ReturnEvaluator
	tokenIssuer   TokenIssuer
	decisionStore *gates.DecisionStore
	sandboxMgr    contract.SandboxManager
	transferMgr   contract.TransferManager
	contentFilter contract.ContentFilter
	tokenMode     string
	authMode      string
	logger        *zap.Logger

	provenanceEnabled bool
	provenanceMode    string

	// purificationEnabled 决定 modifyResponse 是否在 Allow 分支也调用 contentFilter
	// 三态纯化（Phase 4，借鉴 Structured Purification 思想）。默认关闭时 Allow 分支
	// 保持逐字节透传，与 Phase 4 之前完全一致；具体的 log-only/enforce 行为由
	// contentFilter（sandbox.Manager）自身的开关决定，此处只负责"要不要调用"。
	purificationEnabled bool
}

func NewAegisProxy(targetURL string, vkeyMgr *vkey.Manager, tokenIssuer TokenIssuer, tokenMode, authMode string, dynamicRuleRouting bool, tdgSettings gates.TDGSettings, provenanceSettings gates.ProvenanceSettings, purificationEnabled bool, logger *zap.Logger) (*AegisProxy, error) {
	return NewAegisProxyWithPolicyRuntime(targetURL, vkeyMgr, tokenIssuer, tokenMode, authMode, dynamicRuleRouting, tdgSettings, provenanceSettings, purificationEnabled, logger, nil)
}

func NewAegisProxyWithPolicyRuntime(targetURL string, vkeyMgr *vkey.Manager, tokenIssuer TokenIssuer, tokenMode, authMode string, dynamicRuleRouting bool, tdgSettings gates.TDGSettings, provenanceSettings gates.ProvenanceSettings, purificationEnabled bool, logger *zap.Logger, policyRuntime *gates.PolicyRuntime) (*AegisProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	actionGate := gates.NewActionGateWithRuntimeAndStore(logger, tokenMode, policyRuntime, tokenIssuer)
	actionGate.SetDynamicRuleRouting(dynamicRuleRouting)
	actionGate.SetTDG(tdgSettings)

	ap := &AegisProxy{
		target:            target,
		vkeyMgr:           vkeyMgr,
		messageGate:       gates.NewMessageGateWithRuntime(policyRuntime),
		actionGate:        actionGate,
		returnGate:        gates.NewReturnGateWithRuntime(policyRuntime),
		tokenIssuer:       tokenIssuer,
		decisionStore:     gates.NewDecisionStore(1000),
		tokenMode:         normalizeGatewayTokenMode(tokenMode),
		authMode:          normalizeUpstreamAuthMode(authMode),
		logger:            logger,
		provenanceEnabled: provenanceSettings.Enabled,
		provenanceMode:    gates.NormalizeProvenanceMode(provenanceSettings.Mode),

		purificationEnabled: purificationEnabled,
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

	switch ap.authMode {
	case "passthrough":
		if clientAuth := strings.TrimSpace(req.Header.Get("Authorization")); clientAuth != "" {
			if vkey.ExtractGatewayKey(clientAuth) != "" {
				req.Header.Del("Authorization")
			}
		}
		if strings.TrimSpace(req.Header.Get("Authorization")) == "" {
			if fallback := strings.TrimSpace(ap.vkeyMgr.GetLLMAPIKey()); fallback != "" {
				req.Header.Set("Authorization", "Bearer "+fallback)
			}
		}
	default:
		req.Header.Set("Authorization", "Bearer "+ap.vkeyMgr.GetLLMAPIKey())
	}
	req.Host = ap.target.Host
	req.URL.Scheme = ap.target.Scheme
	req.URL.Host = ap.target.Host

	ap.logger.Debug("upstream authorization prepared",
		zap.String("auth_mode", ap.authMode),
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
			Role      string        `json:"role"`
			ToolCalls []interface{} `json:"tool_calls"`
		} `json:"messages"`
	}
	_ = json.Unmarshal(body, &req)
	return strings.Contains(path, "/tools") || hasPendingToolCalls(req.Messages)
}

func hasPendingToolCalls(messages []struct {
	Role      string        `json:"role"`
	ToolCalls []interface{} `json:"tool_calls"`
}) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if len(message.ToolCalls) > 0 {
			if strings.EqualFold(strings.TrimSpace(message.Role), "assistant") || strings.TrimSpace(message.Role) == "" {
				return true
			}
		}
		if strings.EqualFold(strings.TrimSpace(message.Role), "tool") {
			return false
		}
	}
	return false
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
	if missingReason := ap.missingUpstreamAuthorizationReason(req); missingReason != "" {
		er := interfaces.EvaluateResult{Decision: gates.Deny, Reason: missingReason}
		gr := ap.newGateResult("message", er, http.StatusUnauthorized)
		ap.recordDecision(req, gr, "", "")
		ap.blockRequest(req, missingReason)
		return gr, false
	}

	result := ap.messageGate.Evaluate(body)
	gr := ap.newGateResult("message", result, http.StatusOK)
	ap.recordDecision(req, gr, "", "")

	switch result.Decision {
	case gates.Block, gates.Deny:
		ap.blockRequest(req, result.Reason)
		gr.StatusCode = http.StatusForbidden
		return gr, false
	case gates.HumanApproval:
		ap.blockRequest(req, result.Reason)
		gr.StatusCode = http.StatusAccepted
		return gr, false
	case gates.Degrade:
		ap.degradeRequest(req, body)
		return gr, true
	case gates.Allow:
		ap.logger.Debug("message gate allow", zap.String("reason", result.Reason))
	}
	return gr, true
}

func (ap *AegisProxy) handleToolCall(req *http.Request, body []byte) (gateResult, bool) {
	toolName, params := ap.extractToolCall(body)
	if toolName == "" {
		er := interfaces.EvaluateResult{Decision: gates.Deny, Reason: "tool call detected but tool name is empty"}
		result := ap.newGateResult("action", er, http.StatusBadRequest)
		result.TokenStatus = firstTokenStatus(req.Header)
		ap.recordDecision(req, result, "", "")
		return result, false
	}

	agentID, sessionID, taskID := ap.ensureExecutionContextHeaders(req)
	if err := ap.injectToken(req, toolName, params, body, agentID, sessionID, taskID); err != nil {
		ap.logger.Error("failed to issue or inject RequireToken", zap.String("tool", toolName), zap.Error(err))
		req.Header.Set("X-Aegis-Token-Status", "error")
	}

	if traceID := ap.resolveTraceID(req, body, sessionID, taskID, agentID); traceID != "" {
		req.Header.Set("X-Aegis-Trace-ID", traceID)
	}
	// X-Aegis-Tool-Name 供 modifyResponse 在响应阶段读回工具名，供 Phase 4 三态纯化引擎
	// 按工具名分表匹配白名单——响应阶段本身不携带工具名信息，只能靠请求阶段这样透传。
	req.Header.Set("X-Aegis-Tool-Name", toolName)

	result := ap.actionGate.Evaluate(toolName, params, req.Header)
	gr := ap.newGateResult("action", result, http.StatusOK)
	gr.TokenStatus = firstTokenStatus(req.Header)
	gr.UnauthorizedAllow = result.Decision == gates.Allow && req.Header.Get("X-Aegis-Token") == "" && ap.tokenMode != "strict"

	// 溯源校验与其它信号并行运行、不受它们先前结论限制：即便 PolicyEngine/TDG 已经判定
	// HumanApproval/Degrade，一旦确认某个高危参数无法溯源（即最直接的注入证据），enforce
	// 模式下也应直接升级为 Deny，而不是停留在"等待人工审批"这类更弱的处置上。
	if ap.provenanceEnabled {
		if violations := gates.CheckProvenance(toolName, params, body); len(violations) > 0 {
			reason := provenanceViolationReason(violations)
			ap.logger.Warn("provenance violation",
				zap.String("tool", toolName),
				zap.String("mode", ap.provenanceMode),
				zap.String("reason", reason),
			)
			if ap.provenanceMode == "enforce" {
				result.Decision = gates.Deny
				result.Reason = reason
				gr.Decision = result.Decision
				gr.Reason = result.Reason
			}
		}
	}

	ap.recordDecision(req, gr, toolName, "")

	switch result.Decision {
	case gates.Block, gates.Deny:
		ap.denyToolCall(req, result.Reason)
		gr.StatusCode = http.StatusForbidden
		return gr, false
	case gates.HumanApproval:
		ap.holdForApproval(req, toolName)
		gr.StatusCode = http.StatusAccepted
		return gr, false
	case gates.Degrade:
		ap.degradeRequest(req, body)
		return gr, true
	case gates.Allow:
		return gr, true
	default:
		ap.logger.Warn("unknown action gate decision",
			zap.Any("decision", result.Decision),
			zap.String("tool", toolName),
		)
	}
	return gr, true
}

// provenanceViolationReason 将参数溯源校验失败的明细拼接成人类可读的 Deny 原因。
func provenanceViolationReason(violations []gates.ProvenanceViolation) string {
	parts := make([]string, 0, len(violations))
	for _, v := range violations {
		parts = append(parts, fmt.Sprintf("param %q of tool %q has no valid provenance: %s", v.Param, v.ToolName, v.Reason))
	}
	return strings.Join(parts, "; ")
}

func (ap *AegisProxy) newGateResult(gateType string, er interfaces.EvaluateResult, statusCode int) gateResult {
	return gateResult{
		Decision:   er.Decision,
		Reason:     er.Reason,
		StatusCode: statusCode,
		GateType:   gateType,
		RiskScore:  er.RiskScore,
		RiskLevel:  er.RiskLevel,
		Rules:      er.MatchedRules,
		AuthMode:   ap.authMode,
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
		Decision:     result.Decision,
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
	ap.logger.Warn("request blocked", zap.String("reason", reason), zap.String("path", req.URL.Path))
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
	ap.logger.Warn("tool call denied", zap.String("reason", reason), zap.String("path", req.URL.Path))
}

func (ap *AegisProxy) holdForApproval(req *http.Request, toolName string) {
	ap.logger.Info("tool call waiting for human approval", zap.String("tool", toolName), zap.String("path", req.URL.Path))
}

func (ap *AegisProxy) injectToken(req *http.Request, toolName string, params map[string]interface{}, body []byte, agentID, sessionID, taskID string) error {
	if ap.tokenIssuer == nil {
		req.Header.Set("X-Aegis-Token-Status", "skipped")
		ap.logger.Debug("requiretoken skipped", zap.String("tool", toolName))
		return nil
	}

	scope := toolName + ":invoke"
	token, err := ap.tokenIssuer.Issue(toolName, scope, agentID, sessionID, taskID, 5*time.Minute, 1)
	if err != nil {
		return err
	}

	if toolSchema := ap.extractToolSchema(body, toolName); len(toolSchema) > 0 {
		token.SchemaHash = smcrypto.SM3Hex(toolSchema)
		if err := token.Sign(); err != nil {
			return err
		}
		if err := ap.tokenIssuer.Save(token); err != nil {
			return err
		}
		req.Header.Set("X-Aegis-Tool-Schema", base64.StdEncoding.EncodeToString(toolSchema))
	}

	payload, err := json.Marshal(token)
	if err != nil {
		return err
	}
	req.Header.Set("X-Aegis-Token", string(payload))
	req.Header.Set("X-Aegis-Token-Status", "issued")

	ap.logger.Debug("requiretoken injected",
		zap.String("tool", toolName),
		zap.String("token_id", token.Nonce),
	)
	return nil
}

func (ap *AegisProxy) ensureExecutionContextHeaders(req *http.Request) (agentID, sessionID, taskID string) {
	requestID, _ := req.Context().Value("request_id").(string)
	gatewayKey, _ := req.Context().Value("gateway_key").(string)

	agentID = strings.TrimSpace(req.Header.Get("X-Aegis-Agent-ID"))
	if agentID == "" {
		agentID = gatewayKey
	}
	if agentID == "" {
		agentID = "agent-anonymous"
	}

	sessionID = strings.TrimSpace(req.Header.Get("X-Aegis-Session-ID"))
	if sessionID == "" {
		sessionID = requestID
	}
	if sessionID == "" {
		sessionID = "session-anonymous"
	}

	taskID = strings.TrimSpace(req.Header.Get("X-Aegis-Task-ID"))
	if taskID == "" {
		taskID = requestID
	}
	if taskID == "" {
		taskID = "task-anonymous"
	}

	req.Header.Set("X-Aegis-Agent-ID", agentID)
	req.Header.Set("X-Aegis-Session-ID", sessionID)
	req.Header.Set("X-Aegis-Task-ID", taskID)
	return agentID, sessionID, taskID
}

func (ap *AegisProxy) extractToolSchema(body []byte, toolName string) []byte {
	var req struct {
		Tools []struct {
			Type     string          `json:"type"`
			Function json.RawMessage `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(body, &req); err != nil || len(req.Tools) == 0 {
		return nil
	}

	for _, tool := range req.Tools {
		if len(tool.Function) == 0 {
			continue
		}
		var fn struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(tool.Function, &fn); err != nil {
			continue
		}
		if fn.Name != toolName {
			continue
		}
		return tool.Function
	}
	return nil
}

// extractToolCall 从请求体里取出"当前待处理"的工具调用。
//
// 工程审计发现的既有缺陷（在实现 Phase 3 参数溯源时暴露，非本次改动引入）：
// /v1/chat/completions 每轮请求都会携带该任务此前全部历史消息，一旦某个任务已经完成过
// 一次工具调用（历史里已经有一条 assistant tool_calls 消息 + 对应的 tool 回执），
// 再次发起新的工具调用时，请求体里会同时存在"已执行的历史调用"和"本次待评估的新调用"两条
// tool_calls 消息。原实现遍历时命中第一条就直接返回，等于永远只评估任务的第一次工具调用，
// 后续调用全部被误判/绕过风控。修正为返回消息数组中**最后一条**带 tool_calls 的消息——
// 即时间上最新、且后面没有对应 tool 回执跟随的那一条，这才是真正等待网关放行的调用。
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

	var toolName string
	var params map[string]interface{}
	for _, message := range req.Messages {
		if len(message.ToolCalls) == 0 {
			continue
		}
		tc := message.ToolCalls[0]
		candidate := map[string]interface{}{}
		if len(tc.Function.Arguments) > 0 {
			if err := json.Unmarshal(tc.Function.Arguments, &candidate); err != nil {
				var argString string
				if stringErr := json.Unmarshal(tc.Function.Arguments, &argString); stringErr == nil {
					if mapErr := json.Unmarshal([]byte(argString), &candidate); mapErr != nil {
						candidate["raw_arguments"] = argString
					}
				} else {
					candidate["raw_arguments"] = string(tc.Function.Arguments)
				}
			}
		}
		toolName, params = tc.Function.Name, candidate
	}
	return toolName, params
}

// computeTraceID 从请求体中提取稳定的会话指纹，作为 Phase 2 TDG 校验使用的 Trace 标识。
//
// /v1/chat/completions 是无状态协议：Agent 每发起一次工具调用，都会把此前的完整对话历史
// （包括之前的工具调用与返回结果）重新放进 messages 数组一并发送。这意味着不能用逐请求
// 生成的 request_id 作为 Trace（那样每次工具调用都会落入独立的拓扑图，TDG 形同虚设），
// 也无法要求上游 Agent 框架配合透传自定义 Header（网关对其无控制权）。
//
// 因此改为取该对话首条 user 消息内容的 SM3 指纹：该内容在同一任务的多轮工具调用过程中
// 保持不变，是网关能够纯被动、无侵入地观测到的最稳定的会话锚点。
// 已知局限：若两个并发任务的首条用户消息完全相同，会被视为同一条 Trace；
// 这是用零侵入方式换取的工程折衷，不影响拓扑校验"防止单个任务内调用异常发散"的核心目标。
func computeTraceID(body []byte) string {
	var req struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil || len(req.Messages) == 0 {
		return ""
	}

	var anchor json.RawMessage
	for _, m := range req.Messages {
		if m.Role == "user" {
			anchor = m.Content
			break
		}
	}
	if len(anchor) == 0 {
		anchor = req.Messages[0].Content
	}
	if len(anchor) == 0 {
		return ""
	}
	return smcrypto.SM3Hex(anchor)
}

func (ap *AegisProxy) resolveTraceID(req *http.Request, body []byte, sessionID, taskID, agentID string) string {
	if req != nil {
		if existing := strings.TrimSpace(req.Header.Get("X-Aegis-Trace-ID")); existing != "" {
			return existing
		}
	}
	requestID := ""
	if req != nil {
		requestID = requestIDFromContext(req)
	}
	seedParts := []string{agentID}
	if sessionID != "" && sessionID != requestID {
		seedParts = append(seedParts, sessionID)
	}
	if taskID != "" && taskID != requestID {
		seedParts = append(seedParts, taskID)
	}
	if base := computeTraceID(body); base != "" {
		seedParts = append(seedParts, base)
	}
	joined := strings.Join(seedParts, "|")
	if strings.Trim(joined, "|") == "" {
		return ""
	}
	return smcrypto.SM3Hex([]byte(joined))
}

func (ap *AegisProxy) modifyResponse(resp *http.Response) error {
	if resp.Body == nil || ap.returnGate == nil {
		return nil
	}
	if encoding := resp.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		ap.logger.Debug("return gate skipped encoded response", zap.String("content_encoding", encoding))
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	result := ap.returnGate.Evaluate(body)
	gr := ap.newGateResult("return", result, resp.StatusCode)
	ap.recordDecision(resp.Request, gr, "", "")
	setGateHeaders(resp.Header, gr)

	toolName := resp.Request.Header.Get("X-Aegis-Tool-Name")

	switch result.Decision {
	case gates.Block, gates.Deny:
		ap.logger.Warn("response blocked by return gate", zap.String("reason", result.Reason))
		ap.captureSandboxResponse(resp, body, gr, false)
		resp.StatusCode = http.StatusForbidden
		resp.Status = "403 Forbidden"
		body = gateResponseBody(result.Decision, result.Reason)
	case gates.Degrade:
		ap.logger.Info("response filtered by return gate", zap.String("reason", result.Reason))
		ap.captureSandboxResponse(resp, body, gr, true)
		if ap.contentFilter != nil {
			filtered, removed := ap.contentFilter.FilterToolResponse(body, toolName)
			body = filtered
			if len(removed) > 0 {
				resp.Header.Set("X-Aegis-Filtered-Fields", strings.Join(removed, ","))
			}
		} else {
			body = ap.returnGate.Filter(body)
		}
		resp.Header.Set("X-Aegis-Filtered", "true")
	case gates.Allow:
		ap.logger.Debug("return gate allow", zap.String("reason", result.Reason))
		// Phase 4 复核结论：三态纯化不能只在 Degrade 之后才生效，否则真正需要它兜底的场景——
		// "PolicyEngine 评分没抓到异常，但某个字段本不该出现"——永远走不到这里。因此 Allow
		// 分支也要跑一遍纯化引擎；purificationEnabled 关闭时完全跳过，保持与 Phase 4 之前
		// 逐字节一致的透传行为。log-only/enforce 的具体生效与否由 contentFilter 自身决定。
		if ap.purificationEnabled && ap.contentFilter != nil {
			filtered, removed := ap.contentFilter.FilterToolResponse(body, toolName)
			body = filtered
			if len(removed) > 0 {
				resp.Header.Set("X-Aegis-Filtered-Fields", strings.Join(removed, ","))
				resp.Header.Set("X-Aegis-Filtered", "true")
			}
		}
	default:
		ap.logger.Debug("return gate decision not handled by filter pipeline", zap.String("decision", result.Decision.String()), zap.String("reason", result.Reason))
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
		er := interfaces.EvaluateResult{Decision: gates.Block, Reason: "failed to read request body"}
		result := ap.newGateResult("message", er, http.StatusBadRequest)
		ap.writeGateResponse(w, result)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	if ap.isToolCall(r.URL.Path, body) {
		result, ok := ap.handleToolCall(r, body)
		setGateHeaders(w.Header(), result)
		if !ok {
			ap.writeGateResponse(w, result)
			return
		}
	}

	if ap.isChatCompletion(r.URL.Path) {
		result, ok := ap.handleChatRequest(r, body)
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
	if result.TokenStatus != "" {
		header.Set("X-Aegis-Token-Status", result.TokenStatus)
	}
	if result.AuthMode != "" {
		header.Set("X-Aegis-Auth-Mode", result.AuthMode)
	}
	if result.UnauthorizedAllow {
		header.Set("X-Aegis-Unauthorized-Allow", "true")
	}
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

func normalizeGatewayTokenMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "strict", "compat", "warn":
		return strings.TrimSpace(strings.ToLower(mode))
	default:
		return "strict"
	}
}

func normalizeUpstreamAuthMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "gateway_managed", "passthrough":
		return strings.TrimSpace(strings.ToLower(mode))
	default:
		return "gateway_managed"
	}
}

func (ap *AegisProxy) missingUpstreamAuthorizationReason(req *http.Request) string {
	if ap == nil {
		return ""
	}
	switch ap.authMode {
	case "passthrough":
		clientAuth := strings.TrimSpace(req.Header.Get("Authorization"))
		if clientAuth != "" && vkey.ExtractGatewayKey(clientAuth) == "" {
			return ""
		}
		if strings.TrimSpace(ap.vkeyMgr.GetLLMAPIKey()) != "" {
			return ""
		}
		return "missing upstream Authorization"
	default:
		if strings.TrimSpace(ap.vkeyMgr.GetLLMAPIKey()) != "" {
			return ""
		}
		return "gateway-managed upstream Authorization is not configured"
	}
}

func firstTokenStatus(header http.Header) string {
	if header == nil {
		return ""
	}
	return strings.TrimSpace(header.Get("X-Aegis-Token-Status"))
}
