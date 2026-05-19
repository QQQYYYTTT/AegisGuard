package gateway

import (
	"net/http"

	"aegisguard/internal/interfaces"
)

// ActionEvaluator [PARTIAL] [分工1] 工具调用评估器
//
// proxy.go 需要：对工具调用请求做风险评估和授权校验，返回决策和原因。
// 当前实现由 gates.ActionGate 提供，状态：
//   - Evaluate:         [PARTIAL] gates/action.go 有完整 Evaluate 但仅做 token 校验 + scope 前缀匹配，
//     缺真实注入检测、工具参数语义分析
//   - BatchWindowJudge: [DONE] gates/batch_window.go 已实现滑动窗口判定
type ActionEvaluator interface {
	Evaluate(toolName string, params map[string]interface{}, headers http.Header) interfaces.EvaluateResult
}
