package gateway

import "aegisguard/internal/gates"

// MessageEvaluator [PARTIAL] [分工1] 消息门控评估器
//
// proxy.go 需要：对 LLM 聊天请求做风险检测，返回决策和原因。
// 当前实现由 gates.MessageGate 提供，状态：
//   - Evaluate: [PARTIAL] gates/message.go 的 Evaluate 仅返回 Allow，需实现真实检测
type MessageEvaluator interface {
	Evaluate(body []byte) (gates.Decision, string)
}
