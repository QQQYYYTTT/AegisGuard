package gates

// ReturnGate 返回门控
type ReturnGate struct{}

// NewReturnGate 创建返回门控
func NewReturnGate() *ReturnGate {
	return &ReturnGate{}
}

// Evaluate 评估返回结果
func (g *ReturnGate) Evaluate(body []byte) []byte {
	// TODO: 实现返回结果评估
	return nil
}
