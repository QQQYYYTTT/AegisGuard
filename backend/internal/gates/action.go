package gates

import (
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
	verifier     *auth.Verifier
	batchJudge   *BatchWindowJudge
	enableBatch  bool
	logger       *zap.Logger
	policyEngine *PolicyEngine
	tokenMode    string
}

func NewActionGate(logger *zap.Logger) *ActionGate {
	return NewActionGateWithMode(logger, "strict")
}

func NewActionGateWithMode(logger *zap.Logger, tokenMode string) *ActionGate {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ActionGate{
		verifier:     auth.NewVerifier(),
		enableBatch:  false,
		logger:       logger,
		policyEngine: NewPolicyEngine(),
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

func (ag *ActionGate) Evaluate(toolName string, params map[string]interface{}, headers http.Header) interfaces.EvaluateResult {
	contentSummary := ag.extractContentSummary(params)
	score, rules := ag.policyEngine.Score(toolName + "\n" + contentSummary)
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
	if hasRuleFromList(rules, "high_impact_action") || ag.policyEngine.ShouldHumanReview(score) {
		ag.logger.Info("action requires human approval",
			zap.String("tool", toolName),
			zap.Int("score", score),
			zap.Strings("rules", rules),
		)
		return makeEvaluateResult(HumanApproval, "action requires human approval due to semantic risk", score, rules)
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
}

func normalizeTokenMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "strict", "compat", "warn":
		return strings.TrimSpace(strings.ToLower(mode))
	default:
		return "strict"
	}
}
