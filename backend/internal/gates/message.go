package gates

// MessageGate 消息门控
type MessageGate struct{}

// NewMessageGate 创建消息门控
func NewMessageGate() *MessageGate {
	return &MessageGate{}
}

// Evaluate 评估消息
func (g *MessageGate) Evaluate(body []byte) (Decision, string) {
	// TODO: 实现消息评估逻辑
	return Allow, ""
}
