package gateway

import "aegisguard/internal/gates"

// ReturnEvaluator [UNDEFINED] [分工1] 返回门控评估器
//
// proxy.go 需要：对 LLM 返回结果做敏感数据检测和过滤。
// 当前实现由 gates.ReturnGate 提供，状态：
//   - Evaluate: [UNDEFINED] gates/return.go 的 Evaluate 返回 nil，完全未实现
//   - Filter:   [UNDEFINED] 未实现
type ReturnEvaluator interface {
	Evaluate(body []byte) (gates.Decision, string)
	Filter(body []byte) []byte
}
