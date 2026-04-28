package gates

// Decision 决策类型
type Decision int

const (
	Allow Decision = iota
	Block
	Degrade
	Deny
	HumanApproval
)

// String 返回决策字符串表示
func (d Decision) String() string {
	switch d {
	case Allow:
		return "Allow"
	case Block:
		return "Block"
	case Degrade:
		return "Degrade"
	case Deny:
		return "Deny"
	case HumanApproval:
		return "HumanApproval"
	default:
		return "Unknown"
	}
}
