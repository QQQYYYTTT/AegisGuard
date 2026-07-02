package gates

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aegisguard/internal/auth"
	"aegisguard/internal/interfaces"

	"go.uber.org/zap"
)

type ActionGate struct {
	verifier           *auth.Verifier
	batchJudge         *BatchWindowJudge
	enableBatch        bool
	logger             *zap.Logger
	policyEngine       *PolicyEngine
	tokenMode          string
	dynamicRuleRouting bool
	tdgRegistry        *TDGRegistry
	tdgEnabled         bool
	tdgMode            string
}

func NewActionGate(logger *zap.Logger) *ActionGate {
	return NewActionGateWithMode(logger, "strict")
}

func NewActionGateWithMode(logger *zap.Logger, tokenMode string) *ActionGate {
	return NewActionGateWithRuntime(logger, tokenMode, nil)
}

func NewActionGateWithRuntime(logger *zap.Logger, tokenMode string, runtime *PolicyRuntime) *ActionGate {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ActionGate{
		verifier:     auth.NewVerifier(),
		enableBatch:  false,
		logger:       logger,
		policyEngine: NewPolicyEngineWithRuntime(runtime),
		tokenMode:    normalizeTokenMode(tokenMode),
	}
}

func NewActionGateWithBatch(windowSize, maxEvents int, judgeInterval time.Duration, judgeFunc JudgeFunc, logger *zap.Logger) *ActionGate {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ActionGate{
		verifier:     auth.NewVerifier(),
		batchJudge:   NewBatchWindowJudge(windowSize, maxEvents, judgeInterval, judgeFunc, logger),
		enableBatch:  true,
		logger:       logger,
		policyEngine: NewPolicyEngine(),
		tokenMode:    "strict",
	}
}

// SetDynamicRuleRouting 开启/关闭 Phase 1 动态规则路由（按工具名路由规则子集）。
// 默认关闭；关闭时 Evaluate 完全回退到 PolicyEngine.Score 的全量扫描行为。
func (ag *ActionGate) SetDynamicRuleRouting(enabled bool) {
	ag.dynamicRuleRouting = enabled
}

// SetTDG 开启/关闭 Phase 2 工具调用拓扑校验（借鉴 IPIGuard TDG 思想）。
// 默认关闭；log-only 模式下仅记录违规不阻断，enforce 模式下拒绝违规调用。
// 关闭后已创建的 TDGRegistry 会被保留（不销毁已积累的拓扑数据），Evaluate 内部通过
// tdgEnabled 开关短路校验逻辑，行为等价于未开启 Phase 2。
func (ag *ActionGate) SetTDG(settings TDGSettings) {
	ag.tdgEnabled = settings.Enabled
	ag.tdgMode = normalizeTDGMode(settings.Mode)
	if settings.Enabled && ag.tdgRegistry == nil {
		ag.tdgRegistry = NewTDGRegistry(settings.MaxNodes, settings.MaxRepeat, settings.TTL)
	}
}

// extractTraceID 从请求头中提取 Phase 2 TDG 校验使用的 Trace 标识。
// 该值由网关层（gateway/proxy.go 的 computeTraceID）在转发前注入，取对话首条用户消息的
// SM3 指纹——/v1/chat/completions 是无状态协议，每轮工具调用都会重发完整 messages 数组，
// 若改用逐请求生成的 request_id 作为 Trace，每次工具调用都会落入独立的拓扑图，TDG 形同虚设。
func extractTraceID(headers http.Header) string {
	if headers == nil {
		return "unknown"
	}
	if id := strings.TrimSpace(headers.Get("X-Aegis-Trace-ID")); id != "" {
		return id
	}
	return "unknown"
}

