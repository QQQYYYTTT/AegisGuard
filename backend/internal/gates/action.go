package gates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aegisguard/internal/auth"

	"go.uber.org/zap"
)

type ActionGate struct {
	verifier    *auth.Verifier    // Token 校验器
	batchJudge  *BatchWindowJudge // 批量窗口判定器（TrinityGuard 风格）
	enableBatch bool              // 是否启用批量窗口判定
	logger      *zap.Logger
}

func NewActionGate(logger *zap.Logger) *ActionGate {
	if logger == nil {
		logger = zap.NewNop()
	}
	ag := &ActionGate{
		verifier:    auth.NewVerifier(),
		enableBatch: false,
		logger:      logger,
	}
	return ag
}

// NewActionGateWithBatch 创建支持批量窗口判定的 ActionGate
// 参数：
//   - windowSize: 滑动窗口大小（事件数量）
//   - maxEvents: 窗口摘要中最多包含的事件数
//   - judgeInterval: LLM 判定间隔
//   - judgeFunc: LLM 判定函数（如果为 nil，则使用规则引擎）
//
// 返回：ActionGate 实例
func NewActionGateWithBatch(windowSize, maxEvents int, judgeInterval time.Duration, judgeFunc JudgeFunc, logger *zap.Logger) *ActionGate {
	if logger == nil {
		logger = zap.NewNop()
	}
	ag := &ActionGate{
		verifier:    auth.NewVerifier(),
		batchJudge:  NewBatchWindowJudge(windowSize, maxEvents, judgeInterval, judgeFunc, logger),
		enableBatch: true,
		logger:      logger,
	}
	return ag
}

// Evaluate 评估工具调用请求
// toolName: 工具名
// params: 工具参数
// headers: 请求头（从中提取 RequireToken）
func (ag *ActionGate) Evaluate(toolName string, params map[string]interface{}, headers http.Header) (Decision, string) {
	// 1. 从请求头中提取 Token
	tokenStr := headers.Get("X-Aegis-Token")
	if tokenStr == "" {
		// Agent 没申请 Token，尝试根据上下文推断（简化版）
		return Deny, "missing require token: tool call must be pre-authorized"
	}

	// 2. 解析 Token
	var token auth.RequireToken
	if err := json.Unmarshal([]byte(tokenStr), &token); err != nil {
		return Deny, fmt.Sprintf("invalid token format: %v", err)
	}

	// 3. 全字段校验
	if err := ag.verifier.Verify(&token); err != nil {
		return Deny, fmt.Sprintf("token verification failed: %v", err)
	}

	// 4. 工具名称匹配
	if token.ToolName != toolName {
		return Deny, fmt.Sprintf("tool name mismatch: token=%s, actual=%s", token.ToolName, toolName)
	}

	// 5. 权限范围检查（核心！）
	if !ag.checkScope(token.Scope, toolName, params) {
		return Deny, fmt.Sprintf("scope violation: %s not allowed for %s", token.Scope, toolName)
	}

	// 6. 调用次数限流（SAGA 风格防 DoS）
	// 每次成功验证后递增 CallCount（由上层应用逻辑控制）
	if token.MaxCalls > 0 {
		token.CallCount++
	}

	// 7. 批量窗口判定（TrinityGuard 风格，纯代码优化，LLM 可选）
	if ag.enableBatch && ag.batchJudge != nil {
		// 构建窗口事件
		event := WindowEvent{
			Timestamp:   time.Now(),
			AgentID:     token.AgentID,
			ToolName:    toolName,
			Params:      params,
			RiskLevel:   token.RiskLevel,
			Content:     ag.extractContentSummary(params),
			RequiresJud: token.RiskLevel >= 40,
		}

		// 添加到窗口并获取决策
		batchDecision := ag.batchJudge.AddEvent(event)

		// 如果批量判定器决定阻断，优先执行
		if batchDecision == Block {
			return Block, "blocked by batch window judge: suspicious pattern detected"
		} else if batchDecision == HumanApproval {
			return HumanApproval, "batch judge requires human review"
		}
	}

	// 8. 风险等级决策（传统单点判定）
	if token.RiskLevel >= 70 {
		return Block, "high risk level"
	} else if token.RiskLevel >= 40 {
		return HumanApproval, "medium risk, approval required"
	}

	return Allow, "authorized"
}

// checkScope 检查权限范围
// scope 格式："read:file:/tmp/*", "search:web", "exec:shell:ls"
func (ag *ActionGate) checkScope(scope, toolName string, params map[string]interface{}) bool {
	// 简化实现：字符串前缀匹配
	// 后续支持更复杂的模式匹配

	parts := strings.Split(scope, ":")
	if len(parts) < 2 {
		return false
	}

	// 检查工具类型匹配
	toolType := parts[0] // read, search, exec...
	if !strings.HasPrefix(toolName, toolType) {
		return false
	}

	// 检查路径/参数范围（如果有）
	if len(parts) >= 3 && toolName == "read_file" {
		allowedPath := parts[2]
		actualPath, _ := params["path"].(string)
		if !strings.HasPrefix(actualPath, strings.TrimSuffix(allowedPath, "*")) {
			return false
		}
	}

	return true
}

// extractContentSummary 从参数中提取内容摘要（用于窗口判定）
func (ag *ActionGate) extractContentSummary(params map[string]interface{}) string {
	// 简化实现：提取关键参数
	if cmd, ok := params["command"].(string); ok {
		return cmd
	}
	if query, ok := params["query"].(string); ok {
		return query
	}
	if path, ok := params["path"].(string); ok {
		return path
	}
	return ""
}

// Close 关闭 ActionGate（释放批量判定器资源）
func (ag *ActionGate) Close() {
	if ag.batchJudge != nil {
		ag.batchJudge.Close()
	}
}