func (ag *ActionGate) Evaluate(toolName string, params map[string]interface{}, headers http.Header) interfaces.EvaluateResult {
	contentSummary := ag.extractContentSummary(params)
	var score int
	var rules []string
	var ruleAction Decision
	if ag.dynamicRuleRouting {
		score, rules, ruleAction = ag.policyEngine.ScoreForGateAndTool("action", toolName, toolName+"\n"+contentSummary, true)
	} else {
		score, rules, ruleAction = ag.policyEngine.ScoreForGateAndTool("action", toolName, toolName+"\n"+contentSummary, false)
	}
	if hasRuleFromList(rules, "memory_poisoning") {
		ag.logger.Warn("action blocked: memory poisoning detected",
			zap.String("tool", toolName),
			zap.Int("score", score),
			zap.Strings("rules", rules),
		)
		return makeEvaluateResult(Block, "action attempts to modify trusted memory/instructions", score, rules)
	}
	if hasRuleFromList(rules, "illegal_finance") {
		ag.logger.Warn("action denied: illegal finance detected",
			zap.String("tool", toolName),
			zap.Int("score", score),
			zap.Strings("rules", rules),
		)
		return makeEvaluateResult(Deny, "action indicates prohibited financial misconduct", score, rules)
	}
	if hasRuleFromList(rules, "prompt_injection") && (hasRuleFromList(rules, "privileged_scope") || hasRuleFromList(rules, "sensitive_access")) {
		ag.logger.Warn("action denied: prompt injection with privileged scope",
			zap.String("tool", toolName),
			zap.Int("score", score),
			zap.Strings("rules", rules),
		)
		return makeEvaluateResult(Deny, "action combines prompt-injection markers with privileged or sensitive operation", score, rules)
	}
	if ruleAction == Deny && ag.policyEngine.ShouldHumanReview(score) {
		return makeEvaluateResult(Deny, "action denied by policy rule", score, rules)
	}
	if ruleAction == Block && ag.policyEngine.ShouldHumanReview(score) {
		return makeEvaluateResult(Block, "action blocked by policy rule", score, rules)
	}
	if hasRuleFromList(rules, "high_impact_action") || ag.policyEngine.ShouldHumanReview(score) {
		ag.logger.Info("action requires human approval",
			zap.String("tool", toolName),
			zap.Int("score", score),
			zap.Strings("rules", rules),
		)
		return makeEvaluateResult(HumanApproval, "action requires human approval due to semantic risk", score, rules)
	}

	if ag.tdgEnabled && ag.tdgRegistry != nil {
		traceID := extractTraceID(headers)
		tdg := ag.tdgRegistry.GetOrCreate(traceID)
		allowed, violation := tdg.ValidateCall(toolName)
		tdg.RecordCall(toolName)
		if !allowed {
			ag.logger.Warn("tdg topology violation",
				zap.String("tool", toolName),
				zap.String("trace_id", traceID),
				zap.String("mode", ag.tdgMode),
				zap.String("reason", violation),
			)
			if ag.tdgMode == "enforce" {
				return makeEvaluateResult(Deny, "tool call topology violation: "+violation, score, rules)
			}
		}
	}

	tokenStr := headers.Get("X-Aegis-Token")
	if tokenStr == "" {
		return ag.handleMissingToken(score, rules, headers)
	}

	var token auth.RequireToken
	if err := json.Unmarshal([]byte(tokenStr), &token); err != nil {
		ag.logger.Warn("action denied: invalid token format",
			zap.String("tool", toolName),
			zap.Error(err),
		)
		return makeEvaluateResult(Deny, fmt.Sprintf("invalid token format: %v", err), score, rules)
	}

	if err := ag.verifier.Verify(&token); err != nil {
		ag.logger.Warn("action denied: token verification failed",
			zap.String("tool", toolName),
			zap.String("agent_id", token.AgentID),
			zap.Error(err),
		)
		return makeEvaluateResult(Deny, fmt.Sprintf("token verification failed: %v", err), score, rules)
	}

	if token.ToolName != toolName {
		ag.logger.Warn("action denied: tool name mismatch",
			zap.String("expected", token.ToolName),
			zap.String("actual", toolName),
		)
		return makeEvaluateResult(Deny, fmt.Sprintf("tool name mismatch: token=%s, actual=%s", token.ToolName, toolName), score, rules)
	}

	requestSessionID := strings.TrimSpace(headers.Get("X-Aegis-Session-ID"))
	if requestSessionID == "" || token.SessionID != requestSessionID {
		ag.logger.Warn("action denied: session mismatch",
			zap.String("expected", token.SessionID),
			zap.String("actual", requestSessionID),
		)
		return makeEvaluateResult(Deny, fmt.Sprintf("session mismatch: token=%s, actual=%s", token.SessionID, requestSessionID), score, rules)
	}

	requestTaskID := strings.TrimSpace(headers.Get("X-Aegis-Task-ID"))
	if requestTaskID == "" || token.TaskID != requestTaskID {
		ag.logger.Warn("action denied: task mismatch",
			zap.String("expected", token.TaskID),
			zap.String("actual", requestTaskID),
		)
		return makeEvaluateResult(Deny, fmt.Sprintf("task mismatch: token=%s, actual=%s", token.TaskID, requestTaskID), score, rules)
	}

	if token.SchemaHash != "" {
		schemaB64 := strings.TrimSpace(headers.Get("X-Aegis-Tool-Schema"))
		if schemaB64 == "" {
			ag.logger.Warn("action denied: missing tool schema for schema-bound token",
				zap.String("tool", toolName),
			)
			return makeEvaluateResult(Deny, "missing tool schema for schema-bound token", score, rules)
		}
		toolSchema, err := base64.StdEncoding.DecodeString(schemaB64)
		if err != nil {
			ag.logger.Warn("action denied: invalid tool schema header encoding",
				zap.String("tool", toolName),
				zap.Error(err),
			)
			return makeEvaluateResult(Deny, fmt.Sprintf("invalid tool schema header: %v", err), score, rules)
		}
		if err := auth.CompareSchemaHash(&token, toolSchema); err != nil {
			ag.logger.Warn("action denied: schema hash mismatch",
				zap.String("tool", toolName),
				zap.Error(err),
			)
			return makeEvaluateResult(Deny, fmt.Sprintf("schema verification failed: %v", err), score, rules)
		}
	}

	if !ag.checkScope(token.Scope, toolName, params) {
		ag.logger.Warn("action denied: scope violation",
			zap.String("scope", token.Scope),
			zap.String("tool", toolName),
		)
		return makeEvaluateResult(Deny, fmt.Sprintf("scope violation: %s not allowed for %s", token.Scope, toolName), score, rules)
	}

	if token.MaxCalls > 0 {
		token.CallCount++
	}

	if ag.enableBatch && ag.batchJudge != nil {
		event := WindowEvent{
			Timestamp:   time.Now(),
			AgentID:     token.AgentID,
			ToolName:    toolName,
			Params:      params,
			RiskLevel:   token.RiskLevel,
			Content:     contentSummary,
			RequiresJud: token.RiskLevel >= 40,
		}
		batchDecision := ag.batchJudge.AddEvent(event)
		if batchDecision == Block {
			ag.logger.Warn("action blocked by batch judge",
				zap.String("agent_id", token.AgentID),
				zap.String("tool", toolName),
			)
			return makeEvaluateResult(Block, "blocked by batch window judge: suspicious pattern detected", score, rules)
		} else if batchDecision == HumanApproval {
			ag.logger.Info("action requires batch judge review",
				zap.String("agent_id", token.AgentID),
				zap.String("tool", toolName),
			)
			return makeEvaluateResult(HumanApproval, "batch judge requires human review", score, rules)
		}
	}

	if token.RiskLevel >= 70 {
		ag.logger.Warn("action blocked: high risk level",
			zap.String("agent_id", token.AgentID),
			zap.Int("risk_level", token.RiskLevel),
		)
		return makeEvaluateResult(Block, "high risk level", score, rules)
	} else if token.RiskLevel >= 40 {
		ag.logger.Info("action requires human approval: medium risk",
			zap.String("agent_id", token.AgentID),
			zap.Int("risk_level", token.RiskLevel),
		)
		return makeEvaluateResult(HumanApproval, "medium risk, approval required", score, rules)
	}

	ag.logger.Debug("action allowed",
		zap.String("tool", toolName),
		zap.Int("score", score),
		zap.Int("risk_level", token.RiskLevel),
	)
	return makeEvaluateResult(Allow, "authorized by RequireToken and semantic policy checks", score, rules)
}

func (ag *ActionGate) handleMissingToken(score int, rules []string, headers http.Header) interfaces.EvaluateResult {
	status := strings.TrimSpace(headers.Get("X-Aegis-Token-Status"))
	if status == "" {
		status = "missing"
	}

	switch ag.tokenMode {
	case "strict":
		return makeEvaluateResult(Deny, "RequireToken is mandatory in strict mode; token_status="+status, score, rules)
	case "warn":
		ag.logger.Warn("allowing unauthorized action in warn mode", zap.String("token_status", status))
		return makeEvaluateResult(Allow, "action passed semantic policy checks; missing RequireToken allowed in warn mode; token_status="+status, score, rules)
	default:
		return makeEvaluateResult(Allow, "action passed semantic policy checks; missing RequireToken allowed in compat mode; token_status="+status, score, rules)
	}
}

func (ag *ActionGate) checkScope(scope, toolName string, params map[string]interface{}) bool {
	parts := strings.Split(scope, ":")
	if len(parts) < 2 {
		return false
	}

	toolType := parts[0]
	if !strings.HasPrefix(toolName, toolType) {
		return false
	}

	if len(parts) >= 3 && toolName == "read_file" {
		allowedPath := parts[2]
		actualPath, _ := params["path"].(string)
		if !strings.HasPrefix(actualPath, strings.TrimSuffix(allowedPath, "*")) {
			return false
		}
	}

	return true
}

func (ag *ActionGate) extractContentSummary(params map[string]interface{}) string {
	if params == nil {
		return ""
	}
	if cmd, ok := params["command"].(string); ok {
		return cmd
	}
	if query, ok := params["query"].(string); ok {
		return query
	}
	if path, ok := params["path"].(string); ok {
		return path
	}

	raw, err := json.Marshal(params)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (ag *ActionGate) Close() {
	if ag.batchJudge != nil {
		ag.batchJudge.Close()
	}
	if ag.tdgRegistry != nil {
		ag.tdgRegistry.Close()
	}
}

func normalizeTokenMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "strict", "compat", "warn":
		return strings.TrimSpace(strings.ToLower(mode))
	default:
		return "strict"
	}
}
